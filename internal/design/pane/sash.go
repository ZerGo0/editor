package pane

import (
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

type SashOptions struct {
	Label string
	Rows  bool
}

type SashResult struct {
	Delta float32
	Held  bool
	Reset bool
}

func sashAnchor(c *ui.Context, rows bool, before float32) ui.Element {
	size := token.SashSize.Px()
	anchor := ui.Box(c).Shrink(0).ZIndex(token.LayerSticky.Z())
	if rows {
		return anchor.Height(size).FillWidth().Margin(-before, 0, before-size, 0)
	}
	return anchor.Width(size).FillHeight().Margin(0, before-size, 0, -before)
}

func Sash(c *ui.Context, o SashOptions) (ui.Element, SashResult) {
	p := token.Of(c)
	cursor, back, forth := ui.CursorResizeColumn, ui.KeyLeft, ui.KeyRight
	if o.Rows {
		cursor, back, forth = ui.CursorResizeRow, ui.KeyUp, ui.KeyDown
	}
	sash := ui.Box(c).Absolute().Cursor(cursor).ZIndex(token.LayerSticky.Z()).Focusable().
		Role(ui.RoleSplitter).Label(o.Label).Tooltip(o.Label)
	if o.Rows {
		sash.Height(token.SashSize.Px())
	} else {
		sash.Width(token.SashSize.Px())
	}
	dx, dy, held := sash.Dragged()
	if held || sash.Hovered() || sash.FocusVisible() {
		sash.Background(p.Accent.Focus)
	}
	r := SashResult{Held: held, Reset: sash.DoubleClicked()}
	switch {
	case held && o.Rows:
		r.Delta = dy
	case held:
		r.Delta = dx
	}
	step := token.SashKeyStep.Px()
	if sash.Shortcut(0, back) {
		r.Delta -= step
	}
	if sash.Shortcut(0, forth) {
		r.Delta += step
	}
	if sash.Shortcut(0, ui.KeyEnter) {
		r.Reset = true
	}
	return sash, r
}
