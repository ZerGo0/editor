package command

import (
	"slices"
	"strconv"
	"testing"

	"github.com/egoist/mygo"
)

func TestMarkUsedMovesCommandToFrontOfRecent(t *testing.T) {
	r := NewRegistry()
	noop := func(*mygo.Window) {}
	r.Add(&Command{ID: "a", Title: "A", Run: noop}, &Command{ID: "b", Title: "B", Run: noop})
	r.Run("b", nil)
	if len(r.Recent()) != 0 {
		t.Fatalf("Run outside the palette recorded %v", r.Recent())
	}
	r.MarkUsed("a")
	r.MarkUsed("b")
	r.MarkUsed("a")
	r.MarkUsed("missing")
	if got := r.Recent(); !slices.Equal(got, []string{"a", "b"}) {
		t.Fatalf("recent %v", got)
	}
}

func TestSetRecentKeepsAtMostMaxRecent(t *testing.T) {
	ids := make([]string, maxRecent+5)
	for i := range ids {
		ids[i] = strconv.Itoa(i)
	}
	r := NewRegistry()
	r.SetRecent(ids)
	got := r.Recent()
	if len(got) != maxRecent || got[0] != ids[0] {
		t.Fatalf("recent len %d first %q", len(got), got[0])
	}
	ids[0] = "changed"
	if r.Recent()[0] == "changed" {
		t.Fatal("SetRecent must copy its input")
	}
}
