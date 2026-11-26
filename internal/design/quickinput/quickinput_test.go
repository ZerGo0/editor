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

func TestOpenToSideAndInBackground(t *testing.T) {
	items := []Item{{Label: "a.go"}, {Label: "b.go"}}
	s := &State{}
	var last Result
	tt := ui.NewTester(func(c *ui.Context) {
		if r := View(c, "Go to File", s, items, Options{Side: true, Background: true}); r.Accepted >= 0 {
			last = r
		}
	}, testWidth, testHeight)
	tt.Key(ui.Cmd, ui.KeyEnter)
	if last.Accepted != 0 || !last.Side || last.Background {
		t.Fatalf("Cmd+Enter result %+v", last)
	}
	last = Result{Accepted: none}
	tt.Type("ab")
	tt.Key(0, ui.KeyLeft)
	tt.Key(0, ui.KeyRight)
	if last.Accepted != none {
		t.Fatalf("Right inside the query accepted %+v", last)
	}
	tt.Key(0, ui.KeyRight)
	if last.Accepted != 0 || !last.Background || last.Side {
		t.Fatalf("Right at the end result %+v", last)
	}
}

func TestRemoveButtonRemovesRow(t *testing.T) {
	items := []Item{{Label: "a.go", Remove: "Remove from Recently Opened"}}
	var last Result
	tt := ui.NewTester(func(c *ui.Context) {
		if r := View(c, "Go to File", &State{}, items, Options{}); r.Removed >= 0 || r.Accepted >= 0 {
			last = r
		}
	}, testWidth, testHeight)
	tt.Click("Remove from Recently Opened")
	if last.Removed != 0 || last.Accepted != none {
		t.Fatalf("remove click result %+v", last)
	}
}

func TestRowReadsKeys(t *testing.T) {
	items := []Item{{Label: "Go to File...", Keys: "⌘P"}}
	tt := ui.NewTester(func(c *ui.Context) { View(c, "Command Palette", &State{}, items, Options{}) }, testWidth, testHeight)
	for _, a := range tt.AccessAll() {
		if a.Name == "Go to File..." && a.Description == "⌘P" {
			return
		}
	}
	t.Fatal("no row reads its keys as its description")
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
