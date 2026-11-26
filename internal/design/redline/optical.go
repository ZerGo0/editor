package redline

import (
	"image"
	"image/color"
	"math"
	"sort"
	"strings"
)

const (
	smallMax     = 40
	textRowMax   = 40
	opticalTol   = 0.5
	tintMin      = 0.25
	headingRole  = "heading"
	fieldRole    = "textField"
	clusterGap   = 5
	iconLikeMax  = 20
	iconInkMin   = 6
	contrastText = 4.5
	contrastIcon = 3
	glyphGap     = 1
	ringPx       = 2
	tallShare    = 0.5
	hugShare     = 0.6
	capGlyphs    = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789bdfhklt"
)

var (
	skipOptical = map[string]bool{"progress": true}
	textRoles   = map[string]bool{"text": true, "staticText": true, "label": true}
)

func (a *analysis) optical() {
	for i, b := range a.boxes {
		if skipOptical[b.Role] || a.clipped(i) {
			continue
		}
		switch r := b.Rect; {
		case r.W <= smallMax && r.H <= smallMax:
			a.iconBox(i)
		case r.H <= textRowMax:
			a.textBox(i)
		}
	}
}

func (a *analysis) scanBox(i int) (ink, bool) {
	var holes []image.Rectangle
	for _, k := range a.children[i] {
		holes = append(holes, a.px(a.boxes[k].Rect))
	}
	return scan(a.shot, a.px(a.boxes[i].Rect), holes)
}

func (a *analysis) centred(i int, bg color.RGBA) bool {
	if !textRoles[a.boxes[i].Role] {
		return true
	}
	r := a.px(a.boxes[i].Rect)
	outer := r.Inset(-ringPx).Intersect(a.shot.Bounds())
	ring := []image.Rectangle{
		image.Rect(outer.Min.X, outer.Min.Y, outer.Max.X, r.Min.Y),
		image.Rect(outer.Min.X, r.Max.Y, outer.Max.X, outer.Max.Y),
	}
	for _, part := range ring {
		if !part.Empty() && distance(modeColor(a.shot, part), bg) >= inkMin {
			return true
		}
	}
	return false
}

func (a *analysis) hugs(m Measure) bool {
	return m.Text && m.Ink.H >= hugShare*a.boxes[m.Box].Rect.H
}

func (a *analysis) iconBox(i int) {
	b := a.boxes[i]
	s, ok := a.scanBox(i)
	if !ok {
		return
	}
	all := span{0, s.r.Dx()}
	m, ok := a.measureIcon(i, s, all, MeasureIcon)
	if !ok {
		return
	}
	m.HasX = true
	m.Text = textRoles[b.Role] || len(clusters(s.columns(span{0, s.r.Dy()}), glyphGap)) > 1
	if m.Text {
		if t, ok := a.measureText(i, s, all); ok {
			m.Ink.Y, m.Ink.H, m.DY = t.Ink.Y, t.Ink.H, t.DY
		}
	}
	a.measures = append(a.measures, m)
	need := float64(contrastIcon)
	if m.Text {
		need = contrastText
	}
	if a.centred(i, s.bg) && !a.hugs(m) {
		a.flagCentre(m, "ink centre x", m.DX, "move the glyph by "+signed(-m.DX)+" px in whole px: change the box size or padding so the difference is even")
		a.flagCentre(m, "ink centre y", m.DY, "move the glyph by "+signed(-m.DY)+" px in whole px: change the box size or padding so the difference is even")
	}
	a.flagContrast(m, need)
}

func (a *analysis) measureIcon(i int, s ink, cols span, kind MeasureKind) (Measure, bool) {
	r := a.boxes[i].Rect
	x0, x1, okX := extent(s.columns(span{0, s.r.Dy()})[cols.from:cols.to])
	y0, y1, okY := extent(s.rows(cols))
	if !okX || !okY {
		return Measure{}, false
	}
	x0, x1 = x0+float64(cols.from+s.r.Min.X), x1+float64(cols.from+s.r.Min.X)
	y0, y1 = y0+float64(s.r.Min.Y), y1+float64(s.r.Min.Y)
	st := s.stats(a.shot, cols, span{0, s.r.Dy()})
	m := Measure{Box: i, Kind: kind,
		Ink:      Rect{X: x0 / a.scale, Y: y0 / a.scale, W: (x1 - x0) / a.scale, H: (y1 - y0) / a.scale},
		DX:       ((x0+x1)/2 - r.CenterX()*a.scale) / a.scale,
		DY:       ((y0+y1)/2 - r.CenterY()*a.scale) / a.scale,
		Contrast: st.contrast,
		Tinted:   st.chroma > tintMin,
	}
	if st.boundary > 0 {
		m.Stroke = 2 * float64(st.mask) / float64(st.boundary) / a.scale
	}
	if n := st.solid + st.soft; n > 0 {
		m.Soft = float64(st.soft) / float64(n)
	}
	return m, true
}

