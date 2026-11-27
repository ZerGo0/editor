package workspace

import (
	"slices"

	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/chrome"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/fsutil"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/ZerGo0/editor/internal/layout"
	"github.com/ZerGo0/editor/internal/viewer"
	"github.com/ZerGo0/editor/internal/workspace/tablabel"
	"github.com/egoist/mygo/ui"
)

const (
	tabStripLabel    = "Editor Tabs"
	moreActionsLabel = "More Actions..."
	previewToSideID  = "markdown.showPreviewToSide"
)

func (w *Workspace) tabLabels() map[string]string {
	var paths []string
	for _, t := range w.tabs() {
		if onDisk(t.Path) {
			paths = append(paths, t.Path)
		}
	}
	return tablabel.Describe(paths)
}

func (w *Workspace) tabTooltip(p string) string {
	if onDisk(p) {
		return fsutil.DisplayPath(p)
	}
	return w.tabTitle(p)
}

func (w *Workspace) tabBar(c *ui.Context, g *layout.Group) {
	tabs := slices.Clone(g.Tabs)
	labels := w.tabLabels()
	items := make([]chrome.Tab, len(tabs))
	for i, t := range tabs {
		d := w.doc(t.Path)
		items[i] = chrome.Tab{Label: w.tabTitle(t.Path), Description: labels[t.Path], Tooltip: w.tabTooltip(t.Path),
			Leading: func(c *ui.Context) { w.tabIcon(c, t.Path) }, Dirty: d != nil && d.Modified(), Preview: t.Preview, Pinned: t.Pinned}
	}
	selected := g.Active
	closeTab := func(i int) { w.requestClose(g, []*layout.Tab{tabs[i]}) }
	chrome.TabStrip(c, &selected, items, chrome.TabStripOptions{
		Label:              tabStripLabel,
		OnSelect:           func(i int) { w.activate(g, tabs[i]) },
		OnClose:            closeTab,
		OnAuxClick:         closeTab,
		OnUnpin:            func(i int) { g.SetPinned(tabs[i], false) },
		OnDoubleClick:      func(i int) { tabs[i].Preview = false },
		Menu:               func(i int, m *ui.Menu) { w.tabMenu(m, g, tabs[i]) },
		Drag:               func(i int) any { return tabDrag{g, tabs[i]} },
		Drop:               func(at int, el ui.Element) chrome.DropMark { return w.tabStripDrop(c, el, g, at) },
		EmptyMenu:          func(m *ui.Menu) { w.groupMenu(m, g) },
		OnEmptyDoubleClick: func() { w.active = g; w.NewUntitled() },
		Actions:            func(c *ui.Context) { w.editorActions(c, g) },
	})
}

func (w *Workspace) editorActions(c *ui.Context, g *layout.Group) {
	tab := g.ActiveTab()
	if tab != nil && !isVirtual(tab.Path) && previewable(tab.Path) {
		if action.IconButton(c, icons.Preview, previewToSideLabel, action.IconOptions{Keys: w.keyLabel(previewToSideID)}).Clicked() {
			w.activate(g, tab)
			w.OpenPreviewToSide()
		}
	}
	if action.IconButton(c, icons.Split, splitRightLabel, action.IconOptions{Keys: w.keyLabel(splitRightID), ID: "split-group"}).Clicked() {
		w.active = g
		w.SplitEditor(layout.Right)
	}
	action.MenuIconButton(c, icons.More, moreActionsLabel, action.IconOptions{}, func(m *ui.Menu) { w.groupMenu(m, g) })
}

func (w *Workspace) tabIcon(c *ui.Context, p string) {
	if !isVirtual(p) {
		icons.FileIcon(c, w.tabTitle(p), false, false, token.IconSize.Px())
		return
	}
	svg := icons.File
	if p == SettingsTab || p == KeybindingsTab {
		svg = icons.Settings
	}
	ui.Icon(c, svg).Size(token.IconSize.Px(), token.IconSize.Px()).TextColor(token.Of(c).Icon).Shrink(0).AccessHidden(true)
}

func (w *Workspace) tabMenu(m *ui.Menu, g *layout.Group, tab *layout.Tab) {
	if m.Item("Close").Shortcut(ui.Cmd, ui.KeyW).Chosen() {
		w.requestClose(g, []*layout.Tab{tab})
	}
	if m.Item("Close Others").Disabled(len(g.Tabs) < 2).Chosen() {
		w.closeOthers(g, tab)
	}
	if m.Item("Close to the Right").Disabled(g.Index(tab) == len(g.Tabs)-1).Chosen() {
		w.closeToRight(g, tab)
	}
	if m.Item("Close Saved").Chosen() {
		w.closeSaved(g)
	}
	if m.Item("Close All").Chosen() {
		w.closeAll(g)
	}
	if onDisk(tab.Path) {
		m.Separator()
		w.pathItems(m, tab.Path)
		m.Separator()
		if m.Item("Reveal in Finder").Chosen() {
			w.revealInFinder(tab.Path)
		}
		if m.Item("Reveal in Explorer View").Disabled(!fsutil.Within(w.Root, tab.Path)).Chosen() {
			w.activate(g, tab)
			w.RevealInSidebar()
		}
	}
	m.Separator()
	if m.Item("Keep Open").Disabled(!tab.Preview).Chosen() {
		tab.Preview = false
	}
	pin := "Pin"
	if tab.Pinned {
		pin = "Unpin"
	}
	if m.Item(pin).Chosen() {
		g.SetPinned(tab, !tab.Pinned)
	}
	m.Separator()
	if m.Item("Split Right").Chosen() {
		w.activate(g, tab)
		w.SplitEditor(layout.Right)
	}
	if m.Item("Split Down").Chosen() {
		w.activate(g, tab)
		w.SplitEditor(layout.Down)
	}
}

func (w *Workspace) groupMenu(m *ui.Menu, g *layout.Group) {
	w.commandItem(m, "New Text File", "workbench.action.files.newUntitledFile")
	m.Separator()
	w.commandItem(m, "Reopen Closed Editor", "workbench.action.reopenClosedEditor")
	m.Separator()
	if m.Item("Close Saved").Disabled(len(g.Tabs) == 0).Chosen() {
		w.closeSaved(g)
	}
	if m.Item("Close All").Disabled(len(g.Tabs) == 0).Chosen() {
		w.closeAll(g)
	}
	m.Separator()
	if m.Item("Split Right").Disabled(g.ActiveTab() == nil).Chosen() {
		w.active = g
		w.SplitEditor(layout.Right)
	}
	if m.Item("Split Down").Disabled(g.ActiveTab() == nil).Chosen() {
		w.active = g
		w.SplitEditor(layout.Down)
	}
}

func previewable(p string) bool {
	return viewer.Detect(p, nil).Previewable()
}
