package redline

import "math"

const (
	iconSetMin    = 3
	iconSizeRatio = 1.5
	strokeTol     = 0.5
)

type iconGroup struct {
	parent int
	role   string
	w, h   float64
	row    bool
}

func (a *analysis) iconSets() {
	groups := map[iconGroup][]int{}
	var keys []iconGroup
	for mi, m := range a.measures {
		if m.Text || m.Tinted {
			continue
		}
		b := a.boxes[m.Box]
		key := iconGroup{parent: a.parents[m.Box], role: b.Role, w: math.Round(b.Rect.W), h: math.Round(b.Rect.H)}
		if m.Kind == MeasureRowIcon {
			key = iconGroup{parent: key.parent, role: b.Role, h: key.h, row: true}
		}
		if _, ok := groups[key]; !ok {
			keys = append(keys, key)
		}
		groups[key] = append(groups[key], mi)
	}
	for _, k := range keys {
		if len(groups[k]) >= iconSetMin {
			a.iconSet(groups[k])
		}
	}
}

func (a *analysis) iconSet(ms []int) {
	sizes, strokes := make([]float64, len(ms)), make([]float64, len(ms))
	for i, mi := range ms {
		m := a.measures[mi]
		sizes[i], strokes[i] = math.Max(m.Ink.W, m.Ink.H), m.Stroke
	}
	size, stroke := median(sizes), median(strokes)
	for i, mi := range ms {
		m := a.measures[mi]
		area := a.boxes[m.Box].Rect
		if sizes[i] > size*iconSizeRatio || sizes[i]*iconSizeRatio < size {
			a.add(Finding{Rule: RuleIconSet, Box: m.Box, Other: noBox, Check: "optical size", Measured: sizes[i], Expected: size,
				Tolerance: size * (iconSizeRatio - 1), Unit: UnitPx, Fix: "use the same icon size token as its neighbours (token.IconSize or token.IconSizeSmall)", Area: area})
		}
		if stroke > 0 && math.Abs(strokes[i]-stroke) > strokeTol {
			a.add(Finding{Rule: RuleIconSet, Box: m.Box, Other: noBox, Check: "stroke weight", Measured: strokes[i], Expected: stroke,
				Tolerance: strokeTol, Unit: UnitPx, Fix: "use an icon from the same set and size as its neighbours", Area: area})
		}
	}
}
