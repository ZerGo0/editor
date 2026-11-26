package qa

type Rect struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

func (r Rect) Right() float64   { return r.X + r.Width }
func (r Rect) Bottom() float64  { return r.Y + r.Height }
func (r Rect) CenterX() float64 { return r.X + r.Width/2 }
func (r Rect) CenterY() float64 { return r.Y + r.Height/2 }
func (r Rect) Empty() bool      { return r.Width <= 0 || r.Height <= 0 }
func (r Rect) Area() float64    { return max(r.Width, 0) * max(r.Height, 0) }

func (r Rect) Intersect(o Rect) Rect {
	x, y := max(r.X, o.X), max(r.Y, o.Y)
	return Rect{X: x, Y: y, Width: min(r.Right(), o.Right()) - x, Height: min(r.Bottom(), o.Bottom()) - y}
}

func (r Rect) Contains(o Rect, tolerance float64) bool {
	return o.X >= r.X-tolerance && o.Y >= r.Y-tolerance && o.Right() <= r.Right()+tolerance && o.Bottom() <= r.Bottom()+tolerance
}

func (r Rect) Overlaps(o Rect, tolerance float64) bool {
	in := r.Intersect(o)
	return in.Width > tolerance && in.Height > tolerance
}
