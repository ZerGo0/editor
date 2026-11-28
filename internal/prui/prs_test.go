package prui

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ZerGo0/editor/internal/ghshared"
	"github.com/ZerGo0/editor/internal/ghshared/ghtest"
	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/github"
	"github.com/egoist/mygo/ui"
)

func sideTester(p *PRs) *ui.Tester {
	return ghtest.SideTester(p.SideActions, p.SideView, p.Overlay)
}

func TestConnectionErrorShowsRetry(t *testing.T) {
	hs := newHarness(t)
	hs.open = errors.New("boom")
	hs.start(t)
	ghtest.MustHaveText(t, sideTester(hs.p), "Cannot load pull requests", "Boom.", "Retry")
}

func TestRetryReconnects(t *testing.T) {
	hs := newHarness(t)
	hs.token = github.ErrNotLoggedIn
	hs.start(t)
	tt := sideTester(hs.p)
	hs.token = nil
	ghtest.MustClick(t, tt, "Check Again")
	hs.h.Pump(t, func() bool { return hs.p.Phase == ghshared.PhaseReady })
	tt.Frame()
	ghtest.MustHaveText(t, tt, "Current Branch")
}

func TestUnauthorizedResponseShowsSignIn(t *testing.T) {
	hs := newHarness(t)
	hs.be.errs["List:review"] = &github.APIError{Status: 401, Message: "Bad credentials"}
	hs.ready(t)
	tt := sideTester(hs.p)
	hs.h.Pump(t, func() bool { return hs.p.Phase == ghshared.PhaseNotLoggedIn })
	tt.Frame()
	ghtest.MustHaveText(t, tt, "Sign in to GitHub")
}

func TestSideViewSections(t *testing.T) {
	hs := newHarness(t)
	hs.ready(t)
	tt := sideTester(hs.p)
	hs.h.Pump(t, func() bool { return hs.p.side.review.loaded })
	tt.Frame()
	ghtest.MustHaveText(t, tt, "Current Branch", "feature", "Waiting for My Review", "Assigned to Me", "Created by Me", "All Open", "Fix the widget", "#12", "alice", "3h ago", "Review me please", "#31")
	if got := hs.be.Count("List:assigned") + hs.be.Count("List:created") + hs.be.Count("List:all"); got != 0 {
		t.Errorf("collapsed sections loaded %d times", got)
	}
	ghtest.MustClick(t, tt, "All Open")
	hs.h.Pump(t, func() bool { return hs.p.side.all.loaded })
	tt.Frame()
	ghtest.MustHaveText(t, tt, "Everything open")
	if hs.be.Count("List:all") != 1 {
		t.Errorf("all open loaded %d times", hs.be.Count("List:all"))
	}
	tt.Advance(time.Second)
	ghtest.MustClick(t, tt, "All Open")
	if tt.HasText("Everything open") {
		t.Error("collapsing the section kept its rows")
	}
}

func TestHeadOwnerFollowsUpstreamRemote(t *testing.T) {
	hs := newHarness(t)
	hs.local.remotes = append(hs.local.remotes, git.Remote{Name: "fork", FetchURL: "git@github.com:zed/widgets.git"})
	hs.local.setCurrent("topic", "fork/topic-remote")
	hs.ready(t)
	hs.be.mu.Lock()
	defer hs.be.mu.Unlock()
	if !strings.Contains(strings.Join(hs.be.Names(), ","), "Current:zed:topic-remote") {
		t.Errorf("calls %v", hs.be.Names())
	}
}

func TestPRRowShowsDraftAndChecks(t *testing.T) {
	hs := newHarness(t)
	draft := testPR(31, "Draft work")
	draft.Draft = true
	hs.be.lists["review"] = []github.PullRequest{draft}
	hs.ready(t)
	hs.p.checks[31] = checkFailure
	tt := sideTester(hs.p)
	hs.h.Pump(t, func() bool { return hs.p.side.review.loaded })
	tt.Frame()
	ghtest.MustHaveText(t, tt, "Draft", "Draft work")
	if _, ok := tt.Access("Draft work, #31 by alice, Checks failed"); !ok {
		t.Error("the check state is not part of the row name")
	}
}

