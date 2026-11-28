package gitui

import (
	"slices"
	"testing"
	"time"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/egoist/mygo/ui"
)

func TestDiffCommandsStepTheActiveSingleDiff(t *testing.T) {
	tab, tt := openLongDiff(t)
	g, h := tab.g, tab.g.h.(*fakeHost)
	h.viewID = tab.src.id()
	if active, _ := g.DiffContext(); !active {
		t.Fatal("the active single diff is not reported as active")
	}
	for range 2 {
		g.Run(nextChangeCommand)
		tt.Frame()
	}
	if i, inside := tab.state.CurrentHunk(); i != 2 || !inside {
		t.Fatalf("after two Next Change the caret is in hunk %d (inside %v), want 2", i, inside)
	}
	if _, focused := g.DiffContext(); !focused {
		t.Error("the diff has no focus after Next Change")
	}
	g.Run(previousDifferenceCommand)
	tt.Frame()
	if i, _ := tab.state.CurrentHunk(); i != 1 {
		t.Errorf("after Go to Previous Difference the caret is in hunk %d, want 1", i)
	}
}

func TestOpenFileFromASingleDiffOpensTheCaretLine(t *testing.T) {
	tab, tt := openLongDiff(t)
	g, h := tab.g, tab.g.h.(*fakeHost)
	h.viewID = tab.src.id()
	tab.goToChange(1)
	tt.Frame()
	want := tab.state.Hunks()[1].NewStart
	if !g.Run(openFileCommand) || !slices.Equal(h.lines, []int{want}) {
		t.Fatalf("opened %v at lines %v, want line %d", h.opened, h.lines, want)
	}
}

func TestOpenDiffFocusesIt(t *testing.T) {
	tab, tt := openLongDiff(t)
	tt.Frame()
	if !tab.focused {
		t.Fatal("the diff opened with focus has no keyboard focus")
	}
}

func TestLogKeysCopyTheRevisionAndJumpToSource(t *testing.T) {
	r := &fakeRepo{
		status:  &git.Status{Branch: "main", OID: "c2"},
		commits: []git.Commit{{OID: "c2", Parents: []string{"c1"}, Author: "Ada", AuthorTime: time.Now(), Subject: "Second change"}},
		files:   []git.ChangedFile{{Path: "app.go", Status: 'M'}},
	}
	g, h := newFakeGit(t, r)
	abs := writeWorkFile(t, r.dir, "app.go")
	g.Run("git.showLog")
	tt := newTester(g, g.LogPanel)
	h.drain(t, func() bool { tt.Frame(); return tt.HasText("Second change") })
	tt.Frame()
	if !tt.Focused("Commits") {
		t.Fatal("Show Log did not focus the commit list")
	}
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	tt.Key(copyRevisionMods, copyRevisionKey)
	tt.Frame()
	if got := tt.Clipboard(); got != "c2" {
		t.Fatalf("clipboard = %q, want the revision c2", got)
	}
	h.drain(t, func() bool { tt.Frame(); return tt.HasText("app.go") })
	if err := tt.Click("app.go"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Key(0, jumpToSourceKey)
	tt.Frame()
	if !slices.Equal(h.opened, []string{abs}) {
		t.Fatalf("F4 opened %v, want %s", h.opened, abs)
	}
	clear(h.views)
	tt.Key(showDiffMods, showDiffKey)
	tt.Frame()
	if _, ok := h.views["git-commit:c2"]; !ok {
		t.Errorf("Cmd+D opened %v, want the commit diff", h.views)
	}
}

func TestSourceControlRowKeys(t *testing.T) {
	r := &fakeRepo{status: testStatus()}
	g, h := newFakeGit(t, r)
	abs := writeWorkFile(t, r.dir, "work.go")
	tt := newTester(g, g.SideView)
	h.drain(t, func() bool { tt.Frame(); return tt.HasText("work.go") })
	if err := tt.Click("work.go"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	clear(h.views)
	tt.Key(0, jumpToSourceKey)
	tt.Frame()
	if !slices.Equal(h.opened, []string{abs}) {
		t.Fatalf("F4 opened %v, want %s", h.opened, abs)
	}
	tt.Key(showDiffMods, showDiffKey)
	tt.Frame()
	if _, ok := h.views["git-changes:changes"]; !ok {
		t.Errorf("Cmd+D opened %v, want the changes diff", h.views)
	}
}

func TestConflictsDialogOpensTheMergeToolOnEnter(t *testing.T) {
	r := &fakeRepo{
		status:   &git.Status{Branch: "main", OID: "a", Files: []git.FileStatus{{Path: "m.txt", Index: 'U', WorkTree: 'U', Conflict: true}}},
		versions: [3]string{"a\nb\nc\n", "a\nours\nc\n", "a\ntheirs\nc\n"},
		conflict: []git.ConflictFile{{Path: "m.txt", Base: true, Ours: true, Theirs: true}},
	}
	g, h := newFakeGit(t, r)
	g.openConflicts()
	tt := newTester(g, func(*ui.Context) {})
	h.drain(t, func() bool { tt.Frame(); return tt.HasText("m.txt") })
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if _, ok := g.mergeTools["git-merge:m.txt"]; !ok || g.conflicts.open {
		t.Fatalf("Enter did not open the merge tool (dialog open %v)", g.conflicts.open)
	}
}

func TestMergeToolKeysStepAndAccept(t *testing.T) {
	r := &fakeRepo{
		status:   &git.Status{Branch: "main", OID: "a", Files: []git.FileStatus{{Path: "m.txt", Index: 'U', WorkTree: 'U', Conflict: true}}},
		versions: [3]string{"a\nb\nc\nd\ne\n", "a\nours\nc\nd\nmine\n", "a\ntheirs\nc\nd\nyours\n"},
	}
	g, h := newFakeGit(t, r)
	g.openMergeTool("m.txt")
	tool := g.mergeTools["git-merge:m.txt"]
	h.drain(t, func() bool { return tool.model != nil })
	tt := newTester(g, tool.view)
	tt.Frame()
	tt.Frame()
	if !tt.HasText("2 conflicts left") {
		t.Fatalf("texts = %v", tt.Texts())
	}
	tt.Key(0, ui.KeyF7)
	tt.Frame()
	tt.Key(ui.Ctrl|ui.Shift, ui.KeyLeft)
	tt.Frame()
	if got := tool.result.Buffer.String(); got != "a\ntheirs\nc\nd\ne\n" {
		t.Fatalf("after F7 and Accept Right the result is %q", got)
	}
	tt.Key(0, ui.KeyF7)
	tt.Frame()
	tt.Key(ui.Ctrl|ui.Shift, ui.KeyRight)
	tt.Frame()
	if got := tool.result.Buffer.String(); got != "a\ntheirs\nc\nd\nmine\n" {
		t.Fatalf("after F7 and Accept Left the result is %q", got)
	}
	tt.Key(ui.Shift, ui.KeyF7)
	tt.Frame()
	if first := tool.model.changes()[0]; tool.current != first {
		t.Errorf("after Shift+F7 the current change is %d, want %d", tool.current, first)
	}
}
