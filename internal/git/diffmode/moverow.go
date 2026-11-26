package diffmode

import (
	"fmt"
	"time"

	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	selectionFlash = 1500 * time.Millisecond
	moveRowPadding = 8
	moveRowGap     = 4
	moveIconSize   = 12
	moveFontSize   = 12
	scrollAnchor   = 3.0
)

func (e *Engine) moveRow(c *ui.Context, k moveKey) {
	if k.index >= len(e.moves) {
		return
	}
	m := e.moves[k.index]
	label := e.moveLabel(m, k.side)
	down := m.NewStart > m.OldStart
	if k.side == editor.DiffNew {
		down = m.OldStart > m.NewStart
	}
	icon := icons.ChevronUp
	if down {
		icon = icons.ChevronDown
	}
	t := c.Theme()
	b := ui.ButtonBase(c.Key(fmt.Sprintf("move-%d-%d", k.index, k.side))).FillWidth().Height(moveRowHeight).
		Padding(0, moveRowPadding).AlignItems(ui.Center).Justify(ui.Start).Label(label).Background(t.Surface)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		ui.Row(c).Gap(moveRowGap).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, icon).Size(moveIconSize, moveIconSize).TextColor(t.TextMuted)
			ui.Text(c, label).FontSize(moveFontSize).TextColor(t.TextMuted).SingleLine()
		})
	})
	if b.Clicked() {
		e.Jump(k.index, k.side)
	}
}

func (e *Engine) Jump(index int, from editor.DiffSide) {
	if index < 0 || index >= len(e.moves) {
		return
	}
	m, d := e.moves[index], e.State
	side, start, end := editor.DiffNew, m.NewStart, m.NewEnd
	if from == editor.DiffNew {
		side, start, end = editor.DiffOld, m.OldStart, m.OldEnd
	}
	d.SetScrollY(max(d.LineTop(side, start)-d.ViewportHeight()/scrollAnchor, 0))
	flash := editor.DiffLineRange{Side: side, Start: start, End: end - 1}
	d.SetSelectedLines(flash, true)
	e.after(selectionFlash, func() {
		if cur, ok := d.SelectedLines(); ok && cur == flash {
			d.SetSelectedLines(editor.DiffLineRange{}, false)
		}
	})
}
