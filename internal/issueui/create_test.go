package issueui

import (
	"strings"
	"testing"

	"github.com/ZerGo0/editor/internal/ghshared/ghtest"
	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/github"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

func (hs *harness) openCreateView(t *testing.T) (*createForm, *ui.Tester) {
	t.Helper()
	hs.be.labels = []github.Label{{Name: "bug"}, {Name: "ui"}}
	hs.be.users = []github.User{{Login: "bob"}, {Login: "carol"}}
	hs.be.milestones = []github.Milestone{{Number: 1, Title: "v1"}, {Number: 2, Title: "v2"}}
	hs.ready(t)
	hs.i.openCreate(createDraft{})
	hs.h.Pump(t, func() bool { return hs.i.meta.loaded })
	return hs.i.create, viewTester(hs, createViewID)
}

func TestCreateFormShowsFields(t *testing.T) {
	hs := newHarness(t)
	f, tt := hs.openCreateView(t)
	if f.milestone != noMilestoneOption {
		t.Errorf("milestone %q", f.milestone)
	}
	ghtest.MustHaveText(t, tt, "Create Issue", "Title", "Description", "Labels", "Assignees", "Milestone", "No milestone", "Enter a title.")
	for _, name := range []string{"Title", "Description", "Labels", "Assignees", "Milestone"} {
		if _, ok := tt.Access(name); !ok {
			t.Errorf("no accessible element %q", name)
		}
	}
	if a, ok := tt.AccessByRole(ui.RoleButton, "Create issue"); !ok || !a.Disabled {
		t.Errorf("create button %+v %v", a, ok)
	}
	if hs.h.Opened[0] != "issue/new" {
		t.Errorf("opened %v", hs.h.Opened)
	}
}

func TestCreateSubmitsAndOpensIssue(t *testing.T) {
	hs := newHarness(t)
	f, tt := hs.openCreateView(t)
	f.title, f.body = "  My title ", "Body"
	f.labels, f.assignees, f.milestone = []string{"bug"}, []string{"bob"}, "v2"
	tt.Frame()
	ghtest.MustClick(t, tt, "Create")
	hs.h.Pump(t, func() bool { return hs.i.create == nil })
	want := github.IssueInput{Title: "My title", Body: "Body", Labels: []string{"bug"}, Assignees: []string{"bob"}, Milestone: 2}
	got := hs.be.created
	if got.Title != want.Title || got.Body != want.Body || got.Milestone != want.Milestone || strings.Join(got.Labels, ",") != "bug" || strings.Join(got.Assignees, ",") != "bob" {
		t.Errorf("created %+v", got)
	}
	if hs.h.Closed[len(hs.h.Closed)-1] != createViewID || hs.h.Opened[len(hs.h.Opened)-1] != "issue/acme/widgets/77" {
		t.Errorf("closed %v opened %v", hs.h.Closed, hs.h.Opened)
	}
	if hs.h.LastNotice() != "Created #77" {
		t.Errorf("notice %q", hs.h.LastNotice())
	}
}

func TestCreateWithoutMilestoneSendsNone(t *testing.T) {
	hs := newHarness(t)
	f, _ := hs.openCreateView(t)
	f.title = "x"
	hs.i.submitCreate(f)
	hs.h.Pump(t, func() bool { return hs.i.create == nil })
	if hs.be.created.Milestone != 0 {
		t.Errorf("milestone %d", hs.be.created.Milestone)
	}
}

func TestCreateBlockedWithoutTitle(t *testing.T) {
	hs := newHarness(t)
	f, _ := hs.openCreateView(t)
	f.title = "  "
	hs.i.submitCreate(f)
	if hs.be.Count("CreateIssue") != 0 {
		t.Error("a blank title was sent")
	}
}

func TestCreateFailureKeepsForm(t *testing.T) {
	hs := newHarness(t)
	f, _ := hs.openCreateView(t)
	hs.be.with(func() { hs.be.errs["CreateIssue"] = errBoom })
	f.title = "x"
	hs.i.submitCreate(f)
	hs.h.Pump(t, func() bool { return f.busy == "" && hs.h.LastError() != "" })
	if hs.i.create != f {
		t.Error("form dropped after a failure")
	}
}

func TestCreateMetaFailureOffersRetry(t *testing.T) {
	hs := newHarness(t)
	hs.be.errs["Labels"] = errBoom
	hs.ready(t)
	hs.i.openCreate(createDraft{})
	hs.h.Pump(t, func() bool { return hs.i.meta.loaded })
	tt := viewTester(hs, createViewID)
	ghtest.MustHaveText(t, tt, "Some choices could not be loaded: Boom.", "Retry")
	hs.be.with(func() { delete(hs.be.errs, "Labels") })
	ghtest.MustClick(t, tt, "Retry")
	hs.h.Pump(t, func() bool { return hs.i.meta.loaded && hs.i.meta.err == nil })
}

func TestCreateViewWhileLoadingMeta(t *testing.T) {
	hs := newHarness(t)
	hs.ready(t)
	gate := make(chan struct{})
	hs.be.with(func() { hs.be.gate["Labels"] = gate })
	hs.i.openCreate(createDraft{})
	tt := viewTester(hs, createViewID)
	ghtest.MustHaveText(t, tt, "Loading labels, people and milestones...")
	close(gate)
	hs.h.Pump(t, func() bool { return hs.i.meta.loaded })
}

func TestOpenCreateKeepsDraftUnlessPrefilled(t *testing.T) {
	hs := newHarness(t)
	f, _ := hs.openCreateView(t)
	f.title = "draft"
	hs.i.openCreate(createDraft{})
	if hs.i.create != f || f.title != "draft" {
		t.Error("the draft was replaced")
	}
	hs.i.openCreate(createDraft{title: "from TODO", body: "b"})
	if hs.i.create == f || hs.i.create.title != "from TODO" || hs.i.create.body != "b" {
		t.Errorf("form %+v", hs.i.create)
	}
}

func TestCreateRequiresConnection(t *testing.T) {
	hs := newHarness(t)
	hs.token = github.ErrNotLoggedIn
	hs.start(t)
	hs.i.Run(ActionCreate)
	if len(hs.h.Opened) != 0 || hs.h.LastNotice() != "Sign in to GitHub" {
		t.Errorf("opened %v notices %v", hs.h.Opened, hs.h.Notices)
	}
}

func TestCreateFromTODOUsesPermalink(t *testing.T) {
	hs := newHarness(t)
	hs.ready(t)
	hs.i.CreateFromTODO(testRoot+"/src/a.go", 10, "  handle the edge case ")
	hs.h.Pump(t, func() bool { return hs.i.create != nil })
	f := hs.i.create
	if f.title != "handle the edge case" || f.body != "https://github.com/acme/widgets/blob/abc123/src/a.go#L11" {
		t.Errorf("form %+v", f)
	}
	if hs.h.Opened[len(hs.h.Opened)-1] != createViewID {
		t.Errorf("opened %v", hs.h.Opened)
	}
}

func TestCreateFromTODOFallsBackToLocation(t *testing.T) {
	hs := newHarness(t)
	hs.local.outgoing = []git.Commit{{OID: "abc123"}}
	hs.ready(t)
	hs.i.CreateFromTODO(testRoot+"/src/a.go", 0, "fix")
	hs.h.Pump(t, func() bool { return hs.i.create != nil })
	if got := hs.i.create.body; got != "Found in `src/a.go:1`." {
		t.Errorf("body %q", got)
	}
	if len(hs.h.Errs) != 1 || !strings.HasSuffix(hs.h.Errs[0], errPermalinkUnpushed.Error()) {
		t.Errorf("errors %q", hs.h.Errs)
	}
}

func TestCreateFromSelectionBody(t *testing.T) {
	hs := newHarness(t)
	hs.ready(t)
	hs.i.CreateFromSelection(testRoot+"/src/a.go", 2, 4, "x := 1")
	hs.h.Pump(t, func() bool { return hs.i.create != nil })
	if f := hs.i.create; f.title != "" || f.body != "https://github.com/acme/widgets/blob/abc123/src/a.go#L3-L5" {
		t.Errorf("form %+v", f)
	}
	hs.local.mu.Lock()
	hs.local.status.Files = []git.FileStatus{{Path: "src/a.go", Index: ' ', WorkTree: 'M'}}
	hs.local.mu.Unlock()
	hs.i.create = nil
	hs.i.CreateFromSelection(testRoot+"/src/a.go", 2, 4, "x := 1")
	hs.h.Pump(t, func() bool { return hs.i.create != nil })
	if got := hs.i.create.body; got != "Found in `src/a.go:3`.\n\n```\nx := 1\n```" {
		t.Errorf("body %q", got)
	}
}

func TestCreateFromActiveSelection(t *testing.T) {
	hs := newHarness(t)
	hs.ready(t)
	hs.i.Run(ActionCreateFromSelection)
	if hs.h.LastNotice() != "Open a file in the editor first." {
		t.Errorf("notice %q", hs.h.LastNotice())
	}
	buf := editor.NewBuffer("one\ntwo\nthree\nfour\n")
	hs.h.Path = testRoot + "/src/a.go"
	hs.h.State = &editor.State{Buffer: buf}
	hs.h.State.SetSelections(editor.Selection{Anchor: 4, Head: 14})
	hs.i.Run(ActionCreateFromSelection)
	hs.h.Pump(t, func() bool { return hs.i.create != nil })
	if got := hs.i.create.body; got != "https://github.com/acme/widgets/blob/abc123/src/a.go#L2-L3" {
		t.Errorf("body %q", got)
	}
}

func TestFencedSnippet(t *testing.T) {
	if got := fencedSnippet("a\nb\n"); got != "```\na\nb\n```" {
		t.Errorf("fence %q", got)
	}
	if got := fencedSnippet("x ``` y"); !strings.HasPrefix(got, "````\n") || !strings.HasSuffix(got, "\n````") {
		t.Errorf("fence with backticks %q", got)
	}
	long := strings.Repeat("line\n", maxSnippetLines+10)
	if got := fencedSnippet(long); !strings.Contains(got, "\n...\n") || strings.Count(got, "line") != maxSnippetLines {
		t.Errorf("long snippet %q", got)
	}
}
