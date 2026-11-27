package palette

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

const (
	testViewW = 800
	testViewH = 600
)

func items(labels ...string) []Item {
	out := make([]Item, len(labels))
	for i, l := range labels {
		out[i] = Item{ID: l, Label: l}
	}
	return out
}

func TestLateResultsOfOldQueryAreDropped(t *testing.T) {
	var pending []func([]Item)
	var p Palette
	p.Show(Mode{Label: "Files", Search: func(_ string, done func([]Item)) { pending = append(pending, done) }})
	p.state.Query = "ab"
	p.refresh()
	pending[0](items("old"))
	pending[1](items("new"))
	if len(p.items) != 1 || p.items[0].Label != "new" {
		t.Fatalf("items = %v", p.items)
	}
}

func TestShowListMovesAndRenders(t *testing.T) {
	var accepted string
	var p Palette
	p.Show(Mode{Label: "Open Editors", Search: Static(items("one", "two")), Accept: func(it Item) { accepted = it.ID }})
	tt := ui.NewTester(p.View, testViewW, testViewH)
	if !tt.HasText("one") || !tt.HasText("two") {
		t.Fatal("the items are not shown")
	}
	p.Move(1)
	p.Accept()
	if accepted != "two" || p.IsOpen() {
		t.Fatalf("accepted %q, open %v", accepted, p.IsOpen())
	}
}

func TestFilterMarksMatches(t *testing.T) {
	got := Filter(items("Save File", "Close Editor"), "sf")
	if len(got) != 1 || got[0].Label != "Save File" || len(got[0].Positions) != 2 {
		t.Fatalf("filtered %+v", got)
	}
}
