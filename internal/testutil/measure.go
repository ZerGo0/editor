package testutil

import (
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

func NaturalTextWidth(viewW, viewH int, text string) float32 {
	var w float32
	probe := ui.NewTester(func(c *ui.Context) {
		w, _ = c.MeasureText(0, ui.Span{Text: text, Size: token.TextBody.Px()})
	}, viewW, viewH)
	probe.Frame()
	return w
}
