package workspace

import (
	"slices"

	"github.com/ZerGo0/editor/internal/layout"
	"github.com/egoist/mygo/ui"
)

const framesToSettleFocus = 3

type focusReturn struct {
	held   bool
	closed int
	part   part
	tab    *layout.Tab
	last   part
	lastIn *layout.Tab
}

func (w *Workspace) surfaceOpen() bool {
	return w.palette.IsOpen() || w.searchOpen() || w.prompt.open
}

func (w *Workspace) inPart() bool {
	return w.focusedTab != nil || w.focus.side || w.focus.panel
}

func (w *Workspace) holdFocus() {
	b := &w.back
	if !b.held && w.inPart() {
		b.last, b.lastIn = w.currentPart(), w.focusedTab
	}
	if !w.surfaceOpen() {
		return
	}
	if !b.held {
		b.held, b.part, b.tab = true, b.last, b.lastIn
	}
	b.closed = 0
}

func (w *Workspace) releaseFocus(c *ui.Context) {
	if !w.back.held || w.surfaceOpen() {
		return
	}
	w.back.closed++
	c.Invalidate()
}

func (w *Workspace) returnFocus(c *ui.Context) {
	b := &w.back
	if !b.held || b.closed < framesToSettleFocus {
		return
	}
	b.held = false
	if w.focusTab != nil || w.focus.want != partNone || w.inPart() {
		return
	}
	c.Invalidate()
	if b.part == partEditor && slices.Contains(w.tabs(), b.tab) {
		w.focusTab = b.tab
		return
	}
	w.focusPart(b.part)
}
