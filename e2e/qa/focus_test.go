package qa

import "testing"

func stopAt(step int, key, id string, b Rect, window Rect) FocusStop {
	tree := &Tree{Focus: id, Window: window, Nodes: []Node{{ID: id, Role: "button", Label: "n" + id, Bounds: b}}}
	return NewStop(step, key, tree)
}

var focusWindow = Rect{Width: testWindowWidth, Height: testWindowHeight}

func TestVisualOrder(t *testing.T) {
	cases := []struct {
		name string
		a, b Rect
		ok   bool
	}{
		{"down", Rect{0, 0, 100, 22}, Rect{0, 30, 100, 22}, true},
		{"right same row", Rect{0, 0, 40, 22}, Rect{50, 0, 40, 22}, true},
		{"next column above", Rect{0, 300, 100, 22}, Rect{120, 40, 100, 22}, true},
		{"up", Rect{0, 100, 100, 22}, Rect{0, 40, 100, 22}, false},
		{"left same row", Rect{100, 0, 40, 22}, Rect{0, 0, 40, 22}, false},
	}
	for _, c := range cases {
		if got := VisualOrderOK(c.a, c.b); got != c.ok {
			t.Errorf("%s: got %v want %v", c.name, got, c.ok)
		}
	}
}

func TestFocusWalkFlagsBackwardJump(t *testing.T) {
	stops := []FocusStop{
		stopAt(0, "Tab", "1", Rect{0, 100, 100, 22}, focusWindow),
		stopAt(1, "Tab", "2", Rect{0, 40, 100, 22}, focusWindow),
	}
	flags := CheckFocusWalk("scm", stops)
	if !hasRule(flags, RuleFocusOrder) {
		t.Fatalf("rules = %v", rules(flags))
	}
}

func TestFocusWalkAllowsWrapAround(t *testing.T) {
	stops := []FocusStop{
		stopAt(0, "Tab", "1", Rect{0, 10, 100, 22}, focusWindow),
		stopAt(1, "Tab", "2", Rect{0, 100, 100, 22}, focusWindow),
		stopAt(2, "Tab", "1", Rect{0, 10, 100, 22}, focusWindow),
	}
	if flags := CheckFocusWalk("scm", stops); len(flags) != 0 {
		t.Fatalf("wrap flagged: %v", flags)
	}
}

func TestFocusWalkFlagsLostAndStuck(t *testing.T) {
	lost := NewStop(1, "Tab", &Tree{Window: focusWindow})
	stops := []FocusStop{
		stopAt(0, "Tab", "1", Rect{0, 10, 100, 22}, focusWindow),
		lost,
		stopAt(2, "Tab", "3", Rect{0, 50, 100, 22}, focusWindow),
		stopAt(3, "Tab", "3", Rect{0, 50, 100, 22}, focusWindow),
		stopAt(4, "Tab", "3", Rect{0, 50, 100, 22}, focusWindow),
	}
	flags := CheckFocusWalk("scm", stops)
	if !hasRule(flags, RuleFocusLost) || !hasRule(flags, RuleFocusStuck) {
		t.Fatalf("rules = %v", rules(flags))
	}
}

func TestFocusWalkFlagsUnlabeledAndOffscreen(t *testing.T) {
	tree := &Tree{Focus: "1", Window: focusWindow, Nodes: []Node{{ID: "1", Role: "button", Bounds: Rect{testWindowWidth, 0, 20, 22}}}}
	flags := CheckFocusWalk("scm", []FocusStop{NewStop(0, "Tab", tree)})
	if !hasRule(flags, RuleFocusUnlabeled) || !hasRule(flags, RuleFocusOffscreen) {
		t.Fatalf("rules = %v", rules(flags))
	}
}

func TestInteractiveLabels(t *testing.T) {
	tree := testTree(
		Node{ID: "1", Role: "button", Bounds: Rect{0, 0, 22, 22}},
		Node{ID: "2", Role: "button", Placeholder: "Search", Bounds: Rect{30, 0, 22, 22}},
		Node{ID: "3", Role: "group", Bounds: Rect{60, 0, 22, 22}},
	)
	flags := CheckInteractiveLabels("tabs", tree)
	if len(flags) != 1 || flags[0].Bounds.X != 0 {
		t.Fatalf("flags = %v", flags)
	}
}

func TestRestoredFocus(t *testing.T) {
	before := stopAt(0, "Cmd+P", "1", Rect{0, 0, 100, 22}, focusWindow)
	lost := NewStop(1, "Escape", &Tree{Window: focusWindow})
	if flags := CheckRestored("quickinput", "Escape", before, lost); !hasRule(flags, RuleFocusLost) {
		t.Fatalf("flags = %v", flags)
	}
	if flags := CheckRestored("quickinput", "Escape", before, before); len(flags) != 0 {
		t.Fatalf("flags = %v", flags)
	}
}

func TestFocusWalkAllowsWindowWrap(t *testing.T) {
	stops := []FocusStop{
		stopAt(0, "Tab", "1", Rect{700, testWindowHeight - 22, 60, 22}, focusWindow),
		stopAt(1, "Tab", "2", Rect{360, 6, 22, 22}, focusWindow),
	}
	if flags := CheckFocusWalk("scm", stops); len(flags) != 0 {
		t.Fatalf("window wrap flagged: %v", flags)
	}
}
