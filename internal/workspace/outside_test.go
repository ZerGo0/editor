package workspace

import (
	"path/filepath"
	"testing"

	"github.com/ZerGo0/editor/internal/document"
	"github.com/ZerGo0/editor/internal/session"
)

func TestFileOutsideTheRootKeepsItsTabAcrossSessionAndHotExit(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	writeFiles(t, outside, map[string]string{"loose.txt": "disk\n"})
	p := filepath.Join(outside, "loose.txt")
	env := newTestEnv(t)
	w := newTestWorkspace(t, root, env, session.Project{})
	_, e := openTab(t, w, p)
	e.doc.Buffer.Set("edited\n")
	if w.PrepareClose() {
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
	if got := readFile(t, p); got != "edited\n" {
		t.Fatalf("saved %q", got)
	}
	if w2.rel(p) != p {
		t.Fatalf("rel(%s) = %s, want the absolute path", p, w2.rel(p))
	}
}
