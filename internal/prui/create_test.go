package prui

import (
	"testing"

	"github.com/ZerGo0/editor/internal/ghshared/ghtest"
	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/github"
	"github.com/egoist/mygo/ui"
)

func (hs *harness) openCreateView(t *testing.T) (*createForm, *ui.Tester) {
	t.Helper()
	hs.be.current = nil
	hs.ready(t)
	hs.p.openCreate()
	f := hs.p.createForm
	hs.h.Pump(t, func() bool { return !f.loading })
	return f, viewTester(hs, createViewID)
}

func TestCreateFormDefaults(t *testing.T) {
	hs := newHarness(t)
	f, tt := hs.openCreateView(t)
	if f.title != "Subject of feature" || f.base != "main" {
		t.Errorf("title %q base %q", f.title, f.base)
	}
	if len(f.bases) != 2 || f.bases[0] != "main" || f.bases[1] != "develop" {
		t.Errorf("bases %v", f.bases)
	}
	ghtest.MustHaveText(t, tt, "Create Pull Request", "feature", "into", "Title", "Description", "Create as draft", "Create")
	if a, ok := tt.AccessByRole(ui.RoleButton, "Create"); !ok || a.Disabled {
		t.Errorf("create button %+v %v", a, ok)
	}
}

func TestCreateSubmitsAndOpensPR(t *testing.T) {
	hs := newHarness(t)
	f, tt := hs.openCreateView(t)
	f.title, f.body, f.draft = "  My title ", "Body", true
	tt.Frame()
	ghtest.MustClick(t, tt, "Create")
	hs.h.Pump(t, func() bool { return hs.p.createForm == nil })
	want := github.CreateOptions{Title: "My title", Body: "Body", Head: "feature", Base: "main", Draft: true}
	if hs.be.created != want {
		t.Errorf("created %+v", hs.be.created)
	}
	if len(hs.local.pushes) != 0 {
		t.Errorf("pushed although the branch has an upstream: %v", hs.local.pushes)
	}
	if hs.h.Closed[len(hs.h.Closed)-1] != createViewID || hs.h.Opened[len(hs.h.Opened)-1] != "pr/acme/widgets/77" {
		t.Errorf("closed %v opened %v", hs.h.Closed, hs.h.Opened)
	}
	if hs.p.current.pr == nil || hs.p.current.pr.Number != 77 {
		t.Error("current branch PR not updated")
	}
}

func TestCreatePushesBranchWithoutUpstream(t *testing.T) {
	hs := newHarness(t)
	hs.local.setCurrent("topic", "")
	f, tt := hs.openCreateView(t)
	ghtest.MustHaveText(t, tt, "This branch has no upstream. It will be pushed to origin first.")
	_ = f
	ghtest.MustClick(t, tt, "Create")
	hs.h.Pump(t, func() bool { return hs.p.createForm == nil })
	want := git.PushOptions{Remote: "origin", Branch: "topic", SetUpstream: true}
	if len(hs.local.pushes) != 1 || hs.local.pushes[0].Remote != want.Remote || hs.local.pushes[0].Branch != want.Branch || !hs.local.pushes[0].SetUpstream {
		t.Errorf("pushes %+v", hs.local.pushes)
	}
}

func TestCreateFromForkUsesOwnerPrefix(t *testing.T) {
	hs := newHarness(t)
	hs.local.remotes = append(hs.local.remotes, git.Remote{Name: "fork", FetchURL: "git@github.com:zed/widgets.git"})
	hs.local.setCurrent("topic", "fork/topic")
	hs.local.branches = append(hs.local.branches, git.Branch{Name: "fork/topic", Remote: true})
	hs.openCreateView(t)
	hs.p.submitCreate(hs.p.createForm)
	hs.h.Pump(t, func() bool { return hs.p.createForm == nil })
	if hs.be.created.Head != "zed:topic" {
		t.Errorf("head %q", hs.be.created.Head)
	}
}

func TestCreateBlockedStates(t *testing.T) {
	hs := newHarness(t)
	f, tt := hs.openCreateView(t)
	f.title = " "
	tt.Frame()
	ghtest.MustHaveText(t, tt, "Enter a title.")
	if a, ok := tt.AccessByRole(ui.RoleButton, "Create"); !ok || !a.Disabled {
		t.Errorf("create button %+v %v", a, ok)
	}
	f.title, f.base = "x", "feature"
	if got := f.blocked(); got == "" {
		t.Error("base equal to head must block")
	}
	f.base = ""
	if got := f.blocked(); got != "Choose a base branch." {
		t.Errorf("blocked %q", got)
	}
}

func TestCreateFailureShowsError(t *testing.T) {
	hs := newHarness(t)
	hs.be.errs["Create"] = &github.APIError{Status: 422, Message: "A pull request already exists"}
	f, _ := hs.openCreateView(t)
	hs.p.submitCreate(f)
	hs.h.Pump(t, func() bool { return f.busy == "" && hs.h.LastError() != "" })
	if hs.p.createForm != f {
		t.Error("form dropped after a failure")
	}
}

func TestCreateWithoutBranchNotifies(t *testing.T) {
	hs := newHarness(t)
	hs.local.branches = nil
	hs.ready(t)
	hs.p.openCreate()
	if len(hs.h.Opened) != 0 || len(hs.h.Notices) != 1 {
		t.Errorf("opened %v notices %v", hs.h.Opened, hs.h.Notices)
	}
}

func TestCreateBranchChangeResetsForm(t *testing.T) {
	hs := newHarness(t)
	f, _ := hs.openCreateView(t)
	hs.local.setCurrent("another", "origin/another")
	hs.p.BranchChanged()
	hs.h.Pump(t, func() bool { return hs.p.createForm != nil && hs.p.createForm != f && !hs.p.createForm.loading })
	if hs.p.createForm.title != "Subject of another" {
		t.Errorf("title %q", hs.p.createForm.title)
	}
}

func TestCreateBasesFailureOffersRetry(t *testing.T) {
	hs := newHarness(t)
	hs.be.current = nil
	hs.ready(t)
	hs.p.openCreate()
	f := hs.p.createForm
	hs.local.mu.Lock()
	hs.local.branches = []git.Branch{{Name: "feature", Current: true}}
	hs.local.mu.Unlock()
	hs.p.loadCreateBases(f)
	hs.h.Pump(t, func() bool { return !f.loading })
	tt := viewTester(hs, createViewID)
	ghtest.MustHaveText(t, tt, "No remote branches found. Fetch the remote, then retry.", "Retry")
}
