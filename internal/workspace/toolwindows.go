package workspace

import (
	"slices"

	"github.com/ZerGo0/editor/internal/design/chrome"
	"github.com/ZerGo0/editor/internal/design/pane"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/ZerGo0/editor/internal/session"
	"github.com/egoist/mygo/ui"
)

const (
	ExplorerView          = "explorer"
	defaultPanelPercent   = 70
	defaultSidePanel      = 65
	panelLabel            = "Panel"
	togglePanelPositionID = "workbench.action.togglePanelPosition"
	maximizePanelLabel    = "Maximize Panel Size"
	restorePanelLabel     = "Restore Panel Size"
	closePanelLabel       = "Close Panel"
	panelRightLabel       = "Move Panel Right"
	panelBottomLabel      = "Move Panel to Bottom"
)

type toolView struct {
	id, title string
	command   string
	icon      *ui.SVG
	view      func(c *ui.Context)
	badge     func() int
	actions   func(c *ui.Context)
}

type toolState struct {
	side              string
	sideViews         []toolView
	panels            []toolView
	panelOpen         bool
	panel             string
	panelPercent      float32
	panelMax          bool
	panelRight        bool
	sidePercent       float32
	hiddenViews       map[string]bool
	explorerCollapsed bool
	openEditors       bool
	openList          ui.ListState
	revealed          string
	collapsed         map[string]bool
}

