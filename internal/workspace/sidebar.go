package workspace

import (
	"os/exec"
	"slices"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/list"
	"github.com/ZerGo0/editor/internal/design/pane"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/fsutil"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/ZerGo0/editor/internal/layout"
	"github.com/ZerGo0/editor/internal/workspace/tablabel"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	terminalApp      = "Terminal"
	sideBarLabel     = "Primary Side Bar"
	openEditorsTitle = "Open Editors"
	openEditorsPane  = "openEditors"
	foldersPane      = "folders"
	explorerSizesKey = "workbench.explorer.sizes"
	githubSizesKey   = "workbench.github.sizes"
	unsavedLabel     = "Unsaved changes"
)

func (w *Workspace) sidebar(c *ui.Context) {
	p := token.Of(c)
	side := ui.Column(c).Fill().Background(p.Layer.SideBar).BorderWidth(0, token.BorderWidth.Px(), 0, 0).BorderColor(p.Border.Default).
		Role(ui.RoleRegion).Label(sideBarLabel)
	w.focus.side = side.FocusWithin()
	v := findView(w.tools.sideViews, w.tools.side)
	side.Children(func() {
		w.activityBar(c, v, side.Hovered() || w.focus.side)
		if v == nil {
			return
		}
		ui.Column(c.Key("view-" + v.id)).Grow(1).FillWidth().MinHeight(0).Role(ui.RoleRegion).Label(v.title).Children(func() { v.view(c) })
	})
}

func (w *Workspace) explorerView(c *ui.Context) {
	w.autoReveal()
	s := &w.explorerPanes
	if s.Collapsed == nil {
		s.Collapsed = map[string]bool{}
	}
	s.Collapsed[openEditorsPane], s.Collapsed[foldersPane] = !w.tools.openEditors, w.tools.explorerCollapsed
	pane.Stack(c, s, []pane.Pane{
		{ID: openEditorsPane, Title: openEditorsTitle, Actions: w.openEditorsActions, Body: w.openEditorsView},
		{ID: foldersPane, Title: w.Name, Actions: w.explorerActions, Body: func(c *ui.Context) { w.tree.View(c, filetreeHandlers(w)) }},
	})
	w.tools.openEditors, w.tools.explorerCollapsed = !s.Collapsed[openEditorsPane], s.Collapsed[foldersPane]
}

func (w *Workspace) explorerActions(c *ui.Context) {
	w.iconButton(c, icons.FileAdd, "New File...", "explorer.newFile", false, w.NewFile)
	w.iconButton(c, icons.FolderAdd, "New Folder...", "explorer.newFolder", false, w.NewFolder)
	w.iconButton(c, icons.Refresh, "Refresh Explorer", "", false, w.RefreshExplorer)
	w.iconButton(c, icons.CollapseAll, "Collapse Folders in Explorer", "", false, w.tree.CollapseAll)
}

func (w *Workspace) autoReveal() {
	t := w.active.ActiveTab()
	if t == nil || t.Path == w.tools.revealed {
		return
	}
	w.tools.revealed = t.Path
	if onDisk(t.Path) && fsutil.Within(w.Root, t.Path) {
		w.tree.Reveal(t.Path)
	}
}

type openEditor struct {
	group *layout.Group
	tab   *layout.Tab
}

func (w *Workspace) openEditors() []openEditor {
	var out []openEditor
	for _, g := range w.layout.Groups() {
		for _, t := range g.Tabs {
			out = append(out, openEditor{g, t})
		}
	}
	return out
}

func (w *Workspace) openEditorsActions(c *ui.Context) {
	w.iconButton(c, icons.Check, "Save All", "workbench.action.files.saveAll", false, w.SaveAll)
	w.iconButton(c, icons.ClearAll, "Close All Editors", "workbench.action.closeAllEditors", false, w.CloseAllEditors)
}

func (w *Workspace) openEditorsView(c *ui.Context) {
	entries := w.openEditors()
	labels := w.tabLabels()
	w.tools.openList.Selected = &w.tools.openRow
	w.tools.openList.Label = func(i int) string { return w.tabTitle(entries[i].tab.Path) }
	l := list.View(c, &w.tools.openList, len(entries), list.ViewOptions{Label: openEditorsTitle, Empty: func(c *ui.Context) {
		feedback.Empty(c, feedback.EmptyOptions{Message: watermarkLabel})
	}}, func(i int) list.Item {
		e := entries[i]
		name := w.tabTitle(e.tab.Path)
		d := w.doc(e.tab.Path)
		dirty := d != nil && d.Modified()
		it := list.Item{
			Label:       tablabel.LTR(name),
			Description: labels[e.tab.Path],
			Leading:     func(c *ui.Context) { w.tabIcon(c, e.tab.Path) },
			Tooltip:     w.tabTooltip(e.tab.Path),
			OnClick:     func() { w.activate(e.group, e.tab) },
			OnAuxClick:  func() { w.requestClose(e.group, []*layout.Tab{e.tab}) },
			Menu:        func(m *ui.Menu) { w.tabMenu(m, e.group, e.tab) },
			Actions: func(c *ui.Context) {
				if action.IconButton(c, icons.Close, "Close "+name, action.IconOptions{ID: "close-open-editor", Tooltip: "Close"}).Clicked() {
					w.requestClose(e.group, []*layout.Tab{e.tab})
				}
			},
		}
		if dirty {
			it.Trailing = func(c *ui.Context) {
				ui.Box(c).Size(token.DirtyDotSize.Px(), token.DirtyDotSize.Px()).Radius(token.RadiusPill.Px()).
					Background(token.Of(c).Text.Primary).Label(unsavedLabel)
			}
		}
		return it
	})
	w.openEditorsKeys(c, l, entries)
}

func (w *Workspace) openEditorsKeys(c *ui.Context, l ui.Element, entries []openEditor) {
	if !l.FocusWithin() {
		active := slices.IndexFunc(entries, func(e openEditor) bool { return e.group == w.active && e.tab == w.active.ActiveTab() })
		if active != w.tools.openRow {
			w.tools.openRow = active
			c.Invalidate()
		}
		return
	}
	row := w.tools.openRow
	if row < 0 || row >= len(entries) {
		return
	}
	e := entries[row]
	switch {
	case l.Submitted():
		w.activate(e.group, e.tab)
	case l.Shortcut(0, ui.KeyBackspace) || l.Shortcut(0, ui.KeyDelete) || l.Shortcut(ui.Cmd, ui.KeyBackspace):
		w.requestClose(e.group, []*layout.Tab{e.tab})
		w.focusTab = nil
		w.tools.openRow = min(row, len(entries)-2)
	}
}

func (w *Workspace) RefreshExplorer() {
	w.tree.InvalidateAll()
	w.refreshIgnored()
}

func (w *Workspace) pathItems(m *ui.Menu, p string) {
	if m.Item("Copy Path").Chosen() {
		mygo.Clipboard.WriteText(p)
	}
	if m.Item("Copy Relative Path").Chosen() {
		mygo.Clipboard.WriteText(w.rel(p))
	}
}

func (w *Workspace) revealInFinder(p string) {
	mygo.Shell.ShowItemInFolder(p)
}

func (w *Workspace) openInTerminal(dir string) {
	applog.Go(func() {
		if out, err := exec.Command("open", "-a", terminalApp, dir).CombinedOutput(); err != nil {
			w.showError("Cannot open Terminal", errorWithOutput(err, out))
		}
	})
}
