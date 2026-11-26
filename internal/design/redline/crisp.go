package redline

import (
	"fmt"
	"image"
	"image/color"
	"math"
)

const (
	crispMin     = 0.06
	flatMax      = 0.02
	crispSamples = 6
	crispMid     = 3
	sampleQuart  = 4
)

func (a *analysis) crisp() {
	seen := map[string]bool{}
	for i, b := range a.boxes {
		r := a.px(b.Rect)
		if skipOptical[b.Role] || textRoles[b.Role] || b.Role == headingRole || a.clipped(i) || r.Dx() < minScanPx || r.Dy() < minScanPx {
			continue
		}
		edges := []struct {
			check    string
			pos      float64
			vertical bool
			from, to int
		}{
			{"left edge blur", b.Rect.X * a.scale, true, r.Min.Y, r.Max.Y},
			{"right edge blur", b.Rect.Right() * a.scale, true, r.Min.Y, r.Max.Y},
			{"top edge blur", b.Rect.Y * a.scale, false, r.Min.X, r.Max.X},
			{"bottom edge blur", b.Rect.Bottom() * a.scale, false, r.Min.X, r.Max.X},
		}
		for _, e := range edges {
			if math.Abs(e.pos-math.Round(e.pos)) < eps {
				continue
			}
			at := int(math.Round(e.pos))
			key := fmt.Sprint(e.vertical, at, e.from, e.to)
			if seen[key] {
				continue
			}
			seen[key] = true
			quart := (e.to - e.from) / sampleQuart
			if !a.smeared(at, e.from+quart, e.to-quart, e.vertical) {
				continue
			}
			a.add(Finding{Rule: RuleCrisp, Box: i, Other: noBox, Check: e.check, Measured: e.pos, Expected: math.Round(e.pos),
				Unit: UnitDevicePx, Fix: "snap the edge to whole device pixels: use whole-px token sizes and avoid fractional widths from Grow or text",
				Area: b.Rect})
		}
	}
}

func (a *analysis) smeared(at, from, to int, vertical bool) bool {
	if to <= from {
		return false
	}
	var s [crispSamples]color.RGBA
	for k := range s {
		off := at - crispMid + k
		var r image.Rectangle
		if vertical {
			r = image.Rect(off, from, off+1, to)
		} else {
			r = image.Rect(from, off, to, off+1)
		}
		if !r.In(a.shot.Bounds()) {
			return false
		}
		s[k] = modeColor(a.shot, r)
	}
	for _, m := range []int{crispMid - 1, crispMid} {
		if !between(s[m-1], s[m], s[m+1]) || distance(s[m-1], s[m]) < crispMin || distance(s[m], s[m+1]) < crispMin {
			continue
		}
		if distance(s[m-2], s[m-1]) <= flatMax || distance(s[m+1], s[m+2]) <= flatMax {
			return true
		}
	}
	return false
}

func between(lo, mid, hi color.RGBA) bool {
	in := func(a, m, b uint8) bool {
		return m >= min(a, b) && m <= max(a, b)
	}
	return in(lo.R, mid.R, hi.R) && in(lo.G, mid.G, hi.G) && in(lo.B, mid.B, hi.B)
}