func (w *Workspace) initTools(saved session.Project) {
	w.tools.sideViews = []toolView{{id: ExplorerView, title: "Explorer", command: "workbench.view.explorer", icon: icons.Explorer, view: w.explorerView}}
	w.tools.side = cmpOr(saved.SideView, ExplorerView)
	w.tools.collapsed = map[string]bool{}
	for _, id := range saved.Collapsed {
		w.tools.collapsed[id] = true
	}
	w.tools.hiddenViews = map[string]bool{}
	for _, id := range saved.HiddenViews {
		w.tools.hiddenViews[id] = true
	}
	for _, id := range saved.StatusHidden {
		w.statusHidden[id] = true
	}
	w.tools.openEditors, w.tools.panelRight, w.tools.sidePercent = saved.OpenEditors, saved.PanelRight, defaultSidePanel
	w.tools.panelOpen, w.tools.panel = saved.PanelOpen, saved.Panel
	w.tools.panelPercent = saved.PanelPercent
	if w.tools.panelPercent <= 0 {
		w.tools.panelPercent = defaultPanelPercent
	}
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func (w *Workspace) snapshotTools(p *session.Project) {
	p.SideView, p.PanelOpen, p.Panel, p.PanelPercent = w.tools.side, w.tools.panelOpen, w.tools.panel, w.tools.panelPercent
	p.PanelRight, p.OpenEditors = w.tools.panelRight, w.tools.openEditors
	p.Collapsed, p.HiddenViews, p.StatusHidden = trueKeys(w.tools.collapsed), trueKeys(w.tools.hiddenViews), trueKeys(w.statusHidden)
}

func trueKeys(m map[string]bool) []string {
	var out []string
	for k, on := range m {
		if on {
			out = append(out, k)
		}
	}
	slices.Sort(out)
	return out
}

func (w *Workspace) addSideView(v toolView) {
	w.tools.sideViews = append(w.tools.sideViews, v)
}

func (w *Workspace) addPanel(v toolView) {
	w.tools.panels = append(w.tools.panels, v)
}

func findView(views []toolView, id string) *toolView {
	for i := range views {
		if views[i].id == id {
			return &views[i]
		}
	}
	if len(views) > 0 {
		return &views[0]
	}
	return nil
}

func (w *Workspace) ShowSideView(id string) {
	w.sidebarHidden = false
	w.tools.side = id
	if w.tools.hiddenViews[id] {
		w.tools.hiddenViews[id] = false
	}
}

func (w *Workspace) ShowPanel(id string) {
	if w.tools.panelOpen && w.tools.panel == id {
		return
	}
	w.tools.panelOpen, w.tools.panel = true, id
}

func (w *Workspace) TogglePanel() {
	w.tools.panelOpen = !w.tools.panelOpen
	if !w.tools.panelOpen {
		w.tools.panelMax = false
		w.focusTab = w.active.ActiveTab()
	}
}

func (w *Workspace) ToggleMaximizedPanel() {
	w.tools.panelOpen = true
	w.tools.panelMax = !w.tools.panelMax
}

func (w *Workspace) TogglePanelPosition() {
	w.tools.panelRight = !w.tools.panelRight
	w.tools.panelOpen = true
}

func (w *Workspace) withPanel(c *ui.Context, editors func()) {
	switch {
	case !w.tools.panelOpen || len(w.tools.panels) == 0:
		ui.Column(c).Grow(1).FillWidth().MinHeight(0).MinWidth(0).Children(editors)
	case w.tools.panelMax:
		ui.Column(c).Grow(1).FillWidth().MinHeight(0).MinWidth(0).Children(func() { w.panelView(c) })
	case w.tools.panelRight:
		ui.SplitPercent(c, &w.tools.sidePercent, editors, func() { w.panelView(c) }).SplitReset(defaultSidePanel).Grow(1).FillWidth().MinWidth(0)
	default:
		ui.SplitVerticalPercent(c, &w.tools.panelPercent, editors, func() { w.panelView(c) }).SplitReset(defaultPanelPercent).Grow(1).FillWidth().MinWidth(0)
	}
}

func (w *Workspace) panelView(c *ui.Context) {
	p := token.Of(c)
	v := findView(w.tools.panels, w.tools.panel)
	col := ui.Column(c).Fill().MinWidth(0).Background(p.Layer.Panel).BorderColor(p.Border.Default).Role(ui.RoleRegion).Label(panelLabel)
	if w.tools.panelRight {
		col.BorderWidth(0, 0, 0, token.BorderWidth.Px())
	} else {
		col.BorderWidth(token.BorderWidth.Px(), 0, 0, 0)
	}
	w.focus.panel = col.FocusWithin()
	col.Children(func() {
		header := ui.Row(c).Height(token.ViewTitleHeight.Px()).Shrink(0).FillWidth().MinWidth(0).Clip().Padding(0, token.PadRow.Px()).
			Gap(token.GapInline.Px()).AlignItems(ui.Center)
		header.Children(func() {
			selected := slices.IndexFunc(w.tools.panels, func(t toolView) bool { return t.id == v.id })
			tabs := make([]chrome.ViewTab, len(w.tools.panels))
			for i, t := range w.tools.panels {
				tabs[i] = chrome.ViewTab{Title: t.title, Keys: w.keyLabel(t.command)}
				if t.badge != nil {
					tabs[i].Badge = t.badge()
				}
			}
			ui.ScrollHorizontal(c).Shrink(1).MinWidth(0).FillHeight().Children(func() {
				viewTabs := chrome.ViewTabs(c, panelLabel, &selected, tabs)
				w.takeFocus(partPanel, viewTabs)
				if viewTabs.Changed() {
					w.tools.panel = w.tools.panels[selected].id
				}
			})
			gap := ui.Box(c).Grow(1).FillHeight().MinWidth(0)
			if gap.DoubleClicked() {
				w.ToggleMaximizedPanel()
			}
			if v.actions != nil {
				ui.Row(c).Shrink(1).MinWidth(0).Clip().FillHeight().Gap(token.GapTight.Px()).AlignItems(ui.Center).Children(func() { v.actions(c) })
			}
			pane.Separator(c, true)
			w.panelActions(c)
		})
		header.ContextMenu(w.panelMenu)
		ui.Column(c.Key("panel-view-" + v.id)).Grow(1).FillWidth().MinHeight(0).MinWidth(0).Children(func() { v.view(c) })
	})
}

func (w *Workspace) panelActions(c *ui.Context) {
	posIcon, posLabel := icons.SidebarRight, panelRightLabel
	if w.tools.panelRight {
		posIcon, posLabel = icons.Panel, panelBottomLabel
	}
	w.iconButton(c, posIcon, posLabel, togglePanelPositionID, false, w.TogglePanelPosition)
	maxIcon, maxLabel := icons.ChevronUp, maximizePanelLabel
	if w.tools.panelMax {
		maxIcon, maxLabel = icons.ChevronDown, restorePanelLabel
	}
	w.iconButton(c, maxIcon, maxLabel, "workbench.action.toggleMaximizedPanel", false, w.ToggleMaximizedPanel)
	w.iconButton(c, icons.Close, closePanelLabel, togglePanelID, false, w.TogglePanel)
}

func (w *Workspace) panelMenu(m *ui.Menu) {
	for _, p := range w.tools.panels {
		if m.Item(p.title).Radio(p.id == w.tools.panel).Chosen() {
			w.tools.panel = p.id
		}
	}
	m.Separator()
	label := maximizePanelLabel
	if w.tools.panelMax {
		label = restorePanelLabel
	}
	if m.Item(label).Chosen() {
		w.ToggleMaximizedPanel()
	}
	pos := panelRightLabel
	if w.tools.panelRight {
		pos = panelBottomLabel
	}
	w.commandItem(m, pos, togglePanelPositionID)
	w.commandItem(m, closePanelLabel, togglePanelID)
}
