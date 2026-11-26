package gitui

import (
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const percentWhole float32 = 100

type diffBar struct {
	bar    ui.Scrollbar
	active bool
	grab   float32
}

func oldPaneWidth(total, splitPercent float32) float32 {
	room := max(total-token.SeparatorWidth.Px(), 0)
	if splitPercent > 0 {
		return room * min(splitPercent, percentWhole) / percentWhole
	}
	return room / 2
}

func (t *diffTab) oldSideBar(room ui.Rect, style ui.ScrollbarStyle) (ui.Scrollbar, bool) {
	d := t.state
	if d == nil || editor.DiffLayout(t.layout) != editor.DiffSplit || room.W <= 0 {
		return ui.Scrollbar{}, false
	}
	view := float32(d.ViewportHeight())
	content := max(float32(d.LineTop(editor.DiffNew, d.New.LineCount())), view)
	return ui.Scrollbar{
		Box:  ui.Rect{X: room.X, Y: room.Y, W: oldPaneWidth(room.W, float32(d.SplitPercent)), H: view},
		View: view, Content: content, Offset: float32(d.ScrollY()), Style: style,
	}, true
}

func (t *diffTab) diffWithBars(c *ui.Context) {
	d := t.state
	pal := token.Of(c)
	style := token.ScrollbarStyle(pal)
	wrap := ui.Column(c.Key("diff-bars")).Grow(1).FillWidth().MinHeight(0)
	wrap.Children(func() {
		editor.DiffView(c.Key("diff"), d).Grow(1).FillWidth()
		ui.Box(c.Key("diff-old-bar")).Absolute().Top(0).Bottom(0).Left(0).Right(0).AccessHidden(true).
			Draw(func(p *ui.Painter, r ui.Rect) { t.paintOldBar(p, r, style, wrap.Hovered()) }).
			HitTest(t.hitOldBar).HandleInput(t.inputOldBar)
	})
}

func (t *diffTab) paintOldBar(p *ui.Painter, r ui.Rect, style ui.ScrollbarStyle, hovered bool) {
	bar, ok := t.oldSideBar(r, style)
	t.bar.bar = bar
	if !ok || !bar.Scrolls() {
		return
	}
	p.ScrollbarTrack(bar)
	if hovered || t.bar.active {
		p.ScrollbarThumb(bar, ui.ScrollbarState{Dragging: t.bar.active})
	}
}

func (t *diffTab) localBar() (ui.Scrollbar, bool) {
	bar := t.bar.bar
	if !bar.Scrolls() {
		return bar, false
	}
	bar.Box.X, bar.Box.Y = 0, 0
	return bar, true
}

func (t *diffTab) hitOldBar(x, y float32, _ ui.Rect) bool {
	bar, ok := t.localBar()
	if !ok {
		return t.bar.active
	}
	track := bar.Style.TrackWidth
	return t.bar.active || x >= bar.Box.W-track && x <= bar.Box.W && y >= 0 && y <= bar.Box.H
}

func (t *diffTab) inputOldBar(ev ui.InputEvent) bool {
	bar, ok := t.localBar()
	if !ok {
		return false
	}
	switch ev.Kind {
	case ui.InputPointerDown:
		start, length := bar.Thumb()
		t.bar.active = true
		if ev.Y >= start && ev.Y <= start+length {
			t.bar.grab = ev.Y - start
		} else {
			t.bar.grab = length / 2
			t.dragOldBar(bar, ev.Y)
		}
		return true
	case ui.InputPointerMove:
		if t.bar.active {
			t.dragOldBar(bar, ev.Y)
			return true
		}
	case ui.InputPointerUp:
		held := t.bar.active
		t.bar.active = false
		return held
	}
	return false
}

func (t *diffTab) dragOldBar(bar ui.Scrollbar, y float32) {
	t.state.SetScrollY(float64(bar.OffsetAt(y - t.bar.grab)))
}
