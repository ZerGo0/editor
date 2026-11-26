package workspace

import (
	"github.com/ZerGo0/editor/internal/icons"
	searchview "github.com/ZerGo0/editor/internal/search/view"
	"github.com/egoist/mygo/ui"
)

const (
	halfPercent          = 50
	titleBarPadding      = 8
	commandCenterWidth   = 480
	commandCenterMin     = 160
	commandCenterHeight  = 24
	commandCenterRadius  = 6
	commandCenterIcon    = 14
	commandCenterGap     = 6
	commandCenterAlpha   = 0.6
	paletteGap           = 4
	doubleShiftKeyLabel  = "\u21e7\u21e7"
	searchEverywhereName = "Search Everywhere"
)

func (w *Workspace) paletteTop() float32 {
	return (TitleBarHeight-commandCenterHeight)/2 + paletteGap
}

func (w *Workspace) titleBar(c *ui.Context, vibrant bool) {
	t := c.Theme()
	bar := c.TitleBar()
	left, right := bar.Left+titleBarPadding, bar.Right+titleBarPadding
	row := ui.Row(c).Height(TitleBarHeight).Shrink(0).FillWidth().AlignItems(ui.Center).
		Padding(0, right, 0, left).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).DragWindow()
	if !vibrant {
		row.Background(t.Surface)
	}
	row.Children(func() {
		ui.Box(c).Grow(1).Basis(0)
		w.commandCenter(c)
		ui.Row(c).Grow(1).Basis(max(left-right, 0)).Justify(ui.End).AlignItems(ui.Center).Gap(2).Children(func() {
			iconButton(c, "sidebar", "Toggle Sidebar", w.withKeys("Toggle Primary Side Bar", "workbench.action.toggleSidebarVisibility"), icons.Sidebar, !w.sidebarHidden, w.ToggleSidebar)
			iconButton(c, "panel", "Toggle Panel", w.withKeys("Toggle Panel", "workbench.action.togglePanel"), icons.Panel, w.tools.panelOpen, w.TogglePanel)
			iconButton(c, "split", "Split Right", w.withKeys("Split Editor Right", "workbench.action.splitEditorRight"), icons.Split, false, func() { w.Split(false) })
		})
	})
}

func (w *Workspace) commandCenter(c *ui.Context) {
	t := c.Theme()
	b := ui.ButtonBase(c.Key("command-center")).Height(commandCenterHeight).Width(commandCenterWidth).MinWidth(commandCenterMin).Shrink(1).
		Radius(commandCenterRadius).Border(1, t.Border).Background(t.Background.Alpha(commandCenterAlpha)).Center().Gap(commandCenterGap).
		Label(searchEverywhereName).Tooltip(searchEverywhereName + " (" + doubleShiftKeyLabel + ")")
	pressable(b, t)
	b.TextColor(t.TextMuted).Children(func() {
		ui.Icon(c, icons.Search).Size(commandCenterIcon, commandCenterIcon)
		ui.Text(c, w.Name).FontSize(smallFontSize).SingleLine().MinWidth(0)
	})
	b.OnClick(func() { w.FocusSearch(searchview.ScopeAll) })
}
