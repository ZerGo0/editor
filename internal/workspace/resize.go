package workspace

import (
	"github.com/ZerGo0/editor/internal/design/pane"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/layout"
	"github.com/egoist/mygo/ui"
)

const (
	resizeSidebarLabel = "Resize Primary Side Bar"
	resizePanelLabel   = "Resize Panel"
	resizeGroupsLabel  = "Resize Editor Groups"
	ViewSizeStep       = 60
	fullPercent        = 100
)

type sashStates struct {
	sidebar     pane.SplitState
	panelBottom pane.SplitState
	panelRight  pane.SplitState
	groups      map[*layout.Node]*pane.SplitState
}

func (w *Workspace) sidebarSplit(c *ui.Context) ui.Element {
	return pane.Split(c, &w.sashes.sidebar, &w.sidebarWidth, pane.SplitOptions{
		Label: resizeSidebarLabel, FirstMin: token.SidebarMinWidth, SecondMin: token.EditorMinWidth,
		Reset: defaultSidebarWidth, Snap: pane.SnapFirst, Hidden: &w.sidebarHidden,
	}, func() { w.sidebar(c) }, func() { w.main(c) })
}

func (w *Workspace) panelSplit(c *ui.Context, editors func()) ui.Element {
	hidden := !w.tools.panelOpen
	o := pane.SplitOptions{Label: resizePanelLabel, Percent: true, Snap: pane.SnapSecond, Hidden: &hidden}
	state, size := &w.sashes.panelBottom, &w.tools.panelPercent
	if w.tools.panelRight {
		state, size = &w.sashes.panelRight, &w.tools.sidePercent
		o.FirstMin, o.SecondMin, o.Reset = token.EditorMinWidth, token.PanelMinWidth, defaultSidePanel
	} else {
		o.Vertical, o.FirstMin, o.SecondMin, o.Reset = true, token.EditorMinHeight, token.PanelMinHeight, defaultPanelPercent
	}
	split := pane.Split(c, state, size, o, editors, func() { w.panelView(c) })
	if hidden == w.tools.panelOpen {
		w.setPanelOpen(!hidden)
	}
	return split
}

func (w *Workspace) setPanelOpen(open bool) {
	if open != w.tools.panelOpen {
		w.TogglePanel()
	}
}

func (w *Workspace) groupSash(n *layout.Node) *pane.SplitState {
	if w.sashes.groups == nil {
		w.sashes.groups = map[*layout.Node]*pane.SplitState{}
	}
	s, ok := w.sashes.groups[n]
	if !ok {
		s = &pane.SplitState{}
		w.sashes.groups[n] = s
	}
	return s
}

func (w *Workspace) nodeSplit(c *ui.Context, n *layout.Node, first, second func()) ui.Element {
	o := pane.SplitOptions{Label: resizeGroupsLabel, Vertical: n.Vertical, Percent: true, Divider: true, Reset: halfPercent,
		FirstMin: token.EditorMinWidth, SecondMin: token.EditorMinWidth}
	if n.Vertical {
		o.FirstMin, o.SecondMin = token.EditorMinHeight, token.EditorMinHeight
	}
	return pane.Split(c, w.groupSash(n), &n.Percent, o, first, second)
}

func (w *Workspace) EvenEditorWidths() {
	w.layout.Even()
	w.groupsExpanded = false
}

func (w *Workspace) ToggleEditorWidths() {
	if w.groupsExpanded {
		w.EvenEditorWidths()
		return
	}
	w.layout.Expand(w.active)
	w.groupsExpanded = true
}

func (w *Workspace) ResizeView(delta float32, width, height bool) {
	switch {
	case w.focus.side && !w.sidebarHidden && width:
		w.resizeSidebar(delta)
	case w.focus.panel && w.tools.panelOpen:
		w.resizePanel(delta, width, height)
	default:
		w.resizeGroup(delta, width, height)
	}
}

func (w *Workspace) resizeSidebar(delta float32) {
	hi := w.sidebarWidth + delta
	if total := w.sashes.sidebar.Total(); total > 0 {
		hi = min(hi, total-token.EditorMinWidth.Px())
	}
	w.sidebarWidth = max(hi, token.SidebarMinWidth.Px())
}

func (w *Workspace) resizePanel(delta float32, width, height bool) {
	switch {
	case w.tools.panelRight && width:
		w.tools.sidePercent = shiftPercent(w.tools.sidePercent, -delta, w.sashes.panelRight.Total())
	case !w.tools.panelRight && height:
		w.tools.panelPercent = shiftPercent(w.tools.panelPercent, -delta, w.sashes.panelBottom.Total())
	}
}

func (w *Workspace) resizeGroup(delta float32, width, height bool) {
	steps := w.layout.Path(w.active)
	for i := len(steps) - 1; i >= 0; i-- {
		s := steps[i]
		if s.Node.Vertical && !height || !s.Node.Vertical && !width {
			continue
		}
		d := delta
		if !s.First {
			d = -delta
		}
		s.Node.Percent = shiftPercent(s.Node.Percent, d, w.groupSash(s.Node).Total())
		w.groupsExpanded = false
		return
	}
}

func shiftPercent(p, delta, total float32) float32 {
	if total <= 0 {
		return p
	}
	return min(max(p+delta*fullPercent/total, 0), fullPercent)
}