func TestLoadMorePagesSection(t *testing.T) {
	hs := newHarness(t)
	first := make([]github.PullRequest, pageSize)
	for i := range first {
		first[i] = testPR(100+i, fmt.Sprintf("PR number %d", 100+i))
	}
	hs.be.lists["review"] = first
	hs.be.listPages = map[int][]github.PullRequest{2: {testPR(500, "Second page row")}}
	hs.ready(t)
	tt := sideTester(hs.p)
	hs.h.Pump(t, func() bool { return hs.p.side.review.loaded })
	tt.Frame()
	if !hs.p.side.review.more {
		t.Fatal("a full page must offer more")
	}
	hs.p.side.review.items = hs.p.side.review.items[:3]
	hs.p.side.current.open = false
	tt.Frame()
	ghtest.MustClick(t, tt, "Load more")
	hs.h.Pump(t, func() bool { return !hs.p.side.review.loading })
	tt.Frame()
	if n := len(hs.p.side.review.items); n != 4 {
		t.Errorf("items after paging: %d", n)
	}
}

func TestSectionErrorHasRetry(t *testing.T) {
	hs := newHarness(t)
	hs.be.errs["List:review"] = errors.New("github: connection reset")
	hs.ready(t)
	tt := sideTester(hs.p)
	hs.h.Pump(t, func() bool { return hs.p.side.review.err != nil })
	tt.Frame()
	ghtest.MustHaveText(t, tt, "Connection reset.", "Retry")
	hs.be.clearErr("List:review")
	ghtest.MustClick(t, tt, "Retry")
	hs.h.Pump(t, func() bool { return hs.p.side.review.loaded })
	tt.Frame()
	ghtest.MustHaveText(t, tt, "Review me please")
}

func TestEmptySectionSaysWhatToDo(t *testing.T) {
	hs := newHarness(t)
	hs.be.lists["review"] = nil
	hs.ready(t)
	tt := sideTester(hs.p)
	hs.h.Pump(t, func() bool { return hs.p.side.review.loaded })
	tt.Frame()
	ghtest.MustHaveText(t, tt, "No pull requests")
}

func TestCreateRowWhenBranchHasNoPR(t *testing.T) {
	hs := newHarness(t)
	hs.be.current = nil
	hs.ready(t)
	tt := sideTester(hs.p)
	tt.Frame()
	ghtest.MustHaveText(t, tt, "No pull request for feature.")
	ghtest.MustClick(t, tt, "Create Pull Request")
	if len(hs.h.Opened) != 1 || hs.h.Opened[0] != createViewID {
		t.Errorf("opened %v", hs.h.Opened)
	}
}

func TestDetachedHeadHasNoCreateButton(t *testing.T) {
	hs := newHarness(t)
	hs.local.branches = []git.Branch{{Name: "origin/main", Remote: true}}
	hs.ready(t)
	tt := sideTester(hs.p)
	ghtest.MustHaveText(t, tt, "HEAD is detached. Check out a branch to create a pull request.")
	if tt.HasText("Create Pull Request") {
		t.Error("detached HEAD offers to create a pull request")
	}
}

func TestSearchSection(t *testing.T) {
	hs := newHarness(t)
	hs.be.lists["search:is:open label:bug"] = []github.PullRequest{testPR(9, "Bug hunt")}
	hs.ready(t)
	tt := sideTester(hs.p)
	ghtest.MustClick(t, tt, "Search pull requests")
	tt.Type("is:open label:bug")
	tt.Key(0, ui.KeyEnter)
	hs.h.Pump(t, func() bool { return hs.p.side.search != nil && hs.p.side.search.loaded })
	tt.Frame()
	ghtest.MustHaveText(t, tt, "Search Results", "Bug hunt")
	hs.p.runSearch("")
	tt.Frame()
	if tt.HasText("Search Results") {
		t.Error("clearing the search kept its section")
	}
}

