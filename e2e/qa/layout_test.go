package qa

import (
	"strings"
	"testing"
)

const (
	testWindowWidth  = 800.0
	testWindowHeight = 600.0
	testRowHeight    = 22.0
)

func testTree(nodes ...Node) *Tree {
	return &Tree{Nodes: nodes, Window: Rect{Width: testWindowWidth, Height: testWindowHeight}}
}

func rules(flags []Flag) []string {
	var out []string
	for _, f := range flags {
		out = append(out, f.Rule)
	}
	return out
}

func hasRule(flags []Flag, rule string) bool {
	for _, f := range flags {
		if f.Rule == rule {
			return true
		}
	}
	return false
}

func TestLayoutAcceptsCleanTree(t *testing.T) {
	tree := testTree(
		Node{ID: "1", Role: "group", Bounds: Rect{0, 0, 300, 100}},
		Node{ID: "2", Parent: "1", Role: "button", Label: "Save", Bounds: Rect{10, 10, 60, testRowHeight}},
		Node{ID: "3", Parent: "1", Role: "button", Label: "Cancel", Bounds: Rect{80, 10, 60, testRowHeight}},
	)
	if flags := CheckLayout(tree, LayoutOptions{View: "explorer"}); len(flags) != 0 {
		t.Fatalf("clean tree flagged: %v", flags)
	}
}

func TestLayoutFlagsOutsideWindow(t *testing.T) {
	tree := testTree(Node{ID: "1", Role: "button", Label: "Far", Bounds: Rect{testWindowWidth - 10, 10, 60, testRowHeight}})
	flags := CheckLayout(tree, LayoutOptions{View: "explorer"})
	if !hasRule(flags, RuleOutsideWindow) {
		t.Fatalf("rules = %v", rules(flags))
	}
	if flags[0].Owner != OwnerWorkspace && flags[0].Owner == "" {
		t.Fatalf("owner empty: %+v", flags[0])
	}
}

func TestLayoutAllowsScrolledRowsPartiallyHidden(t *testing.T) {
	tree := testTree(
		Node{ID: "1", Role: "list", Bounds: Rect{0, 40, 300, 100}},
		Node{ID: "2", Parent: "1", Role: "listItem", Label: "half", Bounds: Rect{0, 120, 300, testRowHeight}},
	)
	if flags := CheckLayout(tree, LayoutOptions{View: "search-text"}); hasRule(flags, RuleOutsideParent) || hasRule(flags, RuleOutsideWindow) {
		t.Fatalf("scroll clip flagged: %v", flags)
	}
}

func TestLayoutFlagsChildLeavingParent(t *testing.T) {
	tree := testTree(
		Node{ID: "1", Role: "group", Bounds: Rect{0, 0, 100, 30}},
		Node{ID: "2", Parent: "1", Role: "text", Label: "x", Bounds: Rect{90, 5, 60, 16}},
	)
	if flags := CheckLayout(tree, LayoutOptions{View: "scm"}); !hasRule(flags, RuleOutsideParent) {
		t.Fatalf("rules = %v", rules(CheckLayout(tree, LayoutOptions{View: "scm"})))
	}
}

func TestLayoutFlagsPartialOverlapOnly(t *testing.T) {
	overlapping := testTree(
		Node{ID: "1", Role: "text", Label: "a", Bounds: Rect{0, 0, 100, 16}},
		Node{ID: "2", Role: "button", Label: "b", Bounds: Rect{80, 0, 60, testRowHeight}},
	)
	if flags := CheckLayout(overlapping, LayoutOptions{View: "scm"}); !hasRule(flags, RuleOverlap) {
		t.Fatalf("overlap not found: %v", flags)
	}
	nested := testTree(
		Node{ID: "1", Role: "button", Label: "a", Bounds: Rect{0, 0, 100, testRowHeight}},
		Node{ID: "2", Role: "text", Label: "b", Bounds: Rect{4, 2, 60, 16}},
	)
	if flags := CheckLayout(nested, LayoutOptions{View: "scm"}); hasRule(flags, RuleOverlap) {
		t.Fatalf("nested flagged: %v", flags)
	}
}

