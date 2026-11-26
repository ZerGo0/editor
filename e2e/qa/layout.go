package qa

import (
	"fmt"
	"slices"
	"strings"
)

const (
	RuleOutsideWindow = "outside-window"
	RuleOutsideParent = "outside-parent"
	RuleOverlap       = "overlap"
	RuleTextCut       = "text-cut"
	RuleHitTarget     = "hit-target"

	layoutTolerance   = 1.0
	overlapTolerance  = 2.0
	hitTargetMin      = 22.0
	hitTargetSlack    = 0.5
	textSlack         = 1.0
	textCutRatio      = 0.6
	lineHeightRatio   = 1.25
	textLowerBound    = 11.0
	wrappedLineFactor = 1.6
	maxOverlapNodes   = 1500
)

var clipRoles = map[string]bool{"scroll": true, "list": true, "tree": true, "table": true, "grid": true, "treeGrid": true, "tabList": true, "log": true}

var overlayRoles = map[string]bool{"popup": true, "dialog": true, "alertDialog": true, "menu": true, "tooltip": true, "alert": true}

var textRoles = map[string]bool{"text": true, "heading": true, "status": true}

var hitTargetRoles = map[string]bool{"button": true, "toggleButton": true, "menuButton": true, "popUpButton": true, "checkBox": true, "switch": true, "link": true, "disclosure": true}

var overlapRoles = map[string]bool{"text": true, "heading": true, "button": true, "link": true, "checkBox": true, "switch": true, "textField": true, "tab": true, "toggleButton": true, "menuButton": true, "popUpButton": true, "image": true, "slider": true, "status": true}

type TextMeasurer func(texts []string, size float64) []float64

type LayoutOptions struct {
	View     string
	Measurer TextMeasurer
	Allow    []Allow
}

func CheckLayout(t *Tree, o LayoutOptions) []Flag {
	t.Index()
	var flags []Flag
	add := func(n Node, rule, measured, expected, fix string) {
		flags = append(flags, Flag{Rule: rule, View: o.View, Owner: Owner(o.View, n, t.Window), Role: n.Role, Label: n.Name(), Bounds: n.Bounds, Measured: measured, Expected: expected, Fix: fix})
	}
	flagged := map[string]bool{}
	for _, n := range t.Nodes {
		if n.Has("offscreen") || n.Bounds.Width == 0 && n.Bounds.Height == 0 {
			continue
		}
		if slices.ContainsFunc(t.Ancestors(n), func(a Node) bool { return flagged[a.ID] }) {
			continue
		}
		before := len(flags)
		checkWindow(t, n, add)
		checkParent(t, n, add)
		flagged[n.ID] = len(flags) > before
		if hitTargetRoles[n.Role] && !n.Has("disabled") && !insideOverlayRoot(t, n) {
			if n.Bounds.Width < hitTargetMin-hitTargetSlack || n.Bounds.Height < hitTargetMin-hitTargetSlack {
				add(n, RuleHitTarget, fmt.Sprintf("%.0fx%.0f", n.Bounds.Width, n.Bounds.Height), fmt.Sprintf("at least %.0fx%.0f (token.HitTargetMin)", hitTargetMin, hitTargetMin), "use the design component that pads to token.HitTargetMin")
			}
		}
	}
	flags = append(flags, checkOverlaps(t, o)...)
	flags = append(flags, checkText(t, o)...)
	return flags
}

func clipOf(t *Tree, n Node) (Rect, bool) {
	var clip Rect
	found := false
	for _, a := range t.Ancestors(n) {
		if clipRoles[a.Role] {
			if !found {
				clip, found = a.Bounds, true
			} else {
				clip = clip.Intersect(a.Bounds)
			}
		}
	}
	return clip, found
}

func insideOverlayRoot(t *Tree, n Node) bool {
	if overlayRoles[n.Role] {
		return true
	}
	for _, a := range t.Ancestors(n) {
		if overlayRoles[a.Role] {
			return true
		}
	}
	return false
}

func checkWindow(t *Tree, n Node, add func(Node, string, string, string, string)) {
	if t.Window.Empty() {
		return
	}
	if clip, ok := clipOf(t, n); ok && t.Window.Contains(clip, layoutTolerance) {
		return
	}
	if !t.Window.Contains(n.Bounds, layoutTolerance) {
		add(n, RuleOutsideWindow, fmt.Sprintf("%s beyond window %s", describe(n.Bounds), describe(t.Window)), "inside the window", "constrain the width (MinWidth(0), Shrink, Fill) or clip the content")
	}
}

