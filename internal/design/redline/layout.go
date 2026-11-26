package redline

import (
	"math"
	"sort"
)

const (
	overlapShare = 0.5
	gapDrawMax   = 48
	unevenMax    = 4
	nearMissMax  = 3
	gridStep     = 4
	padMax       = 32
	alignReach   = 24
	borderPx     = 1
)

var itemRoles = map[string]bool{
	"listItem": true, "treeItem": true, "row": true, "cell": true, "tab": true, "button": true, "toggleButton": true,
	"menuButton": true, "checkBox": true, "switch": true, "radio": true, "menuItem": true, "option": true,
	"popUpButton": true, "link": true, "heading": true, "text": true, "textField": true, "comboBox": true,
}

type link struct {
	a, b   int
	gap    float64
	across bool
}

type padding struct {
	box   int
	inner Rect
}

func (a *analysis) devicePx() float64 {
	return 1 / a.scale
}

func sortedKeys(m map[int][]int) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}

func (a *analysis) layout() {
	seen := map[[2]int]bool{}
	for _, p := range sortedKeys(a.children) {
		kids := a.children[p]
		a.neighbours(kids)
		a.padding(p, kids)
		a.align(kids, seen)
	}
	for _, g := range sortedKeys(a.children) {
		byRole := map[string][]int{}
		var roles []string
		for _, p := range a.children[g] {
			for _, k := range a.children[p] {
				role := a.boxes[k].Role
				if _, ok := byRole[role]; !ok {
					roles = append(roles, role)
				}
				byRole[role] = append(byRole[role], k)
			}
		}
		sort.Strings(roles)
		for _, role := range roles {
			a.align(byRole[role], seen)
		}
	}
	a.uneven()
}

func (a *analysis) neighbours(kids []int) {
	for _, i := range kids {
		ri := a.boxes[i].Rect
		right, below := noBox, noBox
		rightGap, belowGap := math.Inf(1), math.Inf(1)
		for _, j := range kids {
			rj := a.boxes[j].Rect
			if j == i || ri.contains(rj) || rj.contains(ri) {
				continue
			}
			if g := rj.X - ri.Right(); g >= -eps && g < rightGap && overlap(ri.Y, ri.Bottom(), rj.Y, rj.Bottom()) >= overlapShare*math.Min(ri.H, rj.H) {
				right, rightGap = j, g
			}
			if g := rj.Y - ri.Bottom(); g >= -eps && g < belowGap && overlap(ri.X, ri.Right(), rj.X, rj.Right()) >= overlapShare*math.Min(ri.W, rj.W) {
				below, belowGap = j, g
			}
		}
		if right != noBox {
			a.links = append(a.links, link{a: i, b: right, gap: math.Max(rightGap, 0), across: true})
		}
		if below != noBox {
			a.links = append(a.links, link{a: i, b: below, gap: math.Max(belowGap, 0)})
		}
	}
}

func (a *analysis) uneven() {
	for _, across := range []bool{true, false} {
		next, incoming := map[int]int{}, map[int]bool{}
		var starts []int
		for li, l := range a.links {
			if l.across == across {
				next[l.a] = li
				incoming[l.b] = true
				starts = append(starts, l.a)
			}
		}
		for _, s := range starts {
			if incoming[s] {
				continue
			}
			var chain []link
			for at, ok := next[s]; ok && len(chain) <= len(a.links); at, ok = next[a.links[at].b] {
				l := a.links[at]
				if a.boxes[l.a].Role != a.boxes[l.b].Role {
					a.unevenChain(chain, across)
					chain = nil
					continue
				}
				chain = append(chain, l)
			}
			a.unevenChain(chain, across)
		}
	}
}

func (a *analysis) unevenChain(chain []link, across bool) {
	const minChain = 2
	if len(chain) < minChain {
		return
	}
	gaps := make([]float64, len(chain))
	for i, l := range chain {
		gaps[i] = l.gap
	}
	sort.Float64s(gaps)
	median := gaps[(len(gaps)-1)/2]
	check := "gap below"
	if across {
		check = "gap to next"
	}
	for _, l := range chain {
		d := math.Abs(l.gap - median)
		if d < a.devicePx()-eps || d > unevenMax {
			continue
		}
		a.add(Finding{Rule: RuleGap, Box: l.a, Other: l.b, Check: check, Measured: l.gap, Expected: median,
			Unit: UnitPx, Fix: "use one gap for the whole group: " + spaceTokens(median),
			Area: a.boxes[l.a].Rect.union(a.boxes[l.b].Rect)})
	}
}

