package workspace

import (
	"os"
	"slices"

	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/chrome"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/fsutil"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/ZerGo0/editor/internal/layout"
	"github.com/ZerGo0/editor/internal/viewer"
	"github.com/ZerGo0/editor/internal/workspace/filetree"
	"github.com/ZerGo0/editor/internal/workspace/tablabel"
	"github.com/egoist/mygo/ui"
)

const (
	tabStripLabel    = "Editor Tabs"
	moreActionsLabel = "More Actions..."
	previewToSideID  = "markdown.showPreviewToSide"
)

type tabDrag struct {
	group *layout.Group
	tab   *layout.Tab
}

type dragState struct {
	seen, shown bool
}

func (w *Workspace) beginDragFrame() {
	w.drag.shown, w.drag.seen = w.drag.seen, false
}

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
		return p
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
		Label:         tabStripLabel,
		OnSelect:      func(i int) { w.activate(g, tabs[i]) },
		OnClose:       closeTab,
		OnAuxClick:    closeTab,
		OnUnpin:       func(i int) { g.SetPinned(tabs[i], false) },
		OnDoubleClick: func(i int) { tabs[i].Preview = false },
		Menu:          func(i int, m *ui.Menu) { w.tabMenu(m, g, tabs[i]) },
		Drag:          func(i int) any { return tabDrag{g, tabs[i]} },
		Dragging:      func(int) { w.drag.seen = true },
		Drop: func(at int, el ui.Element) bool {
			w.dropTarget(el, g, at)
			return dragging(el)
		},
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
		w.Split(false)
	}
	action.MenuIconButton(c, icons.More, moreActionsLabel, action.IconOptions{}, func(m *ui.Menu) { w.groupMenu(m, g) })
}

func (w *Workspace) dropTarget(el ui.Element, g *layout.Group, at int) {
	if d, ok := ui.Drop[tabDrag](el); ok {
		w.moveTab(d, g, at)
	}
	if paths, ok := ui.Drop[filetree.Paths](el); ok {
		w.openDropped(g, paths)
	}
	if files := el.DroppedFiles(); len(files) > 0 {
		w.openDropped(g, files)
	}
}

func dragging(el ui.Element) bool {
	_, tab := ui.DragOver[tabDrag](el)
	_, files := ui.DragOver[filetree.Paths](el)
	return tab || files
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
		w.Split(false)
	}
	if m.Item("Split Down").Chosen() {
		w.activate(g, tab)
		w.Split(true)
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
		w.Split(false)
	}
	if m.Item("Split Down").Disabled(g.ActiveTab() == nil).Chosen() {
		w.active = g
		w.Split(true)
	}
}

func (w *Workspace) moveTab(d tabDrag, to *layout.Group, at int) {
	from, tab := d.group, d.tab
	i := from.Index(tab)
	if i < 0 {
		return
	}
	if from == to {
		to.Move(tab, at)
		w.activate(to, tab)
		return
	}
	from.Close(i)
	if j := to.Find(tab.Path); j >= 0 {
		tab = to.Tabs[j]
	} else {
		keepPin := tab.Pinned && at >= len(to.Tabs)
		to.Insert(tab, at)
		if keepPin {
			to.SetPinned(tab, true)
		}
	}
	if len(from.Tabs) == 0 {
		w.layout.Remove(from)
	}
	w.activate(to, tab)
}

func (w *Workspace) splitWith(d tabDrag, g *layout.Group, vertical, before bool) {
	if d.group == g && len(g.Tabs) == 1 {
		return
	}
	added := w.newGroup()
	if !w.layout.Split(g, added, vertical) {
		return
	}
	if before {
		swapSplit(w.layout, g, added)
	}
	w.moveTab(d, added, 0)
}

func swapSplit(n *layout.Node, first, second *layout.Group) bool {
	if n.Group != nil {
		return false
	}
	if n.First.Group == first && n.Second.Group == second {
		n.First, n.Second = n.Second, n.First
		return true
	}
	return swapSplit(n.First, first, second) || swapSplit(n.Second, first, second)
}

func (w *Workspace) openDropped(g *layout.Group, paths []string) {
	w.active = g
	for _, p := range paths {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			w.OpenFile(p, false, true)
		}
	}
}

func previewable(p string) bool {
	return viewer.Detect(p, nil).Previewable()
}
