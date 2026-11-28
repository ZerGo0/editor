package workspace

import (
	"slices"

	"github.com/egoist/mygo/ui"
)

type part int

const (
	partNone part = iota
	partSide
	partEditor
	partPanel
)

type focusFlags struct {
	side, panel bool
	want        part
}

func (w *Workspace) parts() []part {
	var out []part
	if !w.sidebarHidden {
		out = append(out, partSide)
	}
	out = append(out, partEditor)
	if w.tools.panelOpen && len(w.tools.panels) > 0 {
		out = append(out, partPanel)
	}
	return out
}

func (w *Workspace) currentPart() part {
	switch {
	case w.focus.side:
		return partSide
	case w.focus.panel:
		return partPanel
	}
	return partEditor
}

func (w *Workspace) FocusNextPart() {
	w.cyclePart(1)
}

func (w *Workspace) FocusPreviousPart() {
	w.cyclePart(-1)
}

func (w *Workspace) cyclePart(delta int) {
	ps := w.parts()
	i := max(slices.Index(ps, w.currentPart()), 0)
	w.focusPart(ps[(i+delta+len(ps))%len(ps)])
}

func (w *Workspace) focusPart(p part) {
	switch {
	case p == partEditor:
		w.focusTab = w.active.ActiveTab()
	case p == partSide && focusView(findView(w.tools.sideViews, w.tools.side)):
	case p == partPanel && focusView(findView(w.tools.panels, w.tools.panel)):
	default:
		w.focus.want = p
	}
}

func focusView(v *toolView) bool {
	return v != nil && v.focus != nil && v.focus()
}

func (w *Workspace) focusExplorerTree() bool {
	w.tree.Focus()
	return true
}

func (w *Workspace) FocusSideBar() {
	w.FocusSideView(w.tools.side)
}

func (w *Workspace) FocusSideView(id string) {
	w.ShowSideView(id)
	w.focusPart(partSide)
}

func (w *Workspace) takeFocus(p part, el ui.Element) {
	if w.focus.want == p {
		w.focus.want = partNone
		el.Focus()
	}
}
