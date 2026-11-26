package quickinput

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

const (
	testWidth  = 800
	testHeight = 400
)

func TestKeyboard(t *testing.T) {
	items := []Item{{Label: "a.go"}, {Label: "b.go"}, {Label: "c.go"}}
	s := &State{}
	var last Result
	tt := ui.NewTester(func(c *ui.Context) {
		if r := View(c, "Go to File", s, items, Options{}); r.Accepted >= 0 || r.Closed {
			last = r
		}
	}, testWidth, testHeight)
	if _, ok := tt.AccessByRole(ui.RoleDialog, "Go to File"); !ok {
		t.Fatal("quick input has no dialog role")
	}
	tt.Key(0, ui.KeyDown)
	tt.Key(0, ui.KeyEnter)
	if last.Accepted != 1 {
		t.Fatalf("accepted %d, want 1", last.Accepted)
	}
	tt.Key(0, ui.KeyEscape)
	if !last.Closed {
		t.Fatal("Escape does not close the quick input")
	}
}

func TestEmptyList(t *testing.T) {
	s := &State{Selected: 4}
	tt := ui.NewTester(func(c *ui.Context) { View(c, "Go to File", s, nil, Options{}) }, testWidth, testHeight)
	if _, ok := tt.Find(defaultEmpty); !ok {
		t.Fatal("empty quick input shows no message")
	}
	if s.Selected != none {
		t.Fatalf("selected = %d on an empty list", s.Selected)
	}
}
