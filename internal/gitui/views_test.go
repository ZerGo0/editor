package gitui

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	testWidth  = 1000
	testHeight = 700
)

func newTester(g *Git, view func(c *ui.Context)) *ui.Tester {
	return ui.NewTester(func(c *ui.Context) {
		view(c)
		g.Overlay(c)
	}, testWidth, testHeight)
}

func TestNoRepoShowsInit(t *testing.T) {
	h := newFakeHost(tempDir(t))
	inits := make(chan struct{}, 1)
	g := newGit(h, deps{
		available: func() bool { return true },
		open:      func(context.Context, string, string) (repo, error) { return nil, git.ErrNotRepo },
		initRepo:  func(context.Context, string) error { inits <- struct{}{}; return nil },
		watch:     func(context.Context, string, func()) {},
		ask:       func(*mygo.Window, question) bool { return true },
	})
	defer g.Close()
	h.drain(t, func() bool { return g.phase == phaseNoRepo })
	tt := newTester(g, g.SideView)
	if !tt.HasText("This folder is not a Git repository") {
		t.Fatalf("texts = %v", tt.Texts())
	}
	if err := tt.Click("Initialize Repository"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-inits:
	case <-time.After(drainTimeout):
		t.Fatal("git init not run")
	}
	if g.Run("git.push") != true {
		t.Fatal("known action not handled without a repo")
	}
}

func TestNoGit(t *testing.T) {
	h := newFakeHost(tempDir(t))
	g := newGit(h, deps{available: func() bool { return false }})
	defer g.Close()
	h.drain(t, func() bool { return g.phase == phaseNoGit })
	tt := newTester(g, g.SideView)
	if !tt.HasText("Git is not installed") {
		t.Fatalf("texts = %v", tt.Texts())
	}
}

func TestCommitWindow(t *testing.T) {
	st := testStatus()
	st.Files = st.Files[:4]
	r := &fakeRepo{status: st, branches: []git.Branch{{Name: "main", Current: true}}}
	g, h := newFakeGit(t, r)
	tt := newTester(g, g.SideView)
	for _, want := range []string{"staged.go", "work.go", "new.go", "Changes", "Staged Changes", "Graph"} {
		if !tt.HasText(want) {
			t.Fatalf("missing %q in %v", want, tt.Texts())
		}
	}
	if _, ok := tt.Access("Commit Message"); !ok {
		t.Fatal("no commit message field")
	}
	if g.ChangeCount() != 4 {
		t.Fatalf("ChangeCount = %d", g.ChangeCount())
	}
	g.commit.message = "Fix the bug"
	tt.Frame()
	if err := tt.Click("Commit"); err != nil {
		t.Fatal(err)
	}
	h.drain(t, func() bool { return r.called("commit") })
	h.drain(t, func() bool { return g.commit.message == "" })
	if r.commitOK.Message != "Fix the bug" || len(r.commitOK.Paths) != 0 {
		t.Fatalf("commit options = %+v", r.commitOK)
	}
}

func TestStageButton(t *testing.T) {
	st := testStatus()
	st.Files = st.Files[:4]
	r := &fakeRepo{status: st}
	g, h := newFakeGit(t, r)
	tt := newTester(g, g.SideView)
	hover(t, tt, "Changes")
	if err := tt.Click("Stage All Changes"); err != nil {
		t.Fatal(err)
	}
	h.drain(t, func() bool { return r.called("stage both.go,new.go,work.go") })
}

func hover(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	var b ui.Rect
	for _, info := range tt.AccessAll() {
		if info.Name == label {
			b = info.Bounds
		}
	}
	if b.W == 0 {
		var roles []string
		for _, info := range tt.AccessAll() {
			roles = append(roles, fmt.Sprintf("%v:%s", info.Role, info.Name))
		}
		t.Fatalf("no row %q in %v", label, roles)
	}
	tt.Move(b.X+b.W/2, b.Y+b.H/2)
	tt.Frame()
}

