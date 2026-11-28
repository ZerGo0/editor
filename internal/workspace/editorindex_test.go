package workspace

import (
	"testing"

	"github.com/ZerGo0/editor/internal/problems"
	"github.com/ZerGo0/editor/internal/session"
)

func TestOpenEditorAtIndexAndLast(t *testing.T) {
	t.Parallel()
	b := newFilesBed(t)
	b.open("a.txt", "b.txt", "c.txt")
	b.w.OpenEditorAtIndex(0)
	b.settle()
	if got := b.tabs(); got != "*a.txt,b.txt,c.txt" {
		t.Fatalf("after index 1: %s", got)
	}
	b.w.OpenEditorAtIndex(8)
	b.w.LastEditorInGroup()
	b.settle()
	if got := b.tabs(); got != "a.txt,b.txt,*c.txt" {
		t.Fatalf("after last: %s", got)
	}
}

func TestMaximizeEditorGroupShowsOnlyTheActiveGroup(t *testing.T) {
	t.Parallel()
	b := newGroupsBed(t)
	b.focusGroup(0)
	b.w.ToggleMaximizeEditorGroup()
	b.settle()
	if n := len(b.editorRects()); n != 1 {
		t.Fatalf("maximized: %d groups shown, want 1", n)
	}
	b.w.ToggleMaximizeEditorGroup()
	b.settle()
	if n := len(b.editorRects()); n != 2 {
		t.Fatalf("restored: %d groups shown, want 2", n)
	}
	b.w.ToggleMaximizeEditorGroup()
	b.w.FocusGroup(1)
	b.settle()
	if n := len(b.editorRects()); n != 2 {
		t.Fatalf("focusing another group must restore the groups, %d shown", n)
	}
}

func TestHideToolWindowsTogglesSideBarAndPanel(t *testing.T) {
	t.Parallel()
	b := newFilesBed(t)
	b.open("a.txt")
	b.w.tools.panelOpen = true
	b.w.ToggleHideToolWindows()
	if !b.w.sidebarHidden || b.w.tools.panelOpen {
		t.Fatal("the side bar and panel must hide")
	}
	b.w.ToggleHideToolWindows()
	if b.w.sidebarHidden || !b.w.tools.panelOpen {
		t.Fatal("the side bar and panel must come back")
	}
}

func TestNextProblemWrapsThroughFiles(t *testing.T) {
	t.Parallel()
	files := map[string][]problems.Problem{
		"/a.go": {{Line: 5, Col: 1}, {Line: 2, Col: 0}},
		"/b.go": {{Line: 0, Col: 3}},
		"/c.go": nil,
	}
	for _, tc := range []struct {
		name   string
		here   session.Location
		step   int
		across bool
		want   session.Location
	}{
		{"next in file", session.Location{Path: "/a.go", Line: 2}, 1, true, session.Location{Path: "/a.go", Line: 5, Col: 1}},
		{"next file", session.Location{Path: "/a.go", Line: 9}, 1, true, session.Location{Path: "/b.go", Line: 0, Col: 3}},
		{"wraps to first", session.Location{Path: "/b.go", Line: 4}, 1, true, session.Location{Path: "/a.go", Line: 2}},
		{"previous file", session.Location{Path: "/b.go", Line: 0}, -1, true, session.Location{Path: "/a.go", Line: 5, Col: 1}},
		{"from a clean file", session.Location{Path: "/c.go", Line: 3}, -1, true, session.Location{Path: "/b.go", Line: 0, Col: 3}},
		{"stays in file", session.Location{Path: "/a.go", Line: 9}, 1, false, session.Location{Path: "/a.go", Line: 2}},
	} {
		got, ok := nextProblem(files, tc.here, tc.step, tc.across)
		if !ok || got != tc.want {
			t.Errorf("%s: got %+v %v, want %+v", tc.name, got, ok, tc.want)
		}
	}
	if _, ok := nextProblem(files, session.Location{Path: "/c.go"}, 1, false); ok {
		t.Error("a clean file has no next problem in it")
	}
}
