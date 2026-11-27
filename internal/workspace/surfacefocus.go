package workspace

import (
	"slices"

	"github.com/ZerGo0/editor/internal/layout"
	"github.com/egoist/mygo/ui"
)

type focusReturn struct {
	held   bool
	closed bool
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
	b.closed = false
}

func (w *Workspace) focusClaimed() bool {
	return w.focusTab != nil || w.focus.want != partNone || w.inPart()
}

func (w *Workspace) releaseFocus(c *ui.Context) {
	b := &w.back
	if !b.held || w.surfaceOpen() {
		return
	}
	if !w.focusClaimed() && !b.closed {
		b.closed = true
		c.Invalidate()
		return
	}
	b.held, b.closed = false, false
	if w.focusClaimed() {
		return
	}
	c.Invalidate()
	if b.part == partEditor && slices.Contains(w.tabs(), b.tab) {
		w.focusTab = b.tab
		return
	}
	w.focusPart(b.part)
}