func TestChangesTreeAndPreview(t *testing.T) {
	st := &git.Status{Branch: "main", OID: "a", Files: []git.FileStatus{
		{Path: "src/app/main.go", Index: '.', WorkTree: 'M'},
		{Path: "src/app/util.go", Index: '.', WorkTree: 'D'},
	}}
	r := &fakeRepo{status: st}
	g, h := newFakeGit(t, r)
	tt := newTester(g, g.SideView)
	if !tt.HasText("src/app") || !tt.HasText("main.go") {
		t.Fatalf("tree = %v", tt.Texts())
	}
	if err := tt.Click("main.go"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if _, ok := h.views["git-changes:changes"]; !ok {
		t.Fatalf("diff not opened: %v", h.views)
	}
	if err := tt.Click("src/app"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if tt.HasText("main.go") {
		t.Fatal("folder did not collapse")
	}
	g.changes.setListMode(true)
	tt.Frame()
	if !tt.HasText("util.go") || !tt.HasText("src/app") {
		t.Fatalf("list mode = %v", tt.Texts())
	}
	hover(t, tt, "util.go")
	if err := tt.Click("Stage Changes"); err != nil {
		t.Fatal(err)
	}
	h.drain(t, func() bool { return r.called("stage src/app/util.go") })
}

func TestPrimaryActionSync(t *testing.T) {
	r := &fakeRepo{status: &git.Status{Branch: "main", OID: "a", Upstream: "origin/main", Ahead: 2, Behind: 1}}
	g, _ := newFakeGit(t, r)
	tt := newTester(g, g.SideView)
	if !tt.HasText("Sync Changes \u21912 \u21931") {
		t.Fatalf("texts = %v", tt.Texts())
	}
	g.commit.message = "Fix"
	tt.Frame()
	if !tt.HasText(commitLabel) {
		t.Fatalf("texts = %v", tt.Texts())
	}
}

func TestBranchesPopup(t *testing.T) {
	r := &fakeRepo{status: &git.Status{Branch: "main", OID: "a"}, branches: []git.Branch{
		{Name: "main", Current: true}, {Name: "feature"},
	}}
	g, h := newFakeGit(t, r)
	g.openBranches()
	tt := newTester(g, func(*ui.Context) {})
	h.drain(t, func() bool { tt.Frame(); return tt.HasText("feature") })
	if err := tt.Click("feature"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("Merge 'feature' into 'main'") {
		t.Fatalf("actions = %v", tt.Texts())
	}
	if err := tt.Click("Checkout"); err != nil {
		t.Fatal(err)
	}
	h.drain(t, func() bool { return r.called("checkout feature") })
	if g.branches.open {
		t.Fatal("popup still open")
	}
}

func TestLogPanel(t *testing.T) {
	now := time.Now()
	r := &fakeRepo{
		status: &git.Status{Branch: "main", OID: "c2"},
		commits: []git.Commit{
			{OID: "c2", Parents: []string{"c1"}, Author: "Ada", AuthorTime: now, Subject: "Second change", Refs: []string{"HEAD -> main"}},
			{OID: "c1", Author: "Ada", AuthorTime: now, Subject: "First change"},
		},
		files: []git.ChangedFile{{Path: "src/app.go", Status: 'M'}},
	}
	g, h := newFakeGit(t, r)
	tt := newTester(g, g.LogPanel)
	h.drain(t, func() bool { tt.Frame(); return tt.HasText("Second change") })
	if !tt.HasText("Select a commit") {
		t.Fatalf("texts = %v", tt.Texts())
	}
	if err := tt.Click("First change"); err != nil {
		t.Fatal(err)
	}
	h.drain(t, func() bool { tt.Frame(); return tt.HasText("app.go") })
	if !tt.HasText("In main") {
		t.Fatalf("details = %v", tt.Texts())
	}
}

func TestRunActions(t *testing.T) {
	r := &fakeRepo{status: &git.Status{Branch: "main", OID: "a"}}
	g, _ := newFakeGit(t, r)
	if g.Run("nope") || g.Run("git.unknown") {
		t.Fatal("unknown action handled")
	}
	table := g.actionTable()
	for _, a := range Actions {
		if !strings.HasPrefix(a.ID, "git.") {
			t.Errorf("action %s has no git. prefix", a.ID)
		}
		if _, ok := table[a.ID]; !ok && a.ID != "git.init" {
			t.Errorf("action %s has no handler", a.ID)
		}
		if a.Key == "cmd+k" {
			t.Errorf("action %s uses cmd+k", a.ID)
		}
	}
	if !g.Run("git.commit") || !g.commit.focus {
		t.Fatal("git.commit did not focus the message")
	}
	if !g.Run("git.showLog") || !g.WantsPanel() || g.WantsPanel() {
		t.Fatal("WantsPanel is not one-shot")
	}
	if !g.Run("git.stash") || !g.stash.open {
		t.Fatal("stash dialog not open")
	}
}

func TestBaseCache(t *testing.T) {
	st := &git.Status{Branch: "main", OID: "head1", Files: []git.FileStatus{{Path: "new.go", Index: '?', WorkTree: '?'}}}
	r := &fakeRepo{status: st, shows: map[string]string{"head1:a.go": "package a\n"}}
	g, h := newFakeGit(t, r)
	abs := g.paths.abs("a.go")
	if g.Base(abs) != nil {
		t.Fatal("base ready before the load")
	}
	h.drain(t, func() bool { return g.Base(abs) != nil })
	if got := g.Base(abs).String(); got != "package a\n" {
		t.Fatalf("base = %q", got)
	}
	if g.Base(g.paths.abs("new.go")) != nil {
		t.Fatal("untracked file has a base")
	}
	g.Base(g.paths.abs("missing.go"))
	h.settle()
	if g.Base(g.paths.abs("missing.go")) != nil {
		t.Fatal("missing file has a base")
	}
}
