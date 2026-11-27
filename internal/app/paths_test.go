package app

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/ZerGo0/editor/internal/workspace"
)

func TestReplaceWindowOrder(t *testing.T) {
	for name, tc := range map[string]struct {
		confirmed, opened bool
		want              []string
	}{
		"cancel keeps the window and opens nothing": {confirmed: false, opened: true, want: nil},
		"confirm opens the folder, then closes":     {confirmed: true, opened: true, want: []string{"open", "close"}},
		"a folder that does not open keeps window":  {confirmed: true, opened: false, want: []string{"open"}},
	} {
		t.Run(name, func(t *testing.T) {
			var calls []string
			replaceWindow(
				func(done func(ok bool)) { done(tc.confirmed) },
				func() bool { calls = append(calls, "open"); return tc.opened },
				func() { calls = append(calls, "close") },
			)
			if !slices.Equal(calls, tc.want) {
				t.Fatalf("calls = %v, want %v", calls, tc.want)
			}
		})
	}
}

func TestWindowForFilePrefersTheDeepestFolderThenTheLastActiveWindow(t *testing.T) {
	base := t.TempDir()
	outer := &projectWindow{ws: &workspace.Workspace{Root: filepath.Join(base, "outer")}}
	inner := &projectWindow{ws: &workspace.Workspace{Root: filepath.Join(base, "outer", "inner")}}
	other := &projectWindow{ws: &workspace.Workspace{Root: filepath.Join(base, "other")}}
	a := New(Options{})
	if got := a.windowForFile(filepath.Join(base, "x.txt")); got != nil {
		t.Fatal("a file found a window when none is open")
	}
	a.projects = map[string]*projectWindow{outer.ws.Root: outer, inner.ws.Root: inner, other.ws.Root: other}
	for _, pw := range []*projectWindow{outer, inner, other} {
		a.touchWindow(pw)
	}
	for name, tc := range map[string]struct {
		file string
		want *projectWindow
	}{
		"the deepest folder owns the file":                        {filepath.Join(base, "outer", "inner", "a.txt"), inner},
		"the outer folder owns its own file":                      {filepath.Join(base, "outer", "b.txt"), outer},
		"a file outside every folder uses the last active window": {filepath.Join(base, "c.txt"), other},
	} {
		if got := a.windowForFile(tc.file); got != tc.want {
			t.Fatalf("%s: got %s", name, got.ws.Root)
		}
	}
	a.touchWindow(outer)
	if got := a.windowForFile(filepath.Join(base, "c.txt")); got != outer {
		t.Fatal("focusing a window does not make it the last active one")
	}
	if got := a.lastActiveWindow(); got != outer {
		t.Fatal("lastActiveWindow ignores the focus order")
	}
}

func TestFilesOpenedBeforeTheAppIsReadyWait(t *testing.T) {
	a := New(Options{})
	a.openFile("/tmp/a.txt")
	a.openFile("/tmp/b.txt")
	if !slices.Equal(a.launchFiles, []string{"/tmp/a.txt", "/tmp/b.txt"}) {
		t.Fatalf("launch files = %v", a.launchFiles)
	}
	if a.openLaunchFiles() || a.launchFiles != nil {
		t.Fatal("launch files that do not exist opened or stayed queued")
	}
}
