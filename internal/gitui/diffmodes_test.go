package gitui

import (
	"context"
	"strings"
	"testing"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/git/diffmode"
	"github.com/egoist/mygo"
)

const (
	tabbedCode  = "func a() int {\n\tx := 1\n\ty := 2\n\treturn x + y\n}\n"
	spacedCode  = "func a() int {\n    x := 1\n    y := 2\n    return x + y\n}\n"
	editedCode  = "func a() int {\n    x := 1\n    y := 3\n    return x + y\n}\n"
	codePath    = "main.go"
	sessionNone = git.WhitespaceNone
)

func useSession(t *testing.T, o diffmode.Options) {
	t.Helper()
	saved := diffmode.Session
	t.Cleanup(func() { diffmode.Session = saved })
	diffmode.Session = o
}

func newRealGit(t *testing.T, dir string) (*Git, *fakeHost) {
	t.Helper()
	r := openTemp(t, dir)
	h := newFakeHost(dir)
	g := newGit(h, deps{
		available: func() bool { return true },
		open:      func(context.Context, string, string) (repo, error) { return r, nil },
		initRepo:  func(context.Context, string) error { return nil },
		watch:     func(context.Context, string, func()) {},
		ask:       func(*mygo.Window, question) bool { return true },
	})
	t.Cleanup(g.Close)
	h.drain(t, func() bool { return g.ready() && g.snap.status != nil })
	return g, h
}

func TestIgnoreWhitespacesHidesIndentationChange(t *testing.T) {
	if !git.Available() {
		t.Skip("git is not installed")
	}
	dir := newTempRepo(t)
	writeFile(t, dir, codePath, tabbedCode)
	commitAll(t, dir, "init")
	writeFile(t, dir, codePath, editedCode)
	useSession(t, diffmode.Options{Whitespace: git.WhitespaceIgnore})
	g, h := newRealGit(t, dir)
	g.openDiff(diffWorkTree, codePath)
	tab := g.tabs[diffSource{kind: diffWorkTree, path: codePath}.id()]
	h.drain(t, func() bool { return tab.engine != nil && tab.engine.Filtered() })
	if added, deleted := tab.state.Stats(); added != 1 || deleted != 1 {
		t.Fatalf("ignored stats = +%d -%d", added, deleted)
	}
	if tab.state.New.String() != editedCode {
		t.Fatalf("new text = %q", tab.state.New.String())
	}
	tt := newTester(g, tab.view)
	for _, want := range []string{"Ignore whitespaces", "Show Moved Code", "Read only"} {
		if !tt.HasText(want) {
			t.Fatalf("missing %q in %v", want, tt.Texts())
		}
	}
	diffmode.Session.Whitespace = sessionNone
	tt.Frame()
	if added, _ := tab.state.Stats(); added < 3 {
		t.Fatalf("plain stats = +%d", added)
	}
	if tab.engine.Filtered() {
		t.Fatal("still filtered in plain mode")
	}
}

func TestIgnoreWhitespacesOnlyIndentation(t *testing.T) {
	if !git.Available() {
		t.Skip("git is not installed")
	}
	dir := newTempRepo(t)
	writeFile(t, dir, codePath, tabbedCode)
	commitAll(t, dir, "init")
	writeFile(t, dir, codePath, spacedCode)
	useSession(t, diffmode.Options{Whitespace: git.WhitespaceIgnore})
	g, h := newRealGit(t, dir)
	g.openDiff(diffWorkTree, codePath)
	tab := g.tabs[diffSource{kind: diffWorkTree, path: codePath}.id()]
	h.drain(t, func() bool { return tab.engine != nil && tab.engine.Filtered() })
	if hunks := tab.state.Hunks(); len(hunks) != 0 {
		t.Fatalf("hunks = %+v", hunks)
	}
	diffmode.Session.Whitespace = sessionNone
	newTester(g, tab.view).Frame()
	if hunks := tab.state.Hunks(); len(hunks) == 0 {
		t.Fatal("the indentation change is hidden in plain mode")
	}
}

func TestPatchTabFiltersWhitespace(t *testing.T) {
	if !git.Available() {
		t.Skip("git is not installed")
	}
	dir := newTempRepo(t)
	writeFile(t, dir, codePath, tabbedCode)
	commitAll(t, dir, "init")
	writeFile(t, dir, codePath, spacedCode)
	useSession(t, diffmode.Options{})
	g, h := newRealGit(t, dir)
	g.openPatchTab("test-patch", "Patch", func(ctx context.Context, r repo) ([]patchItem, error) {
		fds, err := r.DiffWorkTree(ctx)
		return fileDiffItems(fds), err
	})
	tab := g.patchTabs["test-patch"]
	h.drain(t, func() bool { return !tab.loading && len(tab.items) > 0 })
	tt := newTester(g, tab.view)
	if !tt.HasText("Ignore Whitespace") || tt.HasText("Only whitespace changed") {
		t.Fatalf("texts = %v", tt.Texts())
	}
	diffmode.Session.Whitespace = git.WhitespaceIgnore
	tt.Frame()
	if !tt.HasText("Only whitespace changed") {
		t.Fatalf("texts = %v", tt.Texts())
	}
	if !strings.Contains(strings.Join(tt.Texts(), " "), "Ignore whitespaces") {
		t.Fatalf("mode not shown in %v", tt.Texts())
	}
}