func (a *analysis) padding(p int, kids []int) {
	if p == noBox || len(kids) == 0 || itemRoles[a.boxes[p].Role] {
		return
	}
	outer := a.boxes[p].Rect
	var inner Rect
	found := false
	for _, k := range kids {
		if textRoles[a.boxes[k].Role] || a.boxes[k].Role == headingRole || a.boxes[k].Role == fieldRole {
			continue
		}
		if !found {
			inner, found = a.boxes[k].Rect, true
		}
		inner = inner.union(a.boxes[k].Rect)
	}
	if !found || !outer.contains(inner) {
		return
	}
	a.pads = append(a.pads, padding{box: p, inner: inner})
	sides := []struct {
		check      string
		v, partner float64
	}{
		{"padding left", inner.X - outer.X, outer.Right() - inner.Right()},
		{"padding right", outer.Right() - inner.Right(), inner.X - outer.X},
		{"padding top", inner.Y - outer.Y, outer.Bottom() - inner.Bottom()},
		{"padding bottom", outer.Bottom() - inner.Bottom(), inner.Y - outer.Y},
	}
	for _, s := range sides {
		if s.v <= eps || s.v > padMax || math.Abs(s.v-s.partner) < a.devicePx() {
			continue
		}
		if onGrid(s.v) || onGrid(s.v-borderPx) {
			continue
		}
		want := math.Max(gridStep, math.Round(s.v/gridStep)*gridStep)
		a.add(Finding{Rule: RulePadding, Box: p, Other: noBox, Check: s.check, Measured: s.v, Expected: want,
			Unit: UnitPx, Fix: "put the content on the 4 px grid: " + spaceTokens(want), Area: outer})
	}
}

func onGrid(v float64) bool {
	off := math.Mod(v, gridStep)
	return off < eps || gridStep-off < eps
}

type edge struct {
	name string
	of   func(Rect) float64
}

var (
	columnEdges = []edge{{"left edge", Rect.leftEdge}, {"right edge", Rect.Right}, {"centre x", Rect.CenterX}}
	rowEdges    = []edge{{"top edge", Rect.topEdge}, {"bottom edge", Rect.Bottom}, {"centre y", Rect.CenterY}}
)

func (r Rect) leftEdge() float64 { return r.X }
func (r Rect) topEdge() float64  { return r.Y }

func (a *analysis) align(group []int, seen map[[2]int]bool) {
	for x, i := range group {
		for _, j := range group[x+1:] {
			key := [2]int{min(i, j), max(i, j)}
			if seen[key] {
				continue
			}
			seen[key] = true
			a.alignPair(key[0], key[1])
		}
	}
}

func (a *analysis) alignPair(i, j int) {
	if a.parents[i] == noBox || a.parents[j] == noBox || textRoles[a.boxes[i].Role] || textRoles[a.boxes[j].Role] ||
		a.boxes[i].Role == fieldRole || a.boxes[j].Role == fieldRole ||
		(a.boxes[i].Role != a.boxes[j].Role && a.parents[i] != a.parents[j]) {
		return
	}
	ri, rj := a.boxes[i].Rect, a.boxes[j].Rect
	ox := overlap(ri.X, ri.Right(), rj.X, rj.Right())
	oy := overlap(ri.Y, ri.Bottom(), rj.Y, rj.Bottom())
	var edges []edge
	var fix string
	switch {
	case ox > 0 && oy <= 0 && -oy <= alignReach:
		edges, fix = columnEdges, "give both the same inset or indent token (token.PadRow, token.PadView, token.IndentStep)"
	case oy > 0 && ox <= 0 && -ox <= alignReach:
		edges, fix = rowEdges, "centre both on the row (AlignItems center) or give them the same height token"
	default:
		return
	}
	best, bestEdge := math.Inf(1), edges[0]
	for _, e := range edges {
		d := e.of(rj) - e.of(ri)
		if math.Abs(d) < eps {
			return
		}
		if math.Abs(d) < math.Abs(best) {
			best, bestEdge = d, e
		}
	}
	if math.Abs(best) < a.devicePx()-eps || math.Abs(best) > nearMissMax {
		return
	}
	a.add(Finding{Rule: RuleAlign, Box: i, Other: j, Check: bestEdge.name, Measured: best, Expected: 0,
		Unit: UnitPx, Fix: fix, Area: ri.union(rj)})
}
