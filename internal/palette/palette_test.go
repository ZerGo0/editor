package palette

import (
	"slices"
	"testing"

	"github.com/ZerGo0/editor/internal/design/quickinput"
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

func TestPrefixPicksMode(t *testing.T) {
	var got string
	p := Palette{Modes: []Mode{
		{Label: "Files", Search: Static(items("main.go")), Accept: func(it Item) { got = "file " + it.ID }},
		{Prefix: ">", Label: "Commands", Search: Static(items("Save", "Close")), Accept: func(it Item) { got = "cmd " + it.ID }},
	}}
	p.Open(">clo")
	if p.Label() != "Commands" || len(p.items) != 1 {
		t.Fatalf("mode %q, items %v", p.Label(), p.items)
	}
	p.Accept()
	if got != "cmd Close" || p.IsOpen() {
		t.Fatalf("accepted %q, open %v", got, p.IsOpen())
	}
	p.Open("")
	if p.Label() != "Files" {
		t.Fatalf("empty query mode = %q", p.Label())
	}
}

func TestAcceptVariants(t *testing.T) {
	var got []string
	files := items("a.go", "b.go")
	p := Palette{Modes: []Mode{{
		Label:      "Files",
		Search:     func(_ string, done func([]Item)) { done(files) },
		Accept:     func(it Item) { got = append(got, "open "+it.ID) },
		AcceptSide: func(it Item) { got = append(got, "side "+it.ID) },
		Background: func(it Item) { got = append(got, "background "+it.ID) },
	}}}
	p.Open("")
	p.acceptWith(p.items[0], quickinput.Result{Background: true})
	if !p.IsOpen() {
		t.Fatal("opening in the background closed the palette")
	}
	p.acceptWith(p.items[1], quickinput.Result{Side: true})
	if p.IsOpen() || !slices.Equal(got, []string{"background a.go", "side b.go"}) {
		t.Fatalf("open %v, got %v", p.IsOpen(), got)
	}
}

func TestQueryItemKeepsPaletteOpen(t *testing.T) {
	p := Palette{Modes: []Mode{
		{Prefix: "?", Label: "Help", Search: Static([]Item{{Label: "Commands", Query: ">"}})},
		{Prefix: ">", Label: "Commands", Search: Static(items("Save"))},
	}}
	p.Open("?")
	p.Accept()
	if !p.IsOpen() || p.Label() != "Commands" || p.state.Query != ">" {
		t.Fatalf("open %v, mode %q, query %q", p.IsOpen(), p.Label(), p.state.Query)
	}
}

func TestLateResultsOfOldQueryAreDropped(t *testing.T) {
	var pending []func([]Item)
	p := Palette{Modes: []Mode{{Label: "Files", Search: func(_ string, done func([]Item)) { pending = append(pending, done) }}}}
	p.Open("a")
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
