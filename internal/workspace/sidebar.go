package workspace

import (
	"os/exec"
	"strings"

	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	sideHeaderHeight   = 35
	sideHeaderPadding  = 20
	sideActionsPadding = 8
	sectionHeight      = 22
	sectionChevron     = 12
	sectionPadding     = 4
	sectionGap         = 2
	terminalApp        = "Terminal"
)

func (w *Workspace) sidebar(c *ui.Context, vibrant bool) {
	t := c.Theme()
	side := ui.Column(c).Fill()
	if !vibrant {
		side.Background(t.Surface)
	}
	w.tools.sideHovered = side.Hovered()
	v := findView(w.tools.sideViews, w.tools.side)
	if v == nil {
		return
	}
	side.Children(func() {
		ui.Row(c).Height(sideHeaderHeight).Shrink(0).FillWidth().AlignItems(ui.Center).Padding(0, sideActionsPadding, 0, sideHeaderPadding).Children(func() {
			ui.Text(c, strings.ToUpper(v.title)).FontSize(sectionFontSize).TextColor(t.TextMuted).SingleLine().Grow(1).MinWidth(0)
			if v.actions != nil && w.tools.sideHovered {
				v.actions(c)
			}
		})
		ui.Column(c.Key("view-" + v.id)).Grow(1).FillWidth().MinHeight(0).Children(func() { v.view(c) })
	})
}

func (w *Workspace) explorerView(c *ui.Context) {
	t := c.Theme()
	collapsed := w.tools.explorerCollapsed
	ui.Row(c).Height(sectionHeight).Shrink(0).FillWidth().AlignItems(ui.Center).Padding(0, sideActionsPadding, 0, sectionPadding).Gap(sectionGap).Children(func() {
		head := ui.ButtonBase(c.Key("explorer-root")).Grow(1).MinWidth(0).FillHeight().AlignItems(ui.Center).Justify(ui.Start).Gap(sectionGap).
			Label(w.Name).Expanded(!collapsed).Radius(iconButtonRadius)
		head.Children(func() {
			chevron := icons.ChevronDown
			if collapsed {
				chevron = icons.ChevronRight
			}
			ui.Icon(c, chevron).Size(sectionChevron, sectionChevron).TextColor(t.TextMuted)
			ui.Text(c, strings.ToUpper(w.Name)).FontSize(sectionFontSize).Bold().SingleLine().MinWidth(0)
		})
		head.OnClick(func() { w.tools.explorerCollapsed = !collapsed })
		head.ContextMenu(func(m *ui.Menu) { w.treeMenu(m, w.Root) })
		if !w.tools.sideHovered {
			return
		}
		iconButton(c, "new-file", "New File...", w.withKeys("New File...", "explorer.newFile"), icons.FileAdd, false, w.NewFile)
		iconButton(c, "new-folder", "New Folder...", w.withKeys("New Folder...", "explorer.newFolder"), icons.FolderAdd, false, w.NewFolder)
		iconButton(c, "refresh", "Refresh Explorer", "Refresh Explorer", icons.Refresh, false, w.RefreshExplorer)
		iconButton(c, "collapse", "Collapse Folders in Explorer", "Collapse Folders in Explorer", icons.CollapseAll, false, w.tree.CollapseAll)
	})
	if collapsed {
		ui.Box(c).Grow(1)
	} else {
		w.tree.View(c, c.Vibrancy(), filetreeHandlers(w))
	}
	w.scratchesSection(c)
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
	go func() {
		if out, err := exec.Command("open", "-a", terminalApp, dir).CombinedOutput(); err != nil {
			w.showError("Cannot open Terminal", errorWithOutput(err, out))
		}
	}()
}
