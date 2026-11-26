package redline

import (
	"image"
	"math"
	"strconv"
)

const (
	eps        = 0.01
	numDigits  = 100
	floatBits  = 64
	formatAuto = -1
)

func pxRect(r Rect, scale float64) image.Rectangle {
	return image.Rect(int(math.Round(r.X*scale)), int(math.Round(r.Y*scale)),
		int(math.Round(r.Right()*scale)), int(math.Round(r.Bottom()*scale)))
}

func finite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

func (r Rect) valid() bool {
	return finite(r.X) && finite(r.Y) && finite(r.W) && finite(r.H) && r.W > 0 && r.H > 0
}

func (r Rect) contains(o Rect) bool {
	return o.X >= r.X-eps && o.Y >= r.Y-eps && o.Right() <= r.Right()+eps && o.Bottom() <= r.Bottom()+eps
}

func (a *analysis) clipped(i int) bool {
	for p := a.parents[i]; p != noBox; p = a.parents[p] {
		if !a.boxes[p].Rect.contains(a.boxes[i].Rect) {
			return true
		}
	}
	return false
}

func (r Rect) same(o Rect) bool {
	return math.Abs(r.X-o.X) < eps && math.Abs(r.Y-o.Y) < eps && math.Abs(r.W-o.W) < eps && math.Abs(r.H-o.H) < eps
}

func (r Rect) union(o Rect) Rect {
	x0, y0 := math.Min(r.X, o.X), math.Min(r.Y, o.Y)
	x1, y1 := math.Max(r.Right(), o.Right()), math.Max(r.Bottom(), o.Bottom())
	return Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
}

func (r Rect) grow(d float64) Rect {
	return Rect{X: r.X - d, Y: r.Y - d, W: r.W + 2*d, H: r.H + 2*d}
}

func overlap(a0, a1, b0, b1 float64) float64 {
	return math.Min(a1, b1) - math.Max(a0, b0)
}

func visible(in []Box, bounds image.Rectangle, scale float64) ([]Box, []int) {
	byID := map[string]int{}
	tree := false
	for i, b := range in {
		if b.ID != "" {
			if _, dup := byID[b.ID]; !dup {
				byID[b.ID] = i
			}
		}
		tree = tree || b.Parent != ""
	}
	index := make([]int, len(in))
	var boxes []Box
	for i, b := range in {
		index[i] = noBox
		if b.Rect.valid() && !pxRect(b.Rect, scale).Intersect(bounds).Empty() {
			index[i] = len(boxes)
			boxes = append(boxes, b)
		}
	}
	parents := make([]int, len(boxes))
	for i, b := range in {
		at := index[i]
		if at == noBox {
			continue
		}
		parents[at] = noBox
		if !tree {
			parents[at] = container(boxes, at)
			continue
		}
		p, ok := byID[b.Parent]
		for steps := 0; ok && steps < len(in); steps++ {
			if index[p] != noBox && p != i {
				parents[at] = index[p]
				break
			}
			p, ok = byID[in[p].Parent]
		}
	}
	return boxes, parents
}

func container(boxes []Box, i int) int {
	best, area := noBox, math.Inf(1)
	r := boxes[i].Rect
	for j, b := range boxes {
		if j == i || !b.Rect.contains(r) || (b.Rect.same(r) && j > i) {
			continue
		}
		if a := b.Rect.W * b.Rect.H; a < area || (a == area && j > best) {
			best, area = j, a
		}
	}
	return best
}

func childrenOf(parents []int) map[int][]int {
	out := map[int][]int{}
	for i, p := range parents {
		out[p] = append(out[p], i)
	}
	return out
}

func num(v float64) string {
	v = math.Round(v*numDigits) / numDigits
	if v == 0 {
		v = 0
	}
	return strconv.FormatFloat(v, 'f', formatAuto, floatBits)
}

func signed(v float64) string {
	if math.Round(v*numDigits) > 0 {
		return "+" + num(v)
	}
	return num(v)
}
