package redline

import (
	"image"
	"image/color"
	"math"
	"sort"
	"strconv"
)

var (
	red     = color.RGBA{R: 230, G: 30, B: 40, A: 255}
	pink    = color.RGBA{R: 255, G: 105, B: 180, A: 255}
	cyan    = color.RGBA{R: 0, G: 200, B: 230, A: 255}
	yellow  = color.RGBA{R: 255, G: 210, B: 0, A: 255}
	white   = color.RGBA{R: 255, G: 255, B: 255, A: 255}
	inkLine = color.RGBA{R: 160, G: 60, B: 255, A: 255}
)

const (
	outlineAlpha = 0.55
	guideAlpha   = 0.45
	dimAlpha     = 0.9
	markAlpha    = 1
	guideMin     = 3
	guideMax     = 24
	tickHalf     = 2
	glyphW       = 3
	glyphH       = 5
	glyphSpace   = 1
	labelPad     = 1
	markWidth    = 2
)

var glyphs = map[rune][glyphH]uint8{
	'0': {7, 5, 5, 5, 7}, '1': {2, 6, 2, 2, 7}, '2': {7, 1, 7, 4, 7}, '3': {7, 1, 7, 1, 7},
	'4': {5, 5, 7, 1, 1}, '5': {7, 4, 7, 1, 7}, '6': {7, 4, 7, 5, 7}, '7': {7, 1, 1, 1, 1},
	'8': {7, 5, 7, 5, 7}, '9': {7, 5, 7, 1, 7}, '.': {0, 0, 0, 0, 2}, '-': {0, 0, 7, 0, 0},
	'+': {0, 2, 7, 2, 0}, '#': {5, 7, 5, 7, 5},
}

type canvas struct {
	img *image.RGBA
	k   int
}

func (c canvas) blend(x, y int, col color.RGBA, alpha float64) {
	if !(image.Point{X: x, Y: y}.In(c.img.Bounds())) {
		return
	}
	i := c.img.PixOffset(x, y)
	mix := func(dst, src uint8) uint8 {
		return uint8(math.Round(float64(dst)*(1-alpha) + float64(src)*alpha))
	}
	c.img.Pix[i] = mix(c.img.Pix[i], col.R)
	c.img.Pix[i+1] = mix(c.img.Pix[i+1], col.G)
	c.img.Pix[i+2] = mix(c.img.Pix[i+2], col.B)
	c.img.Pix[i+3] = channelMax
}

func (c canvas) hline(x0, x1, y int, col color.RGBA, alpha float64) {
	for x := min(x0, x1); x <= max(x0, x1); x++ {
		c.blend(x, y, col, alpha)
	}
}

func (c canvas) vline(x, y0, y1 int, col color.RGBA, alpha float64) {
	for y := min(y0, y1); y <= max(y0, y1); y++ {
		c.blend(x, y, col, alpha)
	}
}

func (c canvas) outline(r image.Rectangle, col color.RGBA, alpha float64, width int) {
	for w := 0; w < width; w++ {
		c.hline(r.Min.X-w, r.Max.X-1+w, r.Min.Y-w, col, alpha)
		c.hline(r.Min.X-w, r.Max.X-1+w, r.Max.Y-1+w, col, alpha)
		c.vline(r.Min.X-w, r.Min.Y-w+1, r.Max.Y-2+w, col, alpha)
		c.vline(r.Max.X-1+w, r.Min.Y-w+1, r.Max.Y-2+w, col, alpha)
	}
}

func (c canvas) label(x, y int, text string, fg, bg color.RGBA) {
	runes := []rune(text)
	w := (len(runes)*(glyphW+glyphSpace) - glyphSpace + 2*labelPad) * c.k
	h := (glyphH + 2*labelPad) * c.k
	x0, y0 := x-w/2, y-h/2
	for yy := y0; yy < y0+h; yy++ {
		c.hline(x0, x0+w-1, yy, bg, markAlpha)
	}
	for n, r := range runes {
		g, ok := glyphs[r]
		if !ok {
			continue
		}
		gx := x0 + (labelPad+n*(glyphW+glyphSpace))*c.k
		for row := 0; row < glyphH; row++ {
			for col := 0; col < glyphW; col++ {
				if g[row]&(1<<(glyphW-1-col)) == 0 {
					continue
				}
				for dy := 0; dy < c.k; dy++ {
					for dx := 0; dx < c.k; dx++ {
						c.blend(gx+col*c.k+dx, y0+(labelPad+row)*c.k+dy, fg, markAlpha)
					}
				}
			}
		}
	}
}