func checkParent(t *Tree, n Node, add func(Node, string, string, string, string)) {
	if overlayRoles[n.Role] || n.Parent == "" {
		return
	}
	p, ok := t.ByID(n.Parent)
	if !ok || clipRoles[p.Role] || overlayRoles[p.Role] || p.Bounds.Empty() {
		return
	}
	if _, clipped := clipOf(t, n); clipped {
		return
	}
	if !p.Bounds.Contains(n.Bounds, layoutTolerance) {
		add(n, RuleOutsideParent, fmt.Sprintf("%s outside parent %s %q %s", describe(n.Bounds), p.Role, p.Name(), describe(p.Bounds)), "inside the parent's bounds", "let the parent grow or shrink the child (MinWidth(0), Shrink, Wrap)")
	}
}

func describe(r Rect) string {
	return fmt.Sprintf("(%.0f,%.0f %.0fx%.0f)", r.X, r.Y, r.Width, r.Height)
}

func checkOverlaps(t *Tree, o LayoutOptions) []Flag {
	var cands []Node
	for _, n := range t.Nodes {
		if !overlapRoles[n.Role] || n.Bounds.Empty() || n.Has("offscreen") {
			continue
		}
		if clip, ok := clipOf(t, n); ok {
			visible := n.Bounds.Intersect(clip)
			if visible.Empty() {
				continue
			}
			n.Bounds = visible
		}
		cands = append(cands, n)
	}
	if len(cands) > maxOverlapNodes {
		cands = cands[:maxOverlapNodes]
	}
	slices.SortFunc(cands, func(a, b Node) int {
		if a.Bounds.X != b.Bounds.X {
			return cmpFloat(a.Bounds.X, b.Bounds.X)
		}
		return cmpFloat(a.Bounds.Y, b.Bounds.Y)
	})
	var flags []Flag
	for i, a := range cands {
		for _, b := range cands[i+1:] {
			if b.Bounds.X >= a.Bounds.Right() {
				break
			}
			if !a.Bounds.Overlaps(b.Bounds, overlapTolerance) || related(t, a, b) || a.Bounds == b.Bounds {
				continue
			}
			if a.Bounds.Contains(b.Bounds, layoutTolerance) || b.Bounds.Contains(a.Bounds, layoutTolerance) {
				continue
			}
			if insideOverlayRoot(t, a) != insideOverlayRoot(t, b) {
				continue
			}
			in := a.Bounds.Intersect(b.Bounds)
			flags = append(flags, Flag{
				Rule: RuleOverlap, View: o.View, Owner: Owner(o.View, a, t.Window), Role: a.Role, Label: a.Name(), Bounds: a.Bounds,
				Measured: fmt.Sprintf("overlaps %s %q %s by %.0fx%.0f", b.Role, b.Name(), describe(b.Bounds), in.Width, in.Height),
				Expected: "siblings do not overlap", Fix: "give the siblings a Gap and let the flexible one shrink (MinWidth(0))",
			})
		}
	}
	return flags
}

func cmpFloat(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

func related(t *Tree, a, b Node) bool {
	for _, p := range t.Ancestors(a) {
		if p.ID == b.ID {
			return true
		}
	}
	for _, p := range t.Ancestors(b) {
		if p.ID == a.ID {
			return true
		}
	}
	return false
}

func checkText(t *Tree, o LayoutOptions) []Flag {
	if o.Measurer == nil {
		return nil
	}
	wrappedHeight := textLowerBound * lineHeightRatio * wrappedLineFactor
	var cands []Node
	for _, n := range t.Nodes {
		if n.Label == "" || n.Bounds.Empty() || n.Has("offscreen") || strings.Contains(n.Label, "\n") || !textRoles[n.Role] || n.Bounds.Height > wrappedHeight {
			continue
		}
		cands = append(cands, n)
	}
	texts := make([]string, len(cands))
	for i, n := range cands {
		texts[i] = n.Label
	}
	widths := o.Measurer(texts, textLowerBound)
	if len(widths) != len(cands) {
		return nil
	}
	var flags []Flag
	for i, n := range cands {
		if widths[i]*textCutRatio <= n.Bounds.Width+textSlack {
			continue
		}
		flags = append(flags, Flag{
			Rule: RuleTextCut, View: o.View, Owner: Owner(o.View, n, t.Window), Role: n.Role, Label: n.Label, Bounds: n.Bounds,
			Measured: fmt.Sprintf("text needs at least %.0fpx at %.0fpt, box is %.0fpx", widths[i], textLowerBound, n.Bounds.Width),
			Expected: fmt.Sprintf("box at least %.0f%% of the text width (a light ellipsis is allowed, a crushed label is not)", textCutRatio*100), Fix: "give the label room (Grow(1), MinWidth(0) on its siblings) or shorten it with an ellipsis and a tooltip",
		})
	}
	return flags
}
