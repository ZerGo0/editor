package workspace

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/ZerGo0/editor/internal/document"
	"github.com/ZerGo0/editor/internal/session"
	"github.com/ZerGo0/editor/internal/testutil"
)

func TestFileOutsideTheRootKeepsItsTabAcrossSessionAndHotExit(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	testutil.WriteFiles(t, outside, map[string]string{"loose.txt": "disk\n"})
	p := filepath.Join(outside, "loose.txt")
	env := newTestEnv(t)
	w := newTestWorkspace(t, root, env, session.Project{})
	_, e := openTab(t, w, p)
	e.doc.Buffer.Set("edited\n")
	if w.PrepareClose(AppQuit) {
		t.Fatal("hot exit asks to save the backed-up file outside the root")
	}
	snap := w.Snapshot()
	if got := snap.Layout.Group.Tabs; len(got) != 1 || got[0].Path != p {
		t.Fatalf("snapshot tabs = %+v, want the absolute path %s", got, p)
	}

	w2 := newTestWorkspace(t, root, env, snap)
	if tab := w2.active.ActiveTab(); tab == nil || tab.Path != p {
		t.Fatalf("active tab after restore = %+v, want %s", tab, p)
	}
	d := w2.entry(p).doc
	if d == nil || d.Buffer.String() != "edited\n" || !d.Modified() {
		t.Fatal("the unsaved text outside the root did not restore")
	}
	if !w2.save([]*document.Document{d}) {
		t.Fatal("save failed")
	}
	if got := testutil.ReadText(t, p); got != "edited\n" {
		t.Fatalf("saved %q", got)
	}
	if w2.rel(p) != p {
		t.Fatalf("rel(%s) = %s, want the absolute path", p, w2.rel(p))
	}
}

func TestRecentFileOutsideTheRootSurvivesARestartUntilItIsDeleted(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	testutil.WriteFiles(t, outside, map[string]string{"loose.txt": "disk\n", "gone.txt": "disk\n"})
	loose, gone := filepath.Join(outside, "loose.txt"), filepath.Join(outside, "gone.txt")
	env := newTestEnv(t)
	w := newTestWorkspace(t, root, env, session.Project{})
	openTab(t, w, gone)
	openTab(t, w, loose)
	snap := w.Snapshot()
	if !slices.Equal(snap.RecentFiles, []string{loose, gone}) {
		t.Fatalf("snapshot recent files = %v", snap.RecentFiles)
	}
	if err := os.Remove(gone); err != nil {
		t.Fatal(err)
	}
	w2 := newTestWorkspace(t, root, env, snap)
	if !slices.Equal(w2.search.recent, []string{loose}) {
		t.Fatalf("recent files after restart = %v, want only %s", w2.search.recent, loose)
	}
}

func TestCrumbPathShowsEveryFolderOfAFileOutsideTheRoot(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	w := newTestWorkspace(t, root, newTestEnv(t), session.Project{})
	sep := string(filepath.Separator)
	for name, tc := range map[string]struct {
		file  string
		names []string
		last  string
	}{
		"inside the root":    {filepath.Join(root, "a", "b.txt"), []string{"a", "b.txt"}, filepath.Join(root, "a", "b.txt")},
		"inside the home":    {filepath.Join(home, "d", "e.txt"), []string{"~", "d", "e.txt"}, filepath.Join(home, "d", "e.txt")},
		"outside everything": {sep + filepath.Join("opt", "x", "f.txt"), []string{"opt", "x", "f.txt"}, sep + filepath.Join("opt", "x", "f.txt")},
	} {
		t.Run(name, func(t *testing.T) {
			names, paths := w.crumbPath(tc.file)
			if !slices.Equal(names, tc.names) || len(paths) != len(names) || paths[len(paths)-1] != tc.last {
				t.Fatalf("names %v paths %v", names, paths)
			}
		})
	}
}
