package workspace

import (
	"github.com/ZerGo0/editor/internal/design/chrome"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/ZerGo0/editor/internal/layout"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	halfPercent      = 50
	titleSeparator   = " - "
	navigateBackID   = "workbench.action.navigateBack"
	navigateFwdID    = "workbench.action.navigateForward"
	searchID         = "workbench.action.searchEverywhere"
	toggleSidebarID  = "workbench.action.toggleSidebarVisibility"
	togglePanelID    = "workbench.action.togglePanel"
	splitRightID     = "workbench.action.splitEditorRight"
	splitRightLabel  = "Split Right"
	sidebarLabel     = "Toggle Primary Side Bar"
	panelToggleLabel = "Toggle Panel"
	trafficLightSize = 14
)

func TrafficLightPosition() *mygo.Point {
	inset := (TitleBarHeight - trafficLightSize) / 2
	return &mygo.Point{X: inset, Y: inset}
}

func (w *Workspace) titleBar(c *ui.Context) {
	p := token.Of(c)
	bar := c.TitleBar()
	left, right := bar.Left+token.PadRow.Px(), bar.Right+token.PadRow.Px()
	row := ui.Row(c).Height(token.TitleBarHeight.Px()).Shrink(0).FillWidth().AlignItems(ui.Center).Padding(token.BorderWidth.Px(), right, 0, left).
		Background(p.Layer.TitleBar).BorderWidth(0, 0, token.BorderWidth.Px(), 0).BorderColor(p.Border.Default).DragWindow()
	row.Children(func() {
		ui.Box(c).Grow(1).Basis(0)
		ui.Row(c).Width(token.CommandCenterWidth.Px()).MaxWidthPercent(halfPercent).Shrink(1).MinWidth(0).AlignItems(ui.Center).
			Gap(token.GapInline.Px()).Children(func() {
			chrome.NavButton(c, icons.ArrowLeft, navigateBackID, func(m *ui.Menu) { w.historyMenu(m, true) })
			chrome.NavButton(c, icons.ArrowRight, navigateFwdID, func(m *ui.Menu) { w.historyMenu(m, false) })
			chrome.CommandCenter(c, w.Name, searchID)
		})
		ui.Row(c).Grow(1).Basis(max(left-right, 0)).Justify(ui.End).AlignItems(ui.Center).Gap(token.GapTight.Px()).
			Role(ui.RoleToolbar).Label("Layout Controls").Children(func() {
			w.iconButton(c, icons.Sidebar, sidebarLabel, toggleSidebarID, !w.sidebarHidden, w.ToggleSidebar)
			w.iconButton(c, icons.Panel, panelToggleLabel, togglePanelID, w.tools.panelOpen, w.TogglePanel)
			w.iconButton(c, icons.Split, splitRightLabel, splitRightID, false, func() { w.SplitEditor(layout.Right) })
		})
	})
}

type titleState struct {
	text, file string
}

func (w *Workspace) syncTitle() {
	if w.win == nil {
		return
	}
	title, file := w.Name, ""
	if t := w.active.ActiveTab(); t != nil {
		title = w.tabTitle(t.Path) + titleSeparator + w.Name
		if onDisk(t.Path) {
			file = t.Path
		}
	}
	if title != w.title.text {
		w.title.text = title
		w.win.SetTitle(title)
	}
	if file != w.title.file {
		w.title.file = file
		w.win.SetRepresentedFile(file)
	}
}