func TestRefreshMarksStaleAndReloadsOpenSections(t *testing.T) {
	hs := newHarness(t)
	hs.ready(t)
	tt := sideTester(hs.p)
	hs.h.Pump(t, func() bool { return hs.p.side.review.loaded })
	ghtest.MustClick(t, tt, "Refresh")
	hs.h.Pump(t, func() bool { return hs.be.Count("List:review") == 2 && !hs.p.side.review.loading })
	if hs.be.Count("List:assigned") != 0 {
		t.Error("refresh loaded a collapsed section")
	}
}

func TestBranchChangedRefreshesCurrent(t *testing.T) {
	hs := newHarness(t)
	hs.ready(t)
	hs.local.setCurrent("other", "origin/other")
	other := testPR(55, "Other branch work")
	hs.be.mu.Lock()
	hs.be.current = &other
	hs.be.mu.Unlock()
	hs.p.BranchChanged()
	hs.h.Pump(t, func() bool { return hs.p.current.pr != nil && hs.p.current.pr.Number == 55 })
	if hs.p.branch.Name != "other" {
		t.Errorf("branch %q", hs.p.branch.Name)
	}
}

func TestStaleCurrentResultIgnored(t *testing.T) {
	hs := newHarness(t)
	hs.ready(t)
	hs.p.refreshCurrent()
	gen := hs.p.current.gen
	hs.p.refreshCurrent()
	if hs.p.current.gen == gen {
		t.Fatal("generation did not advance")
	}
	hs.h.Pump(t, func() bool { return hs.be.Count("Checks") >= 3 })
	hs.h.Drain()
	if hs.p.current.loading {
		t.Error("loading flag never cleared")
	}
}

func TestStatusItems(t *testing.T) {
	hs := newHarness(t)
	hs.ready(t)
	hs.p.checks[12] = checkSuccess
	tt := ghtest.StatusTester(hs.p.StatusItems)
	ghtest.MustHaveText(t, tt, "#12")
	if _, ok := tt.Access("Pull request #12, Checks passed"); !ok {
		t.Error("status item has no accessible name")
	}
	ghtest.MustClick(t, tt, "Pull request #12, Checks passed")
	if len(hs.h.Opened) != 1 || hs.h.Opened[0] != "pr/acme/widgets/12" {
		t.Errorf("opened %v", hs.h.Opened)
	}
}

func TestStatusItemsHiddenWithoutPR(t *testing.T) {
	hs := newHarness(t)
	hs.be.current = nil
	hs.ready(t)
	tt := ghtest.StatusTester(hs.p.StatusItems)
	if len(tt.Texts()) != 0 {
		t.Errorf("status item drawn: %q", tt.Texts())
	}
}

func TestRunActions(t *testing.T) {
	hs := newHarness(t)
	hs.ready(t)
	for _, a := range Actions {
		if !strings.HasPrefix(a.ID, "pr.") || a.Title == "" || a.Category == "" {
			t.Errorf("bad action %+v", a)
		}
	}
	if hs.p.Run("other.thing") {
		t.Error("unknown action handled")
	}
	if !hs.p.Run(ActionOpenCurrent) || len(hs.h.Opened) != 1 {
		t.Errorf("open current: %v", hs.h.Opened)
	}
	hs.p.Run(ActionCreate)
	hs.p.Run(ActionCheckout)
	hs.p.Run(ActionSearch)
	if len(hs.h.Asks) != 2 {
		t.Fatalf("asks %d", len(hs.h.Asks))
	}
	if err := hs.h.Asks[0].Submit("abc"); err == nil {
		t.Error("a bad number was accepted")
	}
	if err := hs.h.Asks[0].Submit("#42"); err != nil {
		t.Fatal(err)
	}
	hs.h.Pump(t, func() bool { return len(hs.h.Notices) > 0 })
	hs.mu.Lock()
	co := append([]int(nil), hs.co...)
	hs.mu.Unlock()
	if len(co) != 1 || co[0] != 42 {
		t.Errorf("checkout calls %v", co)
	}
	if err := hs.h.Asks[1].Submit("is:open"); err != nil || hs.p.side.search == nil {
		t.Errorf("search not started: %v", err)
	}
	before := hs.be.Count("Current:acme:feature")
	hs.p.Run(ActionRefresh)
	hs.h.Pump(t, func() bool { return hs.be.Count("Current:acme:feature") > before })
}

