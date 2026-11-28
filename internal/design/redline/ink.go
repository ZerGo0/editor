package redline

import (
	"image"
	"image/color"
	"math"
)

const (
	channelMax = 255
	inkMin     = 0.1
	solidCov   = 0.85
	softCov    = 0.2
	maskCov    = 0.5
	minScanPx  = 4
	rgbaStride = 4
)

type ink struct {
	r       image.Rectangle
	bg      color.RGBA
	cov     []float64
	diffMax float64
}

func pixel(img *image.RGBA, x, y int) color.RGBA {
	i := img.PixOffset(x, y)
	return color.RGBA{R: img.Pix[i], G: img.Pix[i+1], B: img.Pix[i+2], A: img.Pix[i+3]}
}

func chroma(c color.RGBA) float64 {
	return float64(max(c.R, c.G, c.B)-min(c.R, c.G, c.B)) / channelMax
}

func distance(a, b color.RGBA) float64 {
	d := math.Max(math.Abs(float64(a.R)-float64(b.R)), math.Abs(float64(a.G)-float64(b.G)))
	return math.Max(d, math.Abs(float64(a.B)-float64(b.B))) / channelMax
}

func packed(c color.RGBA) uint32 {
	return uint32(c.R)<<24 | uint32(c.G)<<16 | uint32(c.B)<<8 | uint32(c.A)
}

func modeColor(img *image.RGBA, r image.Rectangle) color.RGBA {
	counts := map[uint32]int{}
	var best uint32
	top := 0
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			k := packed(pixel(img, x, y))
			counts[k]++
			if n := counts[k]; n > top || (n == top && k < best) {
				best, top = k, n
			}
		}
	}
	return color.RGBA{R: uint8(best >> 24), G: uint8(best >> 16), B: uint8(best >> 8), A: uint8(best)}
}

func scan(img *image.RGBA, r image.Rectangle, holes []image.Rectangle) (ink, bool) {
	r = r.Intersect(img.Bounds())
	if r.Dx() < minScanPx || r.Dy() < minScanPx {
		return ink{}, false
	}
	w, h := r.Dx(), r.Dy()
	s := ink{r: r, bg: modeColor(img, r), cov: make([]float64, w*h)}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if d := distance(pixel(img, r.Min.X+x, r.Min.Y+y), s.bg); d >= inkMin {
				s.cov[y*w+x] = d
			}
		}
	}
	for _, h := range holes {
		h = h.Intersect(r)
		for y := h.Min.Y; y < h.Max.Y; y++ {
			for x := h.Min.X; x < h.Max.X; x++ {
				s.cov[(y-r.Min.Y)*w+x-r.Min.X] = 0
			}
		}
	}
	s.dropEdgeInk()
	for _, d := range s.cov {
		s.diffMax = math.Max(s.diffMax, d)
	}
	if s.diffMax == 0 {
		return ink{}, false
	}
	for i := range s.cov {
		s.cov[i] /= s.diffMax
	}
	return s, true
}

func (s *ink) dropEdgeInk() {
	w, h := s.r.Dx(), s.r.Dy()
	var queue []int
	push := func(x, y int) {
		if x < 0 || y < 0 || x >= w || y >= h {
			return
		}
		if i := y*w + x; s.cov[i] > 0 {
			s.cov[i] = 0
			queue = append(queue, i)
		}
	}
	for x := 0; x < w; x++ {
		push(x, 0)
		push(x, h-1)
	}
	for y := 0; y < h; y++ {
		push(0, y)
		push(w-1, y)
	}
	for len(queue) > 0 {
		i := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		x, y := i%w, i/w
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				push(x+dx, y+dy)
			}
		}
	}
}

func (s ink) at(x, y int) float64 {
	return s.cov[y*s.r.Dx()+x]
}

type span struct{ from, to int }

func (s ink) columns(rows span) []float64 {
	out := make([]float64, s.r.Dx())
	for y := rows.from; y < rows.to; y++ {
		for x := range out {
			out[x] = math.Max(out[x], s.at(x, y))
		}
	}
	return out
}

func (s ink) columnsSkipping(skip []bool) []float64 {
	out := make([]float64, s.r.Dx())
	for y, skipped := range skip {
		if skipped {
			continue
		}
		for x := range out {
			out[x] = math.Max(out[x], s.at(x, y))
		}
	}
	return out
}

func (s ink) strikeRows(cols span) []bool {
	out := make([]bool, s.r.Dy())
	width := cols.to - cols.from
	for y := range out {
		n := 0
		for x := cols.from; x < cols.to; x++ {
			if s.at(x, y) >= maskCov {
				n++
			}
		}
		out[y] = width > 0 && float64(n) >= strikeShare*float64(width)
	}
	return out
}

func (s ink) rows(cols span) []float64 {
	out := make([]float64, s.r.Dy())
	for y := range out {
		for x := cols.from; x < cols.to; x++ {
			out[y] = math.Max(out[y], s.at(x, y))
		}
	}
	return out
}

func (s ink) rowSums(cols span) []float64 {
	out := make([]float64, s.r.Dy())
	for y := range out {
		for x := cols.from; x < cols.to; x++ {
			out[y] += s.at(x, y)
		}
	}
	return out
}

func extent(profile []float64) (float64, float64, bool) {
	first, last := -1, -1
	for i, v := range profile {
		if v > 0 {
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	if first < 0 {
		return 0, 0, false
	}
	return float64(first) + 1 - profile[first], float64(last) + profile[last], true
}

func clusters(profile []float64, gap int) []span {
	var out []span
	start, blank := -1, 0
	for i, v := range profile {
		switch {
		case v > 0 && start < 0:
			start, blank = i, 0
		case v > 0:
			blank = 0
		case start >= 0:
			blank++
			if blank >= gap {
				out = append(out, span{start, i - blank + 1})
				start = -1
			}
		}
	}
	if start >= 0 {
		out = append(out, span{start, len(profile) - blank})
	}
	return out
}

type inkStats struct {
	area     float64
	mask     int
	boundary int
	solid    int
	soft     int
	contrast float64
	chroma   float64
}

func (s ink) stats(img *image.RGBA, cols, rows span) inkStats {
	var st inkStats
	w := s.r.Dx()
	for y := rows.from; y < rows.to; y++ {
		for x := cols.from; x < cols.to; x++ {
			c := s.at(x, y)
			if c <= 0 {
				continue
			}
			st.area += c
			switch {
			case c >= solidCov:
				st.solid++
			case c >= softCov:
				st.soft++
			}
			if c >= maskCov {
				px := pixel(img, s.r.Min.X+x, s.r.Min.Y+y)
				st.contrast = math.Max(st.contrast, contrast(px, s.bg))
				st.chroma = math.Max(st.chroma, chroma(px))
				st.mask++
				if s.edgeOf(x, y, w) {
					st.boundary++
				}
			}
		}
	}
	return st
}

func (s ink) edgeOf(x, y, w int) bool {
	h := s.r.Dy()
	for _, d := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
		nx, ny := x+d[0], y+d[1]
		if nx < 0 || ny < 0 || nx >= w || ny >= h || s.cov[ny*w+nx] < maskCov {
			return true
		}
	}
	return false
}
