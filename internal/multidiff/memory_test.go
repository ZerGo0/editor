package multidiff

import (
	"fmt"
	"testing"
)

func TestMemoryKeepsTheNewestMarks(t *testing.T) {
	m := NewMemory()
	entry := func(i int) *Entry { return &Entry{Path: fmt.Sprintf("f%d.txt", i), Fingerprint: "x"} }
	for i := range MaxViewedMarks + 5 {
		m.SetViewed("set", entry(i), true)
	}
	if len(m.Marks()) != MaxViewedMarks || m.IsViewed("set", entry(0)) || !m.IsViewed("set", entry(MaxViewedMarks+4)) {
		t.Fatalf("marks = %d, oldest kept %v", len(m.Marks()), m.IsViewed("set", entry(0)))
	}
	m.SetViewed("set", entry(MaxViewedMarks+4), false)
	restored := NewMemory(m.Marks()...)
	if restored.IsViewed("set", entry(MaxViewedMarks+4)) || !restored.IsViewed("set", entry(MaxViewedMarks)) || len(restored.Marks()) != MaxViewedMarks-1 {
		t.Errorf("restored marks = %d", len(restored.Marks()))
	}
	if restored.IsViewed("set", &Entry{Path: "f10.txt", Fingerprint: "changed"}) {
		t.Error("a changed file is still marked")
	}
}

func TestViewLayoutAndTreeAreSettable(t *testing.T) {
	v := New()
	v.SetLayout(1)
	v.SetTree(300, false)
	if w, shown := v.Tree(); v.Layout() != 1 || w != 300 || shown || v.treeShown() {
		t.Errorf("layout %d, tree %v %v", v.Layout(), w, shown)
	}
	v.SetTree(0, true)
	if w, shown := v.Tree(); w != 300 || !shown {
		t.Errorf("a zero width replaced the tree width: %v %v", w, shown)
	}
}
