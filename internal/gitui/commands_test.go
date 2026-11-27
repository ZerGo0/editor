package gitui

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/multidiff"
	"github.com/egoist/mygo/ui"
)

func writeWorkFile(t *testing.T, dir, rel string) string {
	t.Helper()
	abs := filepath.Join(dir, rel)
	if err := os.WriteFile(abs, []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return abs
}

func TestBranchFromPicksARefThenNamesAndChecksOut(t *testing.T) {
	r := &fakeRepo{status: &git.Status{Branch: "main", OID: "a"}, branches: []git.Branch{{Name: "main", Current: true}, {Name: "feature"}}}
	g, h := newFakeGit(t, r)
	if !g.Run("git.branchFrom") || !g.branches.open || g.branches.pick == nil {
		t.Fatal("git.branchFrom did not open the ref picker")
	}
	tt := newTester(g, func(*ui.Context) {})
	h.drain(t, func() bool { tt.Frame(); return tt.HasText("feature") })
	if err := tt.Click("feature"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if g.branches.open {
		t.Error("the picker stayed open after a ref was chosen")
	}
	if s := g.newBranchDlg; !s.open || s.start != "feature" || !s.checkout {
		t.Fatalf("name dialog = %+v, want it open from feature with checkout on", s)
	}
	g.newBranchDlg.name = "topic"
	tt.Frame()
	if err := tt.Click("Create"); err != nil {
		t.Fatal(err)
	}
	h.drain(t, func() bool { return r.called("branch topic from feature checkout true") })
}

func TestCheckoutAndBranchFromAreActions(t *testing.T) {
	var ids []string
	for _, a := range Actions {
		ids = append(ids, a.ID)
		if a.ID == "git.branchFrom" && (a.Title != "Create Branch from..." || a.Category != "Git") {
			t.Errorf("git.branchFrom = %+v", a)
		}
		if a.ID == "git.openFile2" && (a.Title != "Open File" || a.Category != "Git") {
			t.Errorf("git.openFile2 = %+v", a)
		}
	}
	for _, id := range []string{"git.branches", "git.branchFrom", "git.openFile2"} {
		if !slices.Contains(ids, id) {
			t.Errorf("no action %s", id)
		}
	}
}

func TestOpenFileFromASingleFileDiff(t *testing.T) {
	r := &fakeRepo{status: testStatus()}
	g, h := newFakeGit(t, r)
	abs := writeWorkFile(t, r.dir, "work.go")
	src := diffSource{kind: diffWorkTree, path: "work.go"}
	g.diffTabFor(src)
	h.viewID = src.id()
	if !g.Run("git.openFile2") || !slices.Equal(h.opened, []string{abs}) {
		t.Fatalf("opened %v, want %s", h.opened, abs)
	}
}

func TestOpenFileFromTheStackedDiff(t *testing.T) {
	r := &fakeRepo{status: testStatus()}
	g, h := newFakeGit(t, r)
	abs := writeWorkFile(t, r.dir, "both.go")
	view := multidiff.New()
	view.SetEntries([]multidiff.Entry{{Key: "both.go", Path: "both.go"}, {Key: "work.go", Path: "work.go"}})
	g.patchTabs["git-changes:changes"] = &patchTab{g: g, id: "git-changes:changes", view: view}
	h.viewID = "git-changes:changes"
	g.Run("git.openFile2")
	if !slices.Equal(h.opened, []string{abs}) {
		t.Fatalf("opened %v, want the current file %s", h.opened, abs)
	}
}

func selectFileRow(t *testing.T, g *Git, rel string) {
	t.Helper()
	for i, e := range g.changes.current(g.snap.status) {
		if e.kind == entryFile && e.rows[0].file.Path == rel {
			g.changes.cursor = i
			return
		}
	}
	t.Fatalf("no row for %s", rel)
}

func TestOpenFileFromTheSelectedSCMRow(t *testing.T) {
	r := &fakeRepo{status: testStatus()}
	g, h := newFakeGit(t, r)
	abs := writeWorkFile(t, r.dir, "work.go")
	selectFileRow(t, g, "work.go")
	g.Run("git.openFile2")
	if len(h.opened) != 0 || h.notes[len(h.notes)-1] != noFileToOpen {
		t.Fatalf("opened %v, notes %v, want a note when the changes list has no focus", h.opened, h.notes)
	}
	h.scmOn = true
	g.Run("git.openFile2")
	if !slices.Equal(h.opened, []string{abs}) {
		t.Fatalf("opened %v, want %s", h.opened, abs)
	}
}

func TestOpenFileSkipsMissingFilesAndGroups(t *testing.T) {
	r := &fakeRepo{status: testStatus()}
	g, h := newFakeGit(t, r)
	h.scmOn = true
	selectFileRow(t, g, "staged.go")
	g.Run("git.openFile2")
	g.changes.cursor = 0
	g.Run("git.openFile2")
	if len(h.opened) != 0 || len(h.notes) != 2 || h.notes[1] != noFileToOpen {
		t.Fatalf("opened %v, notes %v, want two notes", h.opened, h.notes)
	}
}
