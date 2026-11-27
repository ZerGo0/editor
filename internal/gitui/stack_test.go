package gitui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/git/diffmode"
	"github.com/ZerGo0/editor/internal/multidiff"
	"github.com/ZerGo0/editor/internal/testutil"
)

const (
	changesTabID  = "git-changes:changes"
	stagedTabID   = "git-changes:staged"
	largeFileSize = multidiff.LargeDiffBytes + 1
)

func stackRepo(t *testing.T, extra ...func(dir string)) (*Git, *fakeHost, string) {
	t.Helper()
	if !git.Available() {
		t.Skip("git is not installed")
	}
	dir := newTempRepo(t)
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		testutil.WriteFile(t, filepath.Join(dir, name), "one\ntwo\nthree\n")
	}
	commitAll(t, dir, "init")
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		testutil.WriteFile(t, filepath.Join(dir, name), "one\nchanged "+name+"\nthree\n")
	}
	testutil.WriteFile(t, filepath.Join(dir, "new.txt"), "fresh\n")
	for _, write := range extra {
		write(dir)
	}
	useSession(t, diffmode.Options{})
	g, h := newRealGit(t, dir)
	return g, h, dir
}

func loadedTab(t *testing.T, g *Git, h *fakeHost, id string) *patchTab {
	t.Helper()
	tab := g.patchTabs[id]
	if tab == nil {
		t.Fatalf("no tab %s in %v", id, g.patchOrder)
	}
	h.drain(t, func() bool { return tab.loaded && !tab.loading })
	return tab
}

func TestChangesTabStacksEveryChangedFile(t *testing.T) {
	g, h, _ := stackRepo(t)
	g.openDiffWith(diffSource{kind: diffWorkTree, path: "b.txt"}, false, true)
	tab := loadedTab(t, g, h, changesTabID)
	var paths []string
	for _, e := range tab.view.Entries() {
		paths = append(paths, e.Path)
	}
	if got := strings.Join(paths, ","); got != "a.txt,b.txt,c.txt,new.txt" {
		t.Fatalf("stacked files = %s", got)
	}
	if e := tab.view.Entries()[1]; e.Adds != 1 || e.Dels != 1 {
		t.Errorf("counts of b.txt: +%d -%d", e.Adds, e.Dels)
	}
	tt := newTester(g, tab.render)
	for range 3 {
		tt.Frame()
	}
	h.drain(t, func() bool { tt.Frame(); return tab.view.File(tab.view.Entries()[1].Key).Settled() })
	if !tt.HasText("Diff Toolbar") && !tt.HasText("4 files") {
		t.Errorf("texts = %v", tt.Texts())
	}
}

func TestStageFromTheFileHeaderUpdatesTheStack(t *testing.T) {
	g, h, _ := stackRepo(t)
	g.openDiffWith(diffSource{kind: diffWorkTree, path: "a.txt"}, false, true)
	tab := loadedTab(t, g, h, changesTabID)
	tt := newTester(g, tab.render)
	for range 3 {
		tt.Frame()
	}
	if err := tt.Click("Stage Changes"); err != nil {
		t.Fatal(err)
	}
	h.drain(t, func() bool { return len(tab.view.Entries()) == 3 })
	if got := tab.view.Entries()[0].Path; got != "b.txt" {
		t.Errorf("first file after staging a.txt: %s", got)
	}
}

func TestCommitTabStacksTheFilesOfTheCommit(t *testing.T) {
	g, h, dir := stackRepo(t)
	testutil.RunGit(t, dir, "add", "-A")
	testutil.RunGit(t, dir, "commit", "-q", "-m", "second")
	oid := testutil.RunGit(t, dir, "rev-parse", "HEAD")
	g.openDiffWith(commitSource(git.Commit{OID: oid, Parents: []string{testutil.RunGit(t, dir, "rev-parse", "HEAD~1")}}, git.ChangedFile{Path: "c.txt", Status: 'M'}), false, true)
	tab := loadedTab(t, g, h, "git-commit:"+oid)
	var files int
	for _, e := range tab.view.Entries() {
		if e.Section == nil {
			files++
			if e.Adds != 1 && e.Path != "new.txt" {
				t.Errorf("counts of %s: +%d", e.Path, e.Adds)
			}
		}
	}
	if files != 4 {
		t.Fatalf("files in the commit tab: %d", files)
	}
	if title := h.views["git-commit:"+oid]; !strings.HasPrefix(title, "Commit "+oid[:7]) {
		t.Errorf("title %q", title)
	}
}

func TestLargeFilesWaitForLoadDiffInTheStack(t *testing.T) {
	g, h, _ := stackRepo(t, func(dir string) {
		testutil.WriteFile(t, filepath.Join(dir, "big.txt"), strings.Repeat("x", largeFileSize))
	})
	g.openDiffWith(diffSource{kind: diffWorkTree, path: "big.txt"}, false, true)
	tab := loadedTab(t, g, h, changesTabID)
	h.drain(t, func() bool { return len(tab.view.Entries()) == 5 })
	tt := newTester(g, tab.render)
	for range 3 {
		tt.Frame()
	}
	if err := tt.Click("Load Diff"); err != nil {
		t.Fatalf("no Load Diff: %v (texts %v)", err, tt.Texts())
	}
}
