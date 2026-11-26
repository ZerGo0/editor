package token

import (
	"math"

	"github.com/egoist/mygo/ui"
)

type Nudge struct{ px float32 }

func (n Nudge) Px() float32 { return n.px }

var TextNudgeY = Nudge{-0.5}

func NudgeText(el ui.Element) ui.Element {
	return el.Translate(0, TextNudgeY.px)
}

func (t TextStyle) WholeText(c *ui.Context, s string) ui.Element {
	span := ui.Span{Text: s, Size: t.size, Weight: t.weight}
	if t.mono {
		span.Font = FontMono
	}
	w, _ := c.MeasureText(0, span)
	return t.Apply(ui.Text(c, s)).MinWidth(float32(math.Ceil(float64(w)))).TextAlign(ui.Center)
}