func TestRunActionsBeforeReadyReportState(t *testing.T) {
	hs := newHarness(t)
	hs.token = github.ErrNoGH
	hs.start(t)
	hs.p.Run(ActionCheckout)
	hs.p.Run(ActionOpenCurrent)
	if len(hs.h.Asks) != 0 || len(hs.h.Notices) != 2 {
		t.Errorf("asks %d notices %q", len(hs.h.Asks), hs.h.Notices)
	}
}

func TestAutoRefreshOnlyWhenVisibleAndDue(t *testing.T) {
	hs := newHarness(t)
	hs.ready(t)
	now := testNow
	hs.p.env.Now = func() time.Time { return now }
	hs.p.refreshed = now
	tt := sideTester(hs.p)
	hs.h.Drain()
	base := hs.be.Count("Current:acme:feature")
	now = now.Add(autoRefreshInterval - time.Second)
	tt.Frame()
	hs.h.Drain()
	if hs.be.Count("Current:acme:feature") != base {
		t.Error("refreshed before the interval")
	}
	now = now.Add(2 * time.Second)
	tt.Frame()
	hs.h.Pump(t, func() bool { return hs.be.Count("Current:acme:feature") > base })
}

func TestSideListEnterActivatesSelectedRow(t *testing.T) {
	hs := newHarness(t)
	hs.ready(t)
	tt := sideTester(hs.p)
	hs.h.Pump(t, func() bool { return hs.p.side.review.loaded })
	tt.Frame()
	ghtest.MustClick(t, tt, "Review me please")
	before := len(hs.h.Opened)
	tt.Key(0, ui.KeyEnter)
	if len(hs.h.Opened) != before+1 || hs.h.Opened[before] != "pr/acme/widgets/31" {
		t.Errorf("opened %v", hs.h.Opened)
	}
}

func TestHeaderDoubleClickTogglesOnceAndEnterToggles(t *testing.T) {
	hs := newHarness(t)
	hs.ready(t)
	tt := sideTester(hs.p)
	ghtest.MustClick(t, tt, "Assigned to Me")
	ghtest.MustClick(t, tt, "Assigned to Me")
	if !hs.p.side.assigned.open {
		t.Error("a double click toggled the section twice")
	}
	tt.Advance(2 * time.Second)
	tt.Key(0, ui.KeyEnter)
	if hs.p.side.assigned.open {
		t.Error("Enter did not toggle the selected header")
	}
}

func TestStateViewGatesTheWholeGitHubView(t *testing.T) {
	hs := newHarness(t)
	hs.token = github.ErrNotLoggedIn
	hs.start(t)
	tt := ui.NewTester(func(c *ui.Context) {
		if hs.p.StateView(c) {
			return
		}
		ui.Text(c, "sections")
	}, ghtest.SideWidth, ghtest.SideHeight)
	ghtest.MustHaveText(t, tt, "Sign in to GitHub", "Copy Command")
	if tt.HasText("sections") {
		t.Error("sections drawn behind the view-level state")
	}
	ready := newHarness(t)
	ready.ready(t)
	rt := ui.NewTester(func(c *ui.Context) {
		if ready.p.StateView(c) {
			return
		}
		ui.Text(c, "sections")
	}, ghtest.SideWidth, ghtest.SideHeight)
	ghtest.MustHaveText(t, rt, "sections")
}

func TestHeaderRowsOpenAndCloseWithArrowKeys(t *testing.T) {
	hs := newHarness(t)
	hs.ready(t)
	tt := sideTester(hs.p)
	hs.h.Pump(t, func() bool { return hs.p.side.review.loaded })
	tt.Frame()
	ghtest.MustClick(t, tt, "Assigned to Me")
	if !hs.p.side.assigned.open {
		t.Fatal("click did not open the section")
	}
	tt.Key(0, ui.KeyLeft)
	if hs.p.side.assigned.open {
		t.Error("Left did not close the section")
	}
	tt.Key(0, ui.KeyRight)
	if !hs.p.side.assigned.open {
		t.Error("Right did not open the section")
	}
}
