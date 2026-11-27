package command

import (
	"bytes"
	"log"
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo"
)

func TestSequenceIDRoundTrips(t *testing.T) {
	ids := []string{"a", "b.c", "d"}
	got, ok := ParseSequenceID(SequenceID(ids))
	if !ok || !slices.Equal(got, ids) {
		t.Fatalf("ParseSequenceID = %v, %v, want %v", got, ok, ids)
	}
	for _, id := range []string{"a", "runCommands", "runCommands:", "runCommands:{}"} {
		if _, ok := ParseSequenceID(id); ok {
			t.Errorf("ParseSequenceID(%q) accepted a plain ID", id)
		}
	}
}

func TestSequenceRunsInOrderAndSkipsUnknownOnce(t *testing.T) {
	var logged bytes.Buffer
	log.SetOutput(&logged)
	defer log.SetOutput(log.Writer())
	r := NewRegistry()
	var ran []string
	record := func(id string) func(*mygo.Window) { return func(*mygo.Window) { ran = append(ran, id) } }
	r.Add(&Command{ID: "first", Run: record("first")}, &Command{ID: "second", Run: record("second")})
	id := SequenceID([]string{"first", "missing", "second", "missing"})
	if !r.Run(id, nil) || !r.Run(id, nil) {
		t.Fatal("Run did not accept a sequence ID")
	}
	if want := []string{"first", "second", "first", "second"}; !slices.Equal(ran, want) {
		t.Errorf("ran %v, want %v", ran, want)
	}
	if n := strings.Count(logged.String(), "missing"); n != 1 {
		t.Errorf("logged the unknown command %d times, want once:\n%s", n, logged.String())
	}
}

func TestSequenceNestsAndHasATitle(t *testing.T) {
	r := NewRegistry()
	var ran []string
	r.Add(&Command{ID: "a", Run: func(*mygo.Window) { ran = append(ran, "a") }})
	inner := SequenceID([]string{"a", "a"})
	if !r.Run(SequenceID([]string{inner, "a"}), nil) || len(ran) != 3 {
		t.Errorf("nested sequence ran %v", ran)
	}
	cmd := r.Get(SequenceID([]string{"a", "b"}))
	if cmd == nil || cmd.Title != "Run a, b" {
		t.Errorf("Get = %+v, want the title %q", cmd, "Run a, b")
	}
	if r.Get("missing") != nil || r.Run("missing", nil) {
		t.Error("an unknown plain ID must stay unknown")
	}
}
