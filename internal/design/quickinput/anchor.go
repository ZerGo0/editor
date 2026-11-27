package quickinput

import (
	"math"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

type frame struct {
	x, y, w float32
}

func whole(v float32) float32 { return float32(math.Round(float64(v))) }

func anchored(bar, win ui.Rect) (frame, bool) {
	if bar.W <= 0 || bar.H <= 0 {
		return frame{}, false
	}
	w := whole(max(bar.W, token.QuickInputMinWidth.Px()))
	x := whole(bar.X + (bar.W-w)/2)
	if win.W > 0 {
		x = max(min(x, win.W-w), 0)
	}
	return frame{x: x, y: whole(bar.Y + bar.H + token.OffsetPopover.Px()), w: w}, true
}
