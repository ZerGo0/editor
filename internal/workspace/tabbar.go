package workspace

import (
	"slices"

	"github.com/ZerGo0/editor/internal/fsutil"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/ZerGo0/editor/internal/layout"
	"github.com/ZerGo0/editor/internal/workspace/filetree"
	"github.com/egoist/mygo/ui"
)

const (
	tabBarHeight    = 35
	tabIconSize     = 16
	tabButtonSize   = 20
	tabButtonIcon   = 14
	tabDotIcon      = 8
	tabPaddingStart = 10
	tabPaddingEnd   = 4
	tabGap          = 6
	tabAccentHeight = 2
	tabDropWidth    = 2
	middleButton    = 2
	hiddenOpacity   = 0
	pinnedLabel     = "pinned"
)

type tabDrag struct {
	group *layout.Group
	tab   *layout.Tab
}

func (w *Workspace) tabBar(c *ui.Context, g *layout.Group) {
	t := c.Theme()
	ui.Row(c).Height(tabBarHeight).Shrink(0).FillWidth().Background(t.Surface).Children(func() {
		ui.ScrollHorizontal(c).Shrink(1).MinWidth(0).FillHeight().Children(func() {
			tabs := ui.TabsBase(c, &g.Active, len(g.Tabs))
			tabs.List.FillHeight().AlignItems(ui.Stretch).Children(func() {
				for i, tab := range slices.Clone(g.Tabs) {
					w.tab(c, g, tabs.Tab(i), i, tab)
				}
			})
			if tabs.List.Changed() {
				w.activate(g, g.ActiveTab())
			}
		})
		rest := ui.Box(c).Grow(1).FillHeight().BorderWidth(0, 0, 1, 0).BorderColor(t.Border)
		if rest.DoubleClicked() {
			w.active = g
			w.NewFile()
		}
		rest.ContextMenu(func(m *ui.Menu) { w.groupMenu(m, g) })
		w.dropTarget(rest, g, len(g.Tabs))
		w.editorActions(c, g)
	})
}

func (w *Workspace) dropTarget(el ui.Element, g *layout.Group, at int) {
	if d, ok := ui.Drop[tabDrag](el); ok {
		w.moveTab(d, g, at)
	}
	if paths, ok := ui.Drop[filetree.Paths](el); ok {
		w.openDropped(g, paths)
	}
}

func dragging(el ui.Element) bool {
	_, tab := ui.DragOver[tabDrag](el)
	_, files := ui.DragOver[filetree.Paths](el)
	return tab || files
}

func (w *Workspace) tab(c *ui.Context, g *layout.Group, el ui.Element, i int, tab *layout.Tab) {
	t := c.Theme()
	name := w.tabTitle(tab.Path)
	active := g.ActiveTab() == tab
	hovered := el.Hovered()
	el.Gap(tabGap).Padding(0, tabPaddingEnd, 0, tabPaddingStart).AlignItems(ui.Center).Label(name).Drag(tabDrag{g, tab})
	if tab.Pinned {
		el.Description(pinnedLabel)
	}
	switch {
	case active:
		el.Background(t.Background).BorderWidth(0, 1, 0, 0)
	case hovered:
		el.Background(t.SurfaceHover).BorderWidth(0, 1, 1, 0)
	default:
		el.BorderWidth(0, 1, 1, 0)
	}
	el.BorderColor(t.Border)
	modified := false
	if d := w.doc(tab.Path); d != nil {
		modified = d.Modified()
	}
	el.Children(func() {
		if active && g == w.active {
			ui.Box(c).Absolute().Top(0).Left(0).Right(0).Height(tabAccentHeight).Background(activeAccent(c))
		}
		if dragging(el) {
			ui.Box(c).Absolute().Top(0).Bottom(0).Left(0).Width(tabDropWidth).Background(t.Accent)
		}
		w.tabIcon(c, tab.Path)
		text := ui.Text(c, name).FontSize(uiFontSize).SingleLine()
		if tab.Preview {
			text.Italic()
		}
		if !active {
			text.TextColor(t.TextMuted)
		}
		w.tabButton(c, g, tab, name, active, hovered, modified)
	})
	if b, ok := el.PressedButton(); ok && b == middleButton {
		w.requestClose(g, []*layout.Tab{tab})
	}
	if el.DoubleClicked() {
		tab.Preview = false
	}
	el.ContextMenu(func(m *ui.Menu) { w.tabMenu(m, g, tab) })
	w.dropTarget(el, g, i)
}

func (w *Workspace) tabIcon(c *ui.Context, p string) {
	if !isVirtual(p) {
		icons.FileIcon(c, w.tabTitle(p), false, false, tabIconSize)
		return
	}
	svg := icons.File
	if p == SettingsTab || p == KeybindingsTab {
		svg = icons.Settings
	}
	ui.Icon(c, svg).Size(tabIconSize, tabIconSize).TextColor(c.Theme().TextMuted).Shrink(0)
}

func (w *Workspace) tabButton(c *ui.Context, g *layout.Group, tab *layout.Tab, name string, active, tabHovered, modified bool) {
	t := c.Theme()
	label, svg, size := "Close "+name, icons.Close, float32(tabButtonIcon)
	if tab.Pinned {
		label, svg = "Unpin "+name, icons.Pin
	}
	b := ui.ButtonBase(c.Key("close")).Size(tabButtonSize, tabButtonSize).Radius(iconButtonRadius).Center().Shrink(0).
		TextColor(t.TextMuted).Label(label).Tooltip(label)
	hovered := pressable(b, t)
	if hovered {
		b.TextColor(t.Text)
	}
	switch {
	case tab.Pinned:
	case modified && !hovered:
		svg, size = icons.Dot, tabDotIcon
	case !active && !tabHovered:
		b.Opacity(hiddenOpacity)
	}
	b.Children(func() { ui.Icon(c, svg).Size(size, size) })
	b.OnClick(func() {
		if tab.Pinned {
			g.SetPinned(tab, false)
			return
		}
		w.requestClose(g, []*layout.Tab{tab})
	})
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
	if !isVirtual(tab.Path) {
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
	w.commandItem(m, "New File...", "explorer.newFile")
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

func (w *Workspace) openDropped(g *layout.Group, paths []string) {
	w.active = g
	for _, p := range paths {
		if !w.tree.IsDir(p) {
			w.OpenFile(p, false, true)
		}
	}
}