func TestLayoutIgnoresOverlayAgainstContent(t *testing.T) {
	tree := testTree(
		Node{ID: "1", Role: "text", Label: "under", Bounds: Rect{0, 0, 100, 16}},
		Node{ID: "2", Role: "dialog", Label: "Dialog", Bounds: Rect{50, 0, 300, 200}},
		Node{ID: "3", Parent: "2", Role: "text", Label: "Hello", Bounds: Rect{60, 8, 100, 16}},
	)
	if flags := CheckLayout(tree, LayoutOptions{View: "quickinput"}); hasRule(flags, RuleOverlap) {
		t.Fatalf("overlay flagged: %v", flags)
	}
}

func TestLayoutFlagsSmallHitTarget(t *testing.T) {
	tree := testTree(Node{ID: "1", Role: "button", Label: "Tiny", Bounds: Rect{0, 0, 14, 14}})
	flags := CheckLayout(tree, LayoutOptions{View: "tabs"})
	if !hasRule(flags, RuleHitTarget) {
		t.Fatalf("rules = %v", rules(flags))
	}
	if !strings.Contains(flags[0].Expected, "HitTargetMin") {
		t.Fatalf("expected text: %q", flags[0].Expected)
	}
}

func fixedWidth(per float64) TextMeasurer {
	return func(texts []string, size float64) []float64 {
		out := make([]float64, len(texts))
		for i, s := range texts {
			out[i] = float64(len([]rune(s))) * per
		}
		return out
	}
}

func TestLayoutFlagsCutText(t *testing.T) {
	tree := testTree(
		Node{ID: "1", Role: "text", Label: "a very long label that cannot fit", Bounds: Rect{0, 0, 80, 16}},
		Node{ID: "2", Role: "text", Label: "short", Bounds: Rect{0, 40, 80, 16}},
	)
	flags := CheckLayout(tree, LayoutOptions{View: "explorer", Measurer: fixedWidth(6)})
	if len(flags) != 1 || flags[0].Rule != RuleTextCut || flags[0].Label != "a very long label that cannot fit" {
		t.Fatalf("flags = %v", flags)
	}
}

func TestLayoutSkipsWrappedText(t *testing.T) {
	tree := testTree(Node{ID: "1", Role: "text", Label: "wrapped paragraph that is long", Bounds: Rect{0, 0, 80, 64}})
	if flags := CheckLayout(tree, LayoutOptions{View: "scm", Measurer: fixedWidth(6)}); hasRule(flags, RuleTextCut) {
		t.Fatalf("wrapped text flagged: %v", flags)
	}
}

func TestAllowlistFiltersAndReportsStale(t *testing.T) {
	flags := []Flag{{Rule: RuleOverlap, View: "scm", Role: "text", Label: "Branch name"}, {Rule: RuleTextCut, View: "scm", Label: "x"}}
	allow := []Allow{{Rule: RuleOverlap, View: "scm", LabelPrefix: "Branch", Reason: "known"}, {Rule: RuleHitTarget, Reason: "gone"}}
	kept, allowed := Filter(flags, allow)
	if len(kept) != 1 || len(allowed) != 1 || kept[0].Rule != RuleTextCut {
		t.Fatalf("kept %v allowed %v", kept, allowed)
	}
	stale := Stale(flags, allow)
	if len(stale) != 1 || stale[0].Rule != RuleHitTarget {
		t.Fatalf("stale = %v", stale)
	}
}

func TestFormatGroupsByOwner(t *testing.T) {
	out := Format([]Flag{{Rule: RuleOverlap, Owner: OwnerGit, Label: "a"}, {Rule: RuleOverlap, Owner: OwnerSearch, Label: "b"}, {Rule: RuleTextCut, Owner: OwnerGit, Label: "c"}})
	if !strings.Contains(out, OwnerGit+" (2)") || !strings.Contains(out, OwnerSearch+" (1)") {
		t.Fatalf("report:\n%s", out)
	}
}

func TestOwnerSeparatesChromeFromView(t *testing.T) {
	window := Rect{Width: testWindowWidth, Height: testWindowHeight}
	status := Node{Role: "text", Bounds: Rect{10, window.Height - 20, 80, 16}}
	if got := Owner("scm", status, window); got != OwnerWorkspace {
		t.Fatalf("status bar owner = %s", got)
	}
	row := Node{Role: "text", Bounds: Rect{80, 200, 80, 16}}
	if got := Owner("scm", row, window); got != OwnerGit {
		t.Fatalf("view owner = %s", got)
	}
}