func (c canvas) dimension(from, to image.Point, value float64, col color.RGBA) {
	t := tickHalf * c.k
	if from.Y == to.Y {
		c.hline(from.X, to.X, from.Y, col, dimAlpha)
		c.vline(from.X, from.Y-t, from.Y+t, col, dimAlpha)
		c.vline(to.X, to.Y-t, to.Y+t, col, dimAlpha)
	} else {
		c.vline(from.X, from.Y, to.Y, col, dimAlpha)
		c.hline(from.X-t, from.X+t, from.Y, col, dimAlpha)
		c.hline(to.X-t, to.X+t, to.Y, col, dimAlpha)
	}
	c.label((from.X+to.X)/2, (from.Y+to.Y)/2, num(value), white, col)
}

func (a *analysis) annotate() (*image.RGBA, int) {
	out := toRGBA(a.shot)
	c := canvas{img: out, k: max(1, int(math.Round(a.scale)))}
	guides := a.guides(c)
	for _, b := range a.boxes {
		c.outline(a.px(b.Rect), red, outlineAlpha, 1)
	}
	for _, p := range a.pads {
		a.drawPadding(c, p)
	}
	for _, l := range a.links {
		if l.gap > eps && l.gap <= gapDrawMax {
			a.drawGap(c, l)
		}
	}
	for _, m := range a.measures {
		c.outline(a.px(m.Ink), inkLine, outlineAlpha, 1)
	}
	for n, f := range a.findings {
		r := a.px(f.Area)
		c.outline(r, yellow, markAlpha, markWidth)
		c.label(r.Min.X, r.Min.Y, "#"+strconv.Itoa(n+1), red, yellow)
	}
	return out, guides
}

func (a *analysis) drawGap(c canvas, l link) {
	ra, rb := a.boxes[l.a].Rect, a.boxes[l.b].Rect
	if l.across {
		y := (math.Max(ra.Y, rb.Y) + math.Min(ra.Bottom(), rb.Bottom())) / 2 * a.scale
		c.dimension(image.Pt(int(ra.Right()*a.scale), int(y)), image.Pt(int(rb.X*a.scale)-1, int(y)), l.gap, red)
		return
	}
	x := (math.Max(ra.X, rb.X) + math.Min(ra.Right(), rb.Right())) / 2 * a.scale
	c.dimension(image.Pt(int(x), int(ra.Bottom()*a.scale)), image.Pt(int(x), int(rb.Y*a.scale)-1), l.gap, red)
}

func (a *analysis) drawPadding(c canvas, p padding) {
	o, in := a.boxes[p.box].Rect, p.inner
	s := a.scale
	midX, midY := int(in.CenterX()*s), int(in.CenterY()*s)
	sides := []struct {
		v        float64
		from, to image.Point
	}{
		{in.X - o.X, image.Pt(int(o.X*s), midY), image.Pt(int(in.X*s)-1, midY)},
		{o.Right() - in.Right(), image.Pt(int(in.Right()*s), midY), image.Pt(int(o.Right()*s)-1, midY)},
		{in.Y - o.Y, image.Pt(midX, int(o.Y*s)), image.Pt(midX, int(in.Y*s)-1)},
		{o.Bottom() - in.Bottom(), image.Pt(midX, int(in.Bottom()*s)), image.Pt(midX, int(o.Bottom()*s)-1)},
	}
	for _, side := range sides {
		if side.v > eps && side.v <= padMax {
			c.dimension(side.from, side.to, side.v, pink)
		}
	}
}

type guide struct {
	vertical bool
	at, n    int
}

func (a *analysis) guides(c canvas) int {
	counts := map[[2]int]int{}
	seen := map[image.Rectangle]bool{}
	for _, b := range a.boxes {
		r := a.px(b.Rect)
		if seen[r] {
			continue
		}
		seen[r] = true
		for _, x := range []int{r.Min.X, r.Max.X, (r.Min.X + r.Max.X) / 2} {
			counts[[2]int{1, x}]++
		}
		for _, y := range []int{r.Min.Y, r.Max.Y, (r.Min.Y + r.Max.Y) / 2} {
			counts[[2]int{0, y}]++
		}
	}
	var gs []guide
	for k, n := range counts {
		if n >= guideMin {
			gs = append(gs, guide{vertical: k[0] == 1, at: k[1], n: n})
		}
	}
	sort.Slice(gs, func(i, j int) bool {
		if gs[i].n != gs[j].n {
			return gs[i].n > gs[j].n
		}
		if gs[i].vertical != gs[j].vertical {
			return gs[i].vertical
		}
		return gs[i].at < gs[j].at
	})
	drawn := map[bool]int{}
	total := 0
	b := c.img.Bounds()
	for _, g := range gs {
		if drawn[g.vertical] >= guideMax {
			continue
		}
		drawn[g.vertical]++
		total++
		if g.vertical {
			c.vline(g.at, b.Min.Y, b.Max.Y-1, cyan, guideAlpha)
		} else {
			c.hline(b.Min.X, b.Max.X-1, g.at, cyan, guideAlpha)
		}
	}
	return total
}
