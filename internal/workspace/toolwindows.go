package workspace

import (
	"strconv"
	"strings"

	"github.com/ZerGo0/editor/internal/icons"
	"github.com/ZerGo0/editor/internal/session"
	"github.com/egoist/mygo/ui"
)

const (
	ExplorerView        = "explorer"
	defaultPanelPercent = 70
	panelHeaderHeight   = 35
	panelTitlePadding   = 8
	panelTitleGap       = 6
	panelHeaderPadding  = 8
	panelHeaderGap      = 2
	panelUnderline      = 1
	countFontSize       = 10
	countPadding        = 5
	countHeight         = 16
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
	hiddenViews       map[string]bool
	explorerCollapsed bool
	sideHovered       bool
	collapsed         map[string]bool
}

func (w *Workspace) initTools(saved session.Project) {
	w.tools.sideViews = []toolView{{id: ExplorerView, title: "Explorer", command: "workbench.view.explorer", icon: icons.Explorer, view: w.explorerView}}
	w.tools.side = cmpOr(saved.SideView, ExplorerView)
	w.tools.collapsed = map[string]bool{}
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

func countBadge(c *ui.Context, n int) {
	if n <= 0 {
		return
	}
	t := c.Theme()
	ui.Text(c, strconv.Itoa(n)).FontSize(countFontSize).TextColor(t.Text).Background(t.SurfacePressed).
		Radius(countHeight/2).Height(countHeight).MinWidth(countHeight).TextAlign(ui.Center).Padding(0, countPadding).Shrink(0)
}

func (w *Workspace) withPanel(c *ui.Context, editors func()) {
	switch {
	case !w.tools.panelOpen || len(w.tools.panels) == 0:
		ui.Column(c).Grow(1).FillWidth().MinHeight(0).Children(editors)
	case w.tools.panelMax:
		ui.Column(c).Grow(1).FillWidth().MinHeight(0).Children(func() { w.panelView(c) })
	default:
		ui.SplitVerticalPercent(c, &w.tools.panelPercent, editors, func() { w.panelView(c) }).SplitReset(defaultPanelPercent).Grow(1).FillWidth()
	}
}

func (w *Workspace) panelView(c *ui.Context) {
	t := c.Theme()
	v := findView(w.tools.panels, w.tools.panel)
	ui.Column(c).Fill().Background(t.Background).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
		header := ui.Row(c).Height(panelHeaderHeight).Shrink(0).FillWidth().Padding(0, panelHeaderPadding).Gap(panelHeaderGap).AlignItems(ui.Center)
		header.Children(func() {
			for _, p := range w.tools.panels {
				w.panelTitle(c, p, p.id == v.id)
			}
			gap := ui.Box(c).Grow(1).FillHeight()
			if gap.DoubleClicked() {
				w.ToggleMaximizedPanel()
			}
			if v.actions != nil {
				v.actions(c)
			}
			maxIcon, maxLabel := icons.ChevronUp, "Maximize Panel Size"
			if w.tools.panelMax {
				maxIcon, maxLabel = icons.ChevronDown, "Restore Panel Size"
			}
			iconButton(c, "maximize-panel", maxLabel, maxLabel, maxIcon, false, w.ToggleMaximizedPanel)
			iconButton(c, "close-panel", "Close Panel", w.withKeys("Close Panel", "workbench.action.togglePanel"), icons.Close, false, w.TogglePanel)
		})
		header.ContextMenu(w.panelMenu)
		ui.Column(c.Key("panel-view-" + v.id)).Grow(1).FillWidth().MinHeight(0).Children(func() { v.view(c) })
	})
}

func (w *Workspace) panelTitle(c *ui.Context, p toolView, active bool) {
	t := c.Theme()
	b := ui.ButtonBase(c.Key("panel-"+p.id)).FillHeight().Padding(0, panelTitlePadding).Gap(panelTitleGap).AlignItems(ui.Center).
		Label(p.title).Tooltip(w.withKeys(p.title, p.command)).AccessSelected(active)
	color := t.TextMuted
	if active || b.Hovered() || b.Pressed() {
		color = t.Text
	}
	b.Children(func() {
		ui.Column(c).FillHeight().Justify(ui.Center).Children(func() {
			ui.Text(c, strings.ToUpper(p.title)).FontSize(sectionFontSize).TextColor(color).SingleLine()
			line := ui.Box(c).Absolute().Left(0).Right(0).Bottom(0).Height(panelUnderline)
			if active {
				line.Background(activeAccent(c))
			}
		})
		if p.badge != nil {
			countBadge(c, p.badge())
		}
	})
	b.OnClick(func() { w.tools.panel = p.id })
}

func (w *Workspace) panelMenu(m *ui.Menu) {
	for _, p := range w.tools.panels {
		if m.Item(p.title).Radio(p.id == w.tools.panel).Chosen() {
			w.tools.panel = p.id
		}
	}
	m.Separator()
	label := "Maximize Panel Size"
	if w.tools.panelMax {
		label = "Restore Panel Size"
	}
	if m.Item(label).Chosen() {
		w.ToggleMaximizedPanel()
	}
	w.commandItem(m, "Close Panel", "workbench.action.togglePanel")
}
