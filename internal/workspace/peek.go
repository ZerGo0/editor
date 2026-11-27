package workspace

import (
	"context"
	"slices"

	"github.com/ZerGo0/editor/internal/layout"
	"github.com/ZerGo0/editor/internal/workspace/symbolnav"
	"github.com/egoist/mygo/ui"
)

type peekState struct {
	view   *symbolnav.Peek
	tab    *layout.Tab
	line   int
	cancel context.CancelFunc
	gen    int
}

func (w *Workspace) initPeek() {
	w.peek.view = symbolnav.New(symbolnav.Env{
		Host:  w,
		Open:  w.openLocation,
		Focus: func() { w.focusTab = w.peek.tab },
	})
}

func (w *Workspace) peekView(c *ui.Context) {
	pk := &w.peek
	if !pk.view.Visible() {
		return
	}
	t := pk.tab
	if t == nil || t.Editor == nil || !w.isShown(t) {
		pk.view.Close()
		return
	}
	st := t.Editor
	line := min(pk.line, st.Buffer.LineCount()-1)
	area := c.Resolve(t.Handle).Bounds()
	shift := area.Y - float32(st.ScrollY())
	pk.view.View(c, symbolnav.Anchor{Area: area, Top: shift + float32(st.LineTop(line)), Bottom: shift + float32(st.LineTop(line+1))})
}

func (w *Workspace) isShown(t *layout.Tab) bool {
	return slices.ContainsFunc(w.layout.Groups(), func(g *layout.Group) bool { return g.ActiveTab() == t })
}

func (w *Workspace) referenceSearchVisible() bool {
	return w.peek.view.Visible()
}
