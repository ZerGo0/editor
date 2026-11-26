package prui

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/github"
	"github.com/egoist/mygo/ui"
)

const (
	sideWidth  = 360
	sideHeight = 640
)

func sideTester(p *PRs) *ui.Tester {
	return ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() { p.SideView(c) })
		p.Overlay(c)
	}, sideWidth, sideHeight)
}

func mustHaveText(t *testing.T, tt *ui.Tester, texts ...string) {
	t.Helper()
	for _, s := range texts {
		if !tt.HasText(s) {
			t.Errorf("missing text %q in %q", s, tt.Texts())
		}
	}
}

func mustClick(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	if err := tt.Click(label); err != nil {
		t.Fatalf("click %q: %v (texts %q)", label, err, tt.Texts())
	}
}

func TestStateMessages(t *testing.T) {
	cases := []struct {
		name  string
		setup func(hs *harness)
		texts []string
	}{
		{"no repo", func(hs *harness) { hs.open = fmt.Errorf("%w: /x", git.ErrNotRepo) }, []string{"No Git repository", "Open a folder that is a Git repository"}},
		{"no remote", func(hs *harness) { hs.local.remotes = nil }, []string{"No GitHub remote", "git remote add origin"}},
		{"no gh", func(hs *harness) { hs.token = github.ErrNoGH }, []string{"GitHub CLI not found", "Install the GitHub CLI and run gh auth login"}},
		{"not logged in", func(hs *harness) { hs.token = github.ErrNotLoggedIn }, []string{"Sign in to GitHub with gh auth login", "Copy Command"}},
		{"git error", func(hs *harness) { hs.open = errors.New("boom") }, []string{"Cannot load pull requests", "boom", "Retry"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hs := newHarness(t)
			tc.setup(hs)
			hs.start(t)
			tt := sideTester(hs.p)
			mustHaveText(t, tt, tc.texts...)
		})
	}
}

func TestConnectingShowsSpinner(t *testing.T) {
	hs := newHarness(t)
	hs.p = NewWith(hs.h, hs.env)
	t.Cleanup(hs.p.Close)
	tt := sideTester(hs.p)
	mustHaveText(t, tt, "Connecting to GitHub...")
	if _, ok := tt.AccessByRole(ui.RoleProgress, "Connecting to GitHub..."); !ok {
		t.Error("the spinner has no accessible name")
	}
}

func TestRetryReconnects(t *testing.T) {
	hs := newHarness(t)
	hs.token = github.ErrNotLoggedIn
	hs.start(t)
	tt := sideTester(hs.p)
	hs.token = nil
	mustClick(t, tt, "Check Again")
	hs.h.pump(t, func() bool { return hs.p.phase == phaseReady })
	tt.Frame()
	mustHaveText(t, tt, "Current Branch")
}

func TestUnauthorizedResponseShowsSignIn(t *testing.T) {
	hs := newHarness(t)
	hs.be.errs["List:review"] = &github.APIError{Status: 401, Message: "Bad credentials"}
	hs.ready(t)
	tt := sideTester(hs.p)
	hs.h.pump(t, func() bool { return hs.p.phase == phaseNotLoggedIn })
	tt.Frame()
	mustHaveText(t, tt, "Sign in to GitHub with gh auth login")
}

