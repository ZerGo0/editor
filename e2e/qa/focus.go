package qa

import (
	"fmt"
	"slices"
)

const (
	RuleFocusLost       = "focus-lost"
	RuleFocusStuck      = "focus-stuck"
	RuleFocusOrder      = "focus-order"
	RuleFocusUnlabeled  = "focus-unlabeled"
	RuleFocusOffscreen  = "focus-offscreen"
	RuleUnlabeled       = "unlabeled-interactive"
	RuleFocusNotReturn  = "focus-not-restored"
	focusRowTolerance   = 6.0
	focusColumnSlack    = 2.0
	maxConsecutiveStuck = 2
)

type FocusStop struct {
	Step  int
	Key   string
	Tree  *Tree
	Focus Node
	Found bool
}

func NewStop(step int, key string, t *Tree) FocusStop {
	n, ok := t.Focused()
	return FocusStop{Step: step, Key: key, Tree: t, Focus: n, Found: ok}
}

func (s FocusStop) Describe() string {
	if !s.Found {
		return "no focused element"
	}
	return fmt.Sprintf("%s %q %s", s.Focus.Role, s.Focus.Name(), describe(s.Focus.Bounds))
}

func VisualOrderOK(a, b Rect) bool {
	switch {
	case b.X >= a.Right()-focusColumnSlack:
		return true
	case abs(b.CenterY()-a.CenterY()) <= focusRowTolerance:
		return b.CenterX() >= a.CenterX()
	}
	return b.CenterY() > a.CenterY()
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func CheckFocusWalk(view string, stops []FocusStop) []Flag {
	var flags []Flag
	flag := func(s FocusStop, rule, measured, expected, fix string) {
		n := s.Focus
		window := Rect{}
		if s.Tree != nil {
			window = s.Tree.Window
		}
		flags = append(flags, Flag{Rule: rule, View: view, Owner: Owner(view, n, window), Role: n.Role, Label: n.Name(), Bounds: n.Bounds, Measured: measured, Expected: expected, Fix: fix})
	}
	stuck := 0
	for i, s := range stops {
		if !s.Found {
			flags = append(flags, Flag{Rule: RuleFocusLost, View: view, Owner: Owner(view, Node{}, Rect{}), Measured: fmt.Sprintf("after %s (step %d) the tree has no focused element", s.Key, s.Step), Expected: "an element is focused after every step", Fix: "route focus to the part that opens (see workspace focus.go takeFocus) or keep it on the previous element"})
			stuck = 0
			continue
		}
		if s.Focus.Name() == "" {
			flag(s, RuleFocusUnlabeled, fmt.Sprintf("%s focused with no label, value or placeholder", s.Focus.Role), "every focusable element has an accessibility label", "add Label(...)")
		}
		if s.Tree != nil && !s.Tree.Window.Empty() && !s.Focus.Has("offscreen") && !s.Tree.Window.Contains(s.Focus.Bounds, layoutTolerance) && s.Focus.Bounds.Area() > 0 {
			flag(s, RuleFocusOffscreen, "focus is outside the window "+describe(s.Tree.Window), "focused element is visible", "scroll the focused element into view when it takes focus")
		}
		if i == 0 {
			continue
		}
		prev := stops[i-1]
		if !prev.Found {
			continue
		}
		if prev.Focus.ID == s.Focus.ID {
			stuck++
			if stuck >= maxConsecutiveStuck {
				flag(s, RuleFocusStuck, fmt.Sprintf("focus stayed on the same element for %d presses of %s", stuck+1, s.Key), "each press moves focus", "make the element release Tab or give the part another stop")
			}
			continue
		}
		stuck = 0
		if windowWrap(prev.Focus.Bounds, s.Focus.Bounds, s.Tree) {
			continue
		}
		if s.Key == "Tab" && !VisualOrderOK(prev.Focus.Bounds, s.Focus.Bounds) && !isWrap(stops, i) {
			flag(s, RuleFocusOrder, fmt.Sprintf("moved from %s to %s: up or left within the same column", prev.Describe(), s.Describe()), "top-to-bottom, then left-to-right", "reorder the focusable elements in the view to follow their visual order")
		}
		if s.Key == "Shift+Tab" && !VisualOrderOK(s.Focus.Bounds, prev.Focus.Bounds) && !isWrap(stops, i) {
			flag(s, RuleFocusOrder, fmt.Sprintf("moved back from %s to %s: not the reverse of the visual order", prev.Describe(), s.Describe()), "bottom-to-top, then right-to-left", "reorder the focusable elements in the view to follow their visual order")
		}
	}
	return flags
}

func windowWrap(from, to Rect, t *Tree) bool {
	if t == nil || t.Window.Empty() {
		return false
	}
	atBottom := func(r Rect) bool { return r.Y >= t.Window.Height-statusBarBand }
	atTop := func(r Rect) bool { return r.Bottom() <= titleBarBand }
	return atBottom(from) && atTop(to) || atTop(from) && atBottom(to)
}

func isWrap(stops []FocusStop, i int) bool {
	id := stops[i].Focus.ID
	return slices.IndexFunc(stops[:i], func(s FocusStop) bool { return s.Found && s.Focus.ID == id }) >= 0
}

func CheckInteractiveLabels(view string, t *Tree) []Flag {
	var flags []Flag
	for _, n := range t.Nodes {
		if !ControlRole(n.Role) || n.Name() != "" || n.Has("offscreen") || n.Bounds.Empty() {
			continue
		}
		flags = append(flags, Flag{Rule: RuleUnlabeled, View: view, Owner: Owner(view, n, t.Window), Role: n.Role, Bounds: n.Bounds,
			Measured: "empty label, value and placeholder", Expected: "a non-empty accessibility label", Fix: "add Label(...) (icon buttons use their tooltip text)"})
	}
	return flags
}

func CheckRestored(view, what string, before, after FocusStop) []Flag {
	switch {
	case !after.Found:
		return []Flag{{Rule: RuleFocusLost, View: view, Owner: Owner(view, Node{}, Rect{}), Measured: what + ": no focused element afterwards", Expected: "focus returns to the element that had it", Fix: "restore focus to the previously focused element on close"}}
	case before.Found && before.Focus.ID != after.Focus.ID && before.Focus.Role != after.Focus.Role:
		return []Flag{{Rule: RuleFocusNotReturn, View: view, Owner: Owner(view, after.Focus, Rect{}), Role: after.Focus.Role, Label: after.Focus.Name(), Bounds: after.Focus.Bounds,
			Measured: fmt.Sprintf("%s: focus went from %s to %s", what, before.Describe(), after.Describe()), Expected: "focus returns to " + before.Describe(), Fix: "restore focus to the previously focused element on close"}}
	}
	return nil
}
