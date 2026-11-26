package list

import (
	"testing"

	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const (
	testWidth  = 400
	testHeight = 200
)

func rowView(state *ui.ListState) func(c *ui.Context) {
	names := []string{"a.go", "b.go"}
	return func(c *ui.Context) {
		View(c, state, len(names), ViewOptions{Label: "Changes"}, func(i int) Item {
			return Item{Label: names[i], Icon: icons.File, Actions: func(c *ui.Context) {
				action.IconButton(c, icons.Plus, "Stage "+names[i], action.IconOptions{})
			}}
		})
	}
}

func TestRowActionsShowOnKeyboardFocus(t *testing.T) {
	selected := -1
	state := &ui.ListState{Selected: &selected}
	tt := ui.NewTester(rowView(state), testWidth, testHeight)
	if _, ok := tt.Access("Stage a.go"); ok {
		t.Fatal("row actions show before hover or focus")
	}
	tt.Key(0, ui.KeyTab)
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	if selected < 0 {
		t.Fatal("arrow keys do not choose a row")
	}
	if _, ok := tt.Access("Stage " + []string{"a.go", "b.go"}[selected]); !ok {
		t.Fatal("row actions do not show on the focused row")
	}
}

func TestSpansMarkMatches(t *testing.T) {
	spans := Spans("gallery", []int{0, 1}, ui.Transparent, ui.Transparent)
	if len(spans) != 2 || spans[0].Text != "ga" || spans[1].Text != "llery" {
		t.Fatalf("spans = %+v", spans)
	}
	if spans := Spans("go", []int{5}, ui.Transparent, ui.Transparent); len(spans) != 1 {
		t.Fatalf("out-of-range match changed the spans: %+v", spans)
	}
}
