package filetree

import "testing"

func TestHistoryRedoClearsOnNewChange(t *testing.T) {
	tree := New(t.TempDir(), nil)
	first, second := &Change{Label: "first"}, &Change{Label: "second"}
	tree.Record(first)
	if ch := tree.PopUndo(); ch != first {
		t.Fatalf("PopUndo = %+v", ch)
	}
	if ch := tree.PopUndo(); ch != nil {
		t.Fatal("PopUndo runs while a change is replaying")
	}
	tree.PushRedo(first)
	tree.Settled()
	tree.Record(second)
	if ch := tree.PopRedo(); ch != nil {
		t.Fatalf("a new change kept redo %+v", ch)
	}
	for range undoLimit + 1 {
		tree.Record(&Change{})
	}
	if n := len(tree.history.undo); n != undoLimit {
		t.Fatalf("%d undo steps; want %d", n, undoLimit)
	}
}
