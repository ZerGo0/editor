package token

import (
	"math"

	"github.com/egoist/mygo/ui"
)

var TextRise = Size{1}

func (t TextStyle) ApplyLine(el ui.Element) ui.Element {
	t.Apply(el)
	if lift := 2 * t.rise.Px(); lift > 0 {
		el.FixedLineHeight(t.line-lift).Padding(0, 0, lift, 0)
	}
	return el
}

func (t TextStyle) WholeText(c *ui.Context, s string) ui.Element {
	return t.ApplyLine(ui.Text(c, s)).MinWidth(t.wholeWidth(c, s)).TextAlign(ui.Center)
}

func (t TextStyle) WholeTextMax(c *ui.Context, s string, limit float32) (el ui.Element, cut bool) {
	w := t.wholeWidth(c, s)
	return t.ApplyLine(ui.Text(c, s)).MinWidth(min(w, limit)).MaxWidth(limit).TextAlign(ui.Center).SingleLine(), w > limit
}

func (t TextStyle) wholeWidth(c *ui.Context, s string) float32 {
	span := ui.Span{Text: s, Size: t.size, Weight: t.weight}
	if t.mono {
		span.Font = FontMono
	}
	w, _ := c.MeasureText(0, span)
	return float32(math.Ceil(float64(w)))
}
