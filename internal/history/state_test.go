package history

import (
	"testing"

	"github.com/ZerGo0/editor/internal/session"
)

const savedSplit = 333

func TestSplitRoundTripsThroughProjectState(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	if e.ui.split != listWidth {
		t.Fatalf("default split %v, want %v", e.ui.split, listWidth)
	}
	e.ui.split = savedSplit
	h := newFakeHost(e.root)
	h.Project = session.NewMemento(e.host.ProjectState().Snapshot())
	if got := NewUI(h, e.store).split; got != savedSplit {
		t.Fatalf("restored split %v, want %v", got, savedSplit)
	}
}
