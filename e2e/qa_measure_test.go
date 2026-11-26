//go:build e2e

package e2e

import (
	"sync"

	"github.com/ZerGo0/editor/e2e/qa"
	"github.com/ZerGo0/editor/internal/design"
	"github.com/egoist/mygo/ui"
)

const (
	measureWidth  = 400
	measureHeight = 300
)

var measureMu sync.Mutex

func uiTextMeasurer() qa.TextMeasurer {
	return func(texts []string, size float64) []float64 {
		measureMu.Lock()
		defer measureMu.Unlock()
		widths := make([]float64, len(texts))
		view := func(c *ui.Context) {
			design.Apply(c, design.Settings{Appearance: design.AppearanceLight})
			for i, s := range texts {
				w, _ := c.MeasureText(0, ui.Span{Text: s, Size: float32(size)})
				widths[i] = float64(w)
			}
		}
		ui.NewTester(view, measureWidth, measureHeight)
		return widths
	}
}
