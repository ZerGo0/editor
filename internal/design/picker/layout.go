package picker

import (
	"math"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

const resizeLabel = "Resize Dialog "

type Layout struct {
	Width   int `json:"width,omitempty"`
	Height  int `json:"height,omitempty"`
	Preview int `json:"preview,omitempty"`
	X       int `json:"x,omitempty"`
	Y       int `json:"y,omitempty"`
}

type handle struct {
	name   string
	x, y   float32
	cursor ui.Cursor
}

var handles = [...]handle{
	{"Top", 0, -1, ui.CursorResizeNS},
	{"Bottom", 0, 1, ui.CursorResizeNS},
	{"Left", -1, 0, ui.CursorResizeEW},
	{"Right", 1, 0, ui.CursorResizeEW},
	{"Top Left", -1, -1, ui.CursorResizeNWSE},
	{"Bottom Right", 1, 1, ui.CursorResizeNWSE},
	{"Top Right", 1, -1, ui.CursorResizeNESW},
	{"Bottom Left", -1, 1, ui.CursorResizeNESW},
}

func whole(v float32) float32 { return float32(math.Round(float64(v))) }

func fit(v, lo, win float32) float32 {
	if win <= 0 {
		return max(v, lo)
	}
	hi := win - 2*token.GapControl.Px()
	return max(min(v, hi), min(lo, hi))
}

func centered(v, win float32) float32 {
	if win > 0 && int(win-v)%2 != 0 {
		return v - 1
	}
	return v
}

func (l *Layout) frame(win ui.Rect) (w, h float32) {
	w, h = token.SearchPopupWidth.Px(), token.SearchPopupHeight.Px()
	if l.Width > 0 {
		w = float32(l.Width)
	}
	if l.Height > 0 {
		h = float32(l.Height)
	}
	w, h = fit(w, token.SearchPopupMinWidth.Px(), win.W), fit(h, token.SearchPopupMinHeight.Px(), win.H)
	return centered(w, win.W), centered(h, win.H)
}

func (l *Layout) offset(win ui.Rect, w, h float32) (x, y float32) {
	if win.W > 0 && win.H > 0 {
		l.X, l.Y = int(within(float32(l.X), win.W, w)), int(within(float32(l.Y), win.H, h))
	}
	return float32(l.X), float32(l.Y)
}

func within(v, win, size float32) float32 {
	room := max(0, whole((win-size)/2-token.GapControl.Px()))
	return max(min(v, room), -room)
}

func (l *Layout) Grip(el ui.Element) {
	if el.DoubleClicked() {
		l.X, l.Y = 0, 0
		return
	}
	if dx, dy, ok := el.Dragged(); ok {
		l.X += int(whole(dx))
		l.Y += int(whole(dy))
	}
}

func (l *Layout) previewHeight() float32 {
	if l.Preview > 0 {
		return float32(l.Preview)
	}
	return token.SearchPreviewHeight.Px()
}

func (l *Layout) resize(win, panel ui.Rect, growW, growH float32) {
	if growW != 0 {
		l.Width = int(fit(whole(panel.W+growW), token.SearchPopupMinWidth.Px(), win.W))
	}
	if growH != 0 {
		l.Height = int(fit(whole(panel.H+growH), token.SearchPopupMinHeight.Px(), win.H))
	}
}

func (l *Layout) resizePreview(current, results, dy float32) {
	lo := token.PaneMinHeight.Px()
	hi := current + results - lo
	l.Preview = int(max(min(whole(current-dy), hi), lo))
}

func (l *Layout) sashes(c *ui.Context, win, panel ui.Rect) {
	p := token.Of(c)
	corner, thick := token.SashCorner.Px(), token.SashSize.Px()
	for _, h := range handles {
		el := ui.Box(c.Key("resize-" + h.name)).Absolute().Cursor(h.cursor).ZIndex(token.LayerSticky.Z()).Role(ui.RoleSplitter).Label(resizeLabel + h.name)
		switch {
		case h.x == 0:
			el.Left(corner).Right(corner).Height(thick)
		case h.y == 0:
			el.Top(corner).Bottom(corner).Width(thick)
		default:
			el.Width(corner).Height(corner)
		}
		if h.x < 0 {
			el.Left(0)
		} else if h.x > 0 {
			el.Right(0)
		}
		if h.y < 0 {
			el.Top(0)
		} else if h.y > 0 {
			el.Bottom(0)
		}
		if el.Hovered() || el.Dragging() {
			el.Background(p.Accent.Focus)
		}
		if dx, dy, ok := el.Dragged(); ok {
			l.resize(win, panel, h.x*dx*2, h.y*dy*2)
		}
	}
}