func (a *analysis) textBox(i int) {
	s, ok := a.scanBox(i)
	if !ok {
		return
	}
	parts := clusters(s.columns(span{0, s.r.Dy()}), int(math.Round(clusterGap*a.scale)))
	if len(parts) == 0 {
		return
	}
	text := 0
	for k, p := range parts {
		if p.to-p.from > parts[text].to-parts[text].from {
			text = k
		}
	}
	iconPx := iconLikeMax * a.scale
	if lead := parts[0]; text != 0 && !textRoles[a.boxes[i].Role] && float64(lead.to-lead.from) <= iconPx {
		if m, ok := a.measureIcon(i, s, lead, MeasureRowIcon); ok && m.Ink.H <= iconLikeMax && math.Max(m.Ink.W, m.Ink.H) >= iconInkMin {
			a.measures = append(a.measures, m)
			a.flagCentre(m, "row icon centre y", m.DY, "move the icon by "+signed(-m.DY)+" px in whole px: change the box size or padding so the difference is even")
			a.flagContrast(m, contrastIcon)
		}
	}
	if float64(parts[text].to-parts[text].from) <= iconPx {
		return
	}
	m, ok := a.measureText(i, s, parts[text])
	if !ok {
		return
	}
	a.measures = append(a.measures, m)
	if a.centred(i, s.bg) && !a.hugs(m) {
		a.flagCentre(m, "cap band centre y", m.DY, "move the text by "+signed(-m.DY)+" px in whole px: use token.TextStyle.ApplyLine or an even box size")
	}
	a.flagContrast(m, contrastText)
}

func (a *analysis) measureText(i int, s ink, cols span) (Measure, bool) {
	if !strings.ContainsAny(a.boxes[i].Label, capGlyphs) {
		return Measure{}, false
	}
	h := s.r.Dy()
	var tops, bottoms []int
	for _, g := range clusters(s.columns(span{0, h})[cols.from:cols.to], glyphGap) {
		top, bottom := -1, -1
		for y := 0; y < h; y++ {
			for x := cols.from + g.from; x < cols.from+g.to; x++ {
				if s.at(x, y) >= maskCov {
					if top < 0 {
						top = y
					}
					bottom = y + 1
					break
				}
			}
		}
		if top >= 0 {
			tops, bottoms = append(tops, top), append(bottoms, bottom)
		}
	}
	if len(tops) == 0 {
		return Measure{}, false
	}
	base, xTop, minTop := mode(bottoms), mode(tops), tops[0]
	for _, t := range tops {
		minTop = min(minTop, t)
	}
	var tall []float64
	for _, t := range tops {
		if float64(t) <= float64(minTop)+tallShare*float64(xTop-minTop) {
			tall = append(tall, float64(t))
		}
	}
	r := a.boxes[i].Rect
	y0, y1 := median(tall)+float64(s.r.Min.Y), float64(base+s.r.Min.Y)
	x0 := float64(cols.from + s.r.Min.X)
	st := s.stats(a.shot, cols, span{0, h})
	return Measure{Box: i, Kind: MeasureText, Text: true,
		Ink:      Rect{X: x0 / a.scale, Y: y0 / a.scale, W: float64(cols.to-cols.from) / a.scale, H: (y1 - y0) / a.scale},
		DY:       ((y0+y1)/2 - r.CenterY()*a.scale) / a.scale,
		Contrast: st.contrast,
	}, true
}

func mode(vs []int) int {
	counts := map[int]int{}
	best, top := vs[0], 0
	for _, v := range vs {
		counts[v]++
		if n := counts[v]; n > top || (n == top && v > best) {
			best, top = v, n
		}
	}
	return best
}

func (a *analysis) flagCentre(m Measure, check string, d float64, fix string) {
	tol := math.Max(opticalTol, a.devicePx())
	if math.Abs(d) <= tol {
		return
	}
	rule := RuleOptical
	if m.Kind == MeasureText {
		rule = RuleText
	}
	a.add(Finding{Rule: rule, Box: m.Box, Other: noBox, Check: check, Measured: d, Expected: 0, Tolerance: tol,
		Unit: UnitPx, Fix: fix, Area: a.boxes[m.Box].Rect})
}

func (a *analysis) flagContrast(m Measure, need float64) {
	if a.boxes[m.Box].Disabled || m.Contrast == 0 || m.Contrast >= need {
		return
	}
	what := "text"
	if !m.Text {
		what = "icon"
	}
	a.add(Finding{Rule: RuleContrast, Box: m.Box, Other: noBox, Check: what + " contrast", Measured: m.Contrast, Expected: need,
		Unit: UnitRatio, Fix: "use a stronger token.Of(c) text or icon role for this background", Area: a.boxes[m.Box].Rect})
}

func luminance(c color.RGBA) float64 {
	const (
		rWeight, gWeight, bWeight = 0.2126, 0.7152, 0.0722
		linearLimit, linearDiv    = 0.04045, 12.92
		gammaOffset, gammaDiv     = 0.055, 1.055
		gamma                     = 2.4
	)
	lin := func(v uint8) float64 {
		f := float64(v) / channelMax
		if f <= linearLimit {
			return f / linearDiv
		}
		return math.Pow((f+gammaOffset)/gammaDiv, gamma)
	}
	return rWeight*lin(c.R) + gWeight*lin(c.G) + bWeight*lin(c.B)
}

func contrast(a, b color.RGBA) float64 {
	const flare = 0.05
	la, lb := luminance(a), luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + flare) / (lb + flare)
}

func median(vs []float64) float64 {
	s := append([]float64(nil), vs...)
	sort.Float64s(s)
	return s[(len(s)-1)/2]
}
