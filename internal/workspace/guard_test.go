package workspace

import (
	"testing"

	"github.com/ZerGo0/editor/internal/layout"
	"github.com/ZerGo0/editor/internal/session"
	"github.com/ZerGo0/editor/internal/testutil"
	"github.com/egoist/mygo/ui"
)

func newGuardBed(t *testing.T) (*Workspace, *ui.Tester) {
	t.Helper()
	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{"a.txt": "a\n"})
	w := newTestWorkspace(t, root, newTestEnv(t), session.Project{})
	return w, ui.NewTester(w.View, testWideW, testViewH)
}

func panicTitles(w *Workspace) int {
	n := 0
	for _, note := range w.notes.List() {
		if note.Title == panicTitle {
			n++
		}
	}
	return n
}

func TestAPanicInTheViewShowsAnErrorAndTheWindowRecovers(t *testing.T) {
	w, tt := newGuardBed(t)
	tt.Frame()
	saved := w.layout
	w.layout = &layout.Node{}
	tt.Frame()
	if w.failedBuilds == 0 {
		t.Fatal("the broken layout did not panic the view")
	}
	if n := panicTitles(w); n != 1 {
		t.Fatalf("%d error notifications, want 1", n)
	}
	w.layout = saved
	tt.Frame()
	if w.failedBuilds != 0 {
		t.Fatalf("failed builds = %d after a clean frame, want 0", w.failedBuilds)
	}
}

func TestAViewThatKeepsPanickingAsksForAReloadOnce(t *testing.T) {
	w, tt := newGuardBed(t)
	tt.Frame()
	reloads := 0
	w.OnPanicLoop = func() { reloads++ }
	w.layout = &layout.Node{}
	for range panicLoopBuilds * 2 {
		tt.Frame()
	}
	if reloads != 1 {
		t.Fatalf("reload asked %d times, want 1", reloads)
	}
}

func TestAPanicInAQueuedUpdateKeepsTheFrame(t *testing.T) {
	w, tt := newGuardBed(t)
	ran := false
	w.update(func() { panic("boom") })
	w.update(func() { ran = true })
	tt.Frame()
	if !ran {
		t.Fatal("the update after the panicking one did not run")
	}
	if w.failedBuilds != 0 {
		t.Fatalf("failed builds = %d, want 0: the update panic broke the frame", w.failedBuilds)
	}
	if n := panicTitles(w); n != 1 {
		t.Fatalf("%d error notifications, want 1", n)
	}
}
