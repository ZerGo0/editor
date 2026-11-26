package navhistory

import (
	"slices"
	"testing"

	"github.com/ZerGo0/editor/internal/session"
)

func loc(path string, line int) session.Location {
	return session.Location{Path: path, Line: line}
}

func TestBackAndForward(t *testing.T) {
	var h History
	h.Record(loc("a", 1))
	h.Record(loc("b", 2))
	got, ok := h.Back(loc("c", 3), 1)
	if !ok || got != loc("b", 2) {
		t.Fatalf("back = %v, %v", got, ok)
	}
	got, ok = h.Forward(loc("b", 2), 1)
	if !ok || got != loc("c", 3) {
		t.Fatalf("forward = %v, %v", got, ok)
	}
	if h.CanForward() || !h.CanBack() {
		t.Fatal("forward stack is not empty after going forward to the newest location")
	}
}

func TestBackSeveralSteps(t *testing.T) {
	var h History
	for i := range 3 {
		h.Record(loc("f", i))
	}
	got, ok := h.Back(loc("f", 9), 2)
	if !ok || got != loc("f", 1) {
		t.Fatalf("back 2 = %v, %v", got, ok)
	}
	want := []session.Location{loc("f", 2), loc("f", 9)}
	if fwd := h.ForwardList(); !slices.Equal(fwd, want) {
		t.Fatalf("forward list = %v, want %v", fwd, want)
	}
	if _, ok := h.Back(loc("f", 1), 5); ok {
		t.Fatal("went back more steps than recorded")
	}
}

func TestRecordClearsForwardAndSkipsDuplicates(t *testing.T) {
	var h History
	h.Record(loc("a", 1))
	h.Back(loc("b", 1), 1)
	h.Record(loc("a", 1))
	h.Record(loc("a", 1))
	if h.CanForward() {
		t.Fatal("a new jump keeps the forward history")
	}
	if n := len(h.BackList()); n != 1 {
		t.Fatalf("back entries = %d, want 1", n)
	}
}

func TestLimit(t *testing.T) {
	var h History
	for i := range Limit + 5 {
		h.Record(loc("f", i))
	}
	back := h.BackList()
	if len(back) != Limit || back[0] != loc("f", Limit+4) {
		t.Fatalf("back = %d entries, newest %v", len(back), back[0])
	}
}

func TestSnapshotRestoreAndMap(t *testing.T) {
	var h History
	h.Record(loc("a", 1))
	h.Record(loc("b", 2))
	back, fwd := h.Snapshot()
	var r History
	r.Restore(back, fwd)
	r.Map(func(l session.Location) (session.Location, bool) { return l, l.Path != "a" })
	if got := r.BackList(); !slices.Equal(got, []session.Location{loc("b", 2)}) {
		t.Fatalf("restored and mapped = %v", got)
	}
}