func TestSideViewSections(t *testing.T) {
	hs := newHarness(t)
	hs.ready(t)
	tt := sideTester(hs.p)
	hs.h.pump(t, func() bool { return hs.p.side.review.loaded })
	tt.Frame()
	mustHaveText(t, tt, "Current Branch", "feature", "Waiting for My Review", "Assigned to Me", "Created by Me", "All Open", "Fix the widget", "#12", "alice", "3h ago", "Review me please", "#31")
	if got := hs.be.called("List:assigned") + hs.be.called("List:created") + hs.be.called("List:all"); got != 0 {
		t.Errorf("collapsed sections loaded %d times", got)
	}
	mustClick(t, tt, "All Open")
	hs.h.pump(t, func() bool { return hs.p.side.all.loaded })
	tt.Frame()
	mustHaveText(t, tt, "Everything open")
	if hs.be.called("List:all") != 1 {
		t.Errorf("all open loaded %d times", hs.be.called("List:all"))
	}
	tt.Advance(time.Second)
	mustClick(t, tt, "All Open")
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
	if !strings.Contains(strings.Join(hs.be.calls, ","), "Current:zed:topic-remote") {
		t.Errorf("calls %v", hs.be.calls)
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
	hs.h.pump(t, func() bool { return hs.p.side.review.loaded })
	tt.Frame()
	mustHaveText(t, tt, "Draft", "Draft work")
	if a, ok := tt.Access("Checks failed"); !ok || a.Name != "Checks failed" {
		t.Errorf("check icon not named: %+v %v", a, ok)
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
	hs.h.pump(t, func() bool { return hs.p.side.review.loaded })
	tt.Frame()
	if !hs.p.side.review.more {
		t.Fatal("a full page must offer more")
	}
	hs.p.side.review.items = hs.p.side.review.items[:3]
	hs.p.side.current.open = false
	tt.Frame()
	mustClick(t, tt, "Load more")
	hs.h.pump(t, func() bool { return !hs.p.side.review.loading })
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
	hs.h.pump(t, func() bool { return hs.p.side.review.err != nil })
	tt.Frame()
	mustHaveText(t, tt, "connection reset", "Retry")
	hs.be.clearErr("List:review")
	mustClick(t, tt, "Retry")
	hs.h.pump(t, func() bool { return hs.p.side.review.loaded })
	tt.Frame()
	mustHaveText(t, tt, "Review me please")
}

func TestEmptySectionSaysWhatToDo(t *testing.T) {
	hs := newHarness(t)
	hs.be.lists["review"] = nil
	hs.ready(t)
	tt := sideTester(hs.p)
	hs.h.pump(t, func() bool { return hs.p.side.review.loaded })
	tt.Frame()
	mustHaveText(t, tt, "No pull requests. Press refresh to check again.")
}

func TestCreateRowWhenBranchHasNoPR(t *testing.T) {
	hs := newHarness(t)
	hs.be.current = nil
	hs.ready(t)
	tt := sideTester(hs.p)
	tt.Frame()
	mustHaveText(t, tt, "No pull request for feature.")
	mustClick(t, tt, "Create Pull Request")
	if len(hs.h.opened) != 1 || hs.h.opened[0] != createViewID {
		t.Errorf("opened %v", hs.h.opened)
	}
}

func TestDetachedHeadHasNoCreateButton(t *testing.T) {
	hs := newHarness(t)
	hs.local.branches = []git.Branch{{Name: "origin/main", Remote: true}}
	hs.ready(t)
	tt := sideTester(hs.p)
	mustHaveText(t, tt, "HEAD is detached. Check out a branch to create a pull request.")
	if tt.HasText("Create Pull Request") {
		t.Error("detached HEAD offers to create a pull request")
	}
}

func TestSearchSection(t *testing.T) {
	hs := newHarness(t)
	hs.be.lists["search:is:open label:bug"] = []github.PullRequest{testPR(9, "Bug hunt")}
	hs.ready(t)
	tt := sideTester(hs.p)
	mustClick(t, tt, "Search pull requests")
	tt.Type("is:open label:bug")
	tt.Key(0, ui.KeyEnter)
	hs.h.pump(t, func() bool { return hs.p.side.search != nil && hs.p.side.search.loaded })
	tt.Frame()
	mustHaveText(t, tt, "Search Results", "Bug hunt")
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
	hs.h.pump(t, func() bool { return hs.p.side.review.loaded })
	mustClick(t, tt, "Refresh")
	hs.h.pump(t, func() bool { return hs.be.called("List:review") == 2 && !hs.p.side.review.loading })
	if hs.be.called("List:assigned") != 0 {
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
	hs.h.pump(t, func() bool { return hs.p.current.pr != nil && hs.p.current.pr.Number == 55 })
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
	hs.h.pump(t, func() bool { return hs.be.called("Checks") >= 3 })
	hs.h.drain()
	if hs.p.current.loading {
		t.Error("loading flag never cleared")
	}
}

func TestStatusItems(t *testing.T) {
	hs := newHarness(t)
	hs.ready(t)
	hs.p.checks[12] = checkSuccess
	tt := ui.NewTester(func(c *ui.Context) { ui.Row(c).Children(func() { hs.p.StatusItems(c) }) }, sideWidth, 40)
	mustHaveText(t, tt, "#12")
	if _, ok := tt.Access("Pull request #12, Checks passed"); !ok {
		t.Error("status item has no accessible name")
	}
	mustClick(t, tt, "Pull request #12, Checks passed")
	if len(hs.h.opened) != 1 || hs.h.opened[0] != "pr/acme/widgets/12" {
		t.Errorf("opened %v", hs.h.opened)
	}
}

func TestStatusItemsHiddenWithoutPR(t *testing.T) {
	hs := newHarness(t)
	hs.be.current = nil
	hs.ready(t)
	tt := ui.NewTester(func(c *ui.Context) { ui.Row(c).Children(func() { hs.p.StatusItems(c) }) }, sideWidth, 40)
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
	if !hs.p.Run(ActionOpenCurrent) || len(hs.h.opened) != 1 {
		t.Errorf("open current: %v", hs.h.opened)
	}
	hs.p.Run(ActionCreate)
	hs.p.Run(ActionCheckout)
	hs.p.Run(ActionSearch)
	if len(hs.h.asks) != 2 {
		t.Fatalf("asks %d", len(hs.h.asks))
	}
	if err := hs.h.asks[0].submit("abc"); err == nil {
		t.Error("a bad number was accepted")
	}
	if err := hs.h.asks[0].submit("#42"); err != nil {
		t.Fatal(err)
	}
	hs.h.pump(t, func() bool { return len(hs.h.notices) > 0 })
	hs.mu.Lock()
	co := append([]int(nil), hs.co...)
	hs.mu.Unlock()
	if len(co) != 1 || co[0] != 42 {
		t.Errorf("checkout calls %v", co)
	}
	if err := hs.h.asks[1].submit("is:open"); err != nil || hs.p.side.search == nil {
		t.Errorf("search not started: %v", err)
	}
	before := hs.be.called("Current:acme:feature")
	hs.p.Run(ActionRefresh)
	hs.h.pump(t, func() bool { return hs.be.called("Current:acme:feature") > before })
}

func TestRunActionsBeforeReadyReportState(t *testing.T) {
	hs := newHarness(t)
	hs.token = github.ErrNoGH
	hs.start(t)
	hs.p.Run(ActionCheckout)
	hs.p.Run(ActionOpenCurrent)
	if len(hs.h.asks) != 0 || len(hs.h.notices) != 2 {
		t.Errorf("asks %d notices %q", len(hs.h.asks), hs.h.notices)
	}
}

func TestAutoRefreshOnlyWhenVisibleAndDue(t *testing.T) {
	hs := newHarness(t)
	hs.ready(t)
	now := testNow
	hs.p.env.Now = func() time.Time { return now }
	hs.p.refreshed = now
	tt := sideTester(hs.p)
	hs.h.drain()
	base := hs.be.called("Current:acme:feature")
	now = now.Add(autoRefreshInterval - time.Second)
	tt.Frame()
	hs.h.drain()
	if hs.be.called("Current:acme:feature") != base {
		t.Error("refreshed before the interval")
	}
	now = now.Add(2 * time.Second)
	tt.Frame()
	hs.h.pump(t, func() bool { return hs.be.called("Current:acme:feature") > base })
}

func TestSideListEnterActivatesSelectedRow(t *testing.T) {
	hs := newHarness(t)
	hs.ready(t)
	tt := sideTester(hs.p)
	hs.h.pump(t, func() bool { return hs.p.side.review.loaded })
	tt.Frame()
	mustClick(t, tt, "Review me please")
	before := len(hs.h.opened)
	tt.Key(0, ui.KeyEnter)
	if len(hs.h.opened) != before+1 || hs.h.opened[before] != "pr/acme/widgets/31" {
		t.Errorf("opened %v", hs.h.opened)
	}
}

func TestHeaderDoubleClickTogglesOnceAndEnterToggles(t *testing.T) {
	hs := newHarness(t)
	hs.ready(t)
	tt := sideTester(hs.p)
	mustClick(t, tt, "Assigned to Me")
	mustClick(t, tt, "Assigned to Me")
	if !hs.p.side.assigned.open {
		t.Error("a double click toggled the section twice")
	}
	tt.Advance(2 * time.Second)
	tt.Key(0, ui.KeyEnter)
	if hs.p.side.assigned.open {
		t.Error("Enter did not toggle the selected header")
	}
}
