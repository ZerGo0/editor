package issueui

import (
	"fmt"
	"testing"
	"time"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/github"
	"github.com/egoist/mygo/ui"
)

const (
	sideWidth  = 360
	sideHeight = 640

	queryAssigned = "is:open assignee:me repo:acme/widgets"
	queryCreatedX = "author:me state:open repo:acme/widgets sort:created-desc"
	queryRecentX  = "state:open repo:acme/widgets sort:updated-desc"
)

func sideTester(i *Issues) *ui.Tester {
	return ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() { i.SideView(c) })
	}, sideWidth, sideHeight)
}

func milestoneIssue(number int, title, milestone string) github.Issue {
	issue := testIssue(number, title)
	if milestone != "" {
		issue.Milestone = &github.Milestone{Number: number, Title: milestone}
	}
	return issue
}

func (hs *harness) seedSide() {
	hs.be.pages[queryAssigned] = []github.IssuePage{{
		Total: 3,
		Issues: []github.Issue{
			milestoneIssue(5, "Crash on start", "v1"),
			milestoneIssue(6, "No milestone yet", ""),
			milestoneIssue(7, "Second in v1", "v1"),
		},
	}}
	hs.be.pages[queryCreatedX] = []github.IssuePage{{Total: 1, Issues: []github.Issue{testIssue(8, "Created by me")}}}
	hs.be.pages[queryRecentX] = []github.IssuePage{{Total: 1, Issues: []github.Issue{testIssue(9, "Recent one")}}}
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
		{"not logged in", func(hs *harness) { hs.token = github.ErrNotLoggedIn }, []string{"Sign in to GitHub", "gh auth login", "Copy Command"}},
		{"git error", func(hs *harness) { hs.open = errBoom }, []string{"Cannot connect to GitHub", "boom", "Retry"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hs := newHarness(t)
			tc.setup(hs)
			hs.start(t)
			mustHaveText(t, sideTester(hs.i), tc.texts...)
		})
	}
}

func TestConnectingShowsSpinner(t *testing.T) {
	hs := newHarness(t)
	hs.i = NewWith(hs.h, hs.env)
	t.Cleanup(hs.i.Close)
	tt := sideTester(hs.i)
	mustHaveText(t, tt, "Connecting to GitHub...")
	if _, ok := tt.AccessByRole(ui.RoleProgress, "Connecting to GitHub..."); !ok {
		t.Error("the spinner has no accessible name")
	}
}

func TestRetryReconnects(t *testing.T) {
	hs := newHarness(t)
	hs.token = github.ErrNotLoggedIn
	hs.start(t)
	tt := sideTester(hs.i)
	hs.token = nil
	mustClick(t, tt, "Check Again")
	hs.h.pump(t, func() bool { return hs.i.phase == phaseReady })
	tt.Frame()
	mustHaveText(t, tt, "My Issues")
}

func TestUnauthorizedViewerShowsSignIn(t *testing.T) {
	hs := newHarness(t)
	hs.be.errs["Viewer"] = &github.APIError{Status: 401, Message: "Bad credentials"}
	hs.start(t)
	if hs.i.phase != phaseNotLoggedIn {
		t.Fatalf("phase %d", hs.i.phase)
	}
	mustHaveText(t, sideTester(hs.i), "Sign in to GitHub")
}

func TestUnauthorizedSearchShowsSignIn(t *testing.T) {
	hs := newHarness(t)
	hs.be.errs["Search"] = &github.APIError{Status: 401, Message: "Bad credentials"}
	hs.ready(t)
	tt := sideTester(hs.i)
	hs.h.pump(t, func() bool { return hs.i.phase == phaseNotLoggedIn })
	tt.Frame()
	mustHaveText(t, tt, "Sign in to GitHub")
}

func TestSideViewDefaultQueriesAndMilestoneGroups(t *testing.T) {
	hs := newHarness(t)
	hs.seedSide()
	hs.ready(t)
	tt := sideTester(hs.i)
	hs.h.pump(t, func() bool { return hs.i.side.sections[0].loaded })
	tt.Frame()
	mustHaveText(t, tt, "My Issues", "Created Issues", "Recent Issues", "v1", "No Milestone", "Crash on start", "#5", "alice", "3h ago", "No milestone yet", "Second in v1")
	for _, name := range []string{queryCreatedX, queryRecentX} {
		for _, q := range hs.be.searches {
			if q == name {
				t.Errorf("collapsed section %q was loaded", name)
			}
		}
	}
	mustClick(t, tt, "Created Issues")
	hs.h.pump(t, func() bool { return hs.i.side.sections[1].loaded })
	tt.Frame()
	mustHaveText(t, tt, "Created by me")
	tt.Advance(time.Second)
	mustClick(t, tt, "Created Issues")
	if tt.HasText("Created by me") {
		t.Error("collapsing the section kept its rows")
	}
}

func TestSectionOrderGroupsByMilestone(t *testing.T) {
	groups := groupByMilestoneTitle([]github.Issue{
		milestoneIssue(1, "a", "v2"),
		milestoneIssue(2, "b", ""),
		milestoneIssue(3, "c", "v1"),
		milestoneIssue(4, "d", "v2"),
	})
	if len(groups) != 3 || groups[0].title != "v2" || len(groups[0].items) != 2 || groups[1].title != "v1" || groups[2].title != noMilestoneTitle {
		t.Errorf("groups %+v", groups)
	}
	if len(groupByMilestoneTitle(nil)) != 0 {
		t.Error("empty input must give no groups")
	}
}

func TestLoadMoreAppendsNextPage(t *testing.T) {
	hs := newHarness(t)
	hs.be.pages[queryAssigned] = []github.IssuePage{
		{Total: 3, HasMore: true, Issues: []github.Issue{testIssue(1, "First")}},
		{Total: 3, Issues: []github.Issue{testIssue(2, "Second"), testIssue(1, "First")}},
	}
	hs.i = NewWith(hs.h, hs.env)
	t.Cleanup(hs.i.Close)
	hs.h.pump(t, func() bool { return hs.i.phase == phaseReady })
	hs.i.SetSettings(Settings{Queries: []Query{{Label: "Mine", Query: queryAssigned}}})
	tt := sideTester(hs.i)
	hs.h.pump(t, func() bool { return hs.i.side.sections[0].loaded })
	tt.Frame()
	mustHaveText(t, tt, "Mine", "First", "Load more")
	mustClick(t, tt, "Load more")
	hs.h.pump(t, func() bool { return len(hs.i.side.sections[0].items) == 2 })
	tt.Frame()
	mustHaveText(t, tt, "Second")
	if tt.HasText("Load more") {
		t.Error("Load more stays after the last page")
	}
	if len(hs.i.side.sections[0].items) != 2 {
		t.Errorf("items %d: duplicates were not removed", len(hs.i.side.sections[0].items))
	}
}

func TestSectionErrorOffersRetry(t *testing.T) {
	hs := newHarness(t)
	hs.be.errs["Search"] = &github.RateLimitError{Status: 403, Message: "slow"}
	hs.ready(t)
	tt := sideTester(hs.i)
	hs.h.pump(t, func() bool { return hs.i.side.sections[0].err != nil })
	tt.Frame()
	mustHaveText(t, tt, "GitHub rate limit reached. Try again later.", "Retry")
	hs.be.with(func() { delete(hs.be.errs, "Search") })
	hs.be.with(hs.seedSide)
	mustClick(t, tt, "Retry")
	hs.h.pump(t, func() bool { return hs.i.side.sections[0].loaded })
	tt.Frame()
	mustHaveText(t, tt, "Crash on start")
}

func TestEmptySectionSaysWhatToDo(t *testing.T) {
	hs := newHarness(t)
	hs.ready(t)
	tt := sideTester(hs.i)
	hs.h.pump(t, func() bool { return hs.i.side.sections[0].loaded })
	tt.Frame()
	mustHaveText(t, tt, "No issues found. Refresh to check again.")
}

func TestStaleSectionResultIsIgnored(t *testing.T) {
	hs := newHarness(t)
	hs.seedSide()
	hs.ready(t)
	s := hs.i.side.sections[0]
	gate := make(chan struct{})
	hs.be.with(func() { hs.be.gate["Search"] = gate })
	hs.i.loadSection(s, false)
	staleGen := s.gen
	hs.be.with(func() {
		delete(hs.be.gate, "Search")
		hs.be.pages[queryAssigned] = []github.IssuePage{{Total: 1, Issues: []github.Issue{testIssue(50, "Newer result")}}}
	})
	hs.i.loadSection(s, false)
	hs.h.pump(t, func() bool { return s.loaded })
	close(gate)
	hs.h.drain()
	if s.gen == staleGen || len(s.items) != 1 || s.items[0].Number != 50 {
		t.Errorf("items %+v", s.items)
	}
}

func TestSearchRunsRepositoryQuery(t *testing.T) {
	hs := newHarness(t)
	hs.seedSide()
	hs.be.pages["repo:acme/widgets crash"] = []github.IssuePage{{Total: 1, Issues: []github.Issue{testIssue(31, "Crash in parser")}}}
	hs.be.pages["repo:other/thing label:bug"] = []github.IssuePage{{Total: 1, Issues: []github.Issue{testIssue(32, "Elsewhere")}}}
	hs.ready(t)
	tt := sideTester(hs.i)
	mustClick(t, tt, "Search issues")
	tt.Type("crash")
	tt.Key(0, ui.KeyEnter)
	hs.h.pump(t, func() bool { return hs.i.side.search != nil && hs.i.side.search.loaded })
	tt.Frame()
	mustHaveText(t, tt, "Search Results", "Crash in parser")
	hs.i.runSearch("repo:other/thing label:bug")
	hs.h.pump(t, func() bool {
		return hs.i.side.search.loaded && len(hs.i.side.search.items) == 1 && hs.i.side.search.items[0].Number == 32
	})
	hs.i.runSearch("  ")
	if hs.i.side.search != nil {
		t.Error("an empty search must close the results")
	}
}

func TestClickingIssueOpensDetail(t *testing.T) {
	hs := newHarness(t)
	hs.seedSide()
	hs.be.issues[5] = testIssue(5, "Crash on start")
	hs.ready(t)
	tt := sideTester(hs.i)
	hs.h.pump(t, func() bool { return hs.i.side.sections[0].loaded })
	tt.Frame()
	mustClick(t, tt, "Crash on start, #5 by alice")
	if len(hs.h.opened) != 1 || hs.h.opened[0] != "issue/acme/widgets/5" {
		t.Errorf("opened %v", hs.h.opened)
	}
	if hs.h.titles["issue/acme/widgets/5"] != "#5 Crash on start" {
		t.Errorf("title %q", hs.h.titles["issue/acme/widgets/5"])
	}
}

func TestRefreshReloadsOpenSections(t *testing.T) {
	hs := newHarness(t)
	hs.seedSide()
	hs.ready(t)
	tt := sideTester(hs.i)
	hs.h.pump(t, func() bool { return hs.i.side.sections[0].loaded })
	tt.Frame()
	before := hs.be.called("Search")
	mustClick(t, tt, "Refresh Issues")
	tt.Frame()
	hs.h.pump(t, func() bool { return hs.be.called("Search") > before && !hs.i.side.sections[0].loading })
	if !hs.i.Run(ActionRefresh) {
		t.Error("refresh action not handled")
	}
}

func TestSideViewAccessibility(t *testing.T) {
	hs := newHarness(t)
	hs.seedSide()
	hs.ready(t)
	tt := sideTester(hs.i)
	hs.h.pump(t, func() bool { return hs.i.side.sections[0].loaded })
	tt.Frame()
	for _, a := range tt.AccessAll() {
		if a.Role == ui.RoleButton && a.Name == "" {
			t.Errorf("a button at %v has no name", a.Bounds)
		}
	}
	for _, name := range []string{"Search issues", "Create Issue", "Refresh Issues"} {
		if _, ok := tt.Access(name); !ok {
			t.Errorf("no accessible element %q", name)
		}
	}
}

func TestTitleAndBadge(t *testing.T) {
	hs := newHarness(t)
	hs.seedSide()
	hs.start(t)
	if hs.i.Title() != "Issues" || hs.i.Badge() != 0 {
		t.Errorf("title %q badge %d before load", hs.i.Title(), hs.i.Badge())
	}
	tt := sideTester(hs.i)
	hs.h.pump(t, func() bool { return hs.i.side.sections[0].loaded })
	tt.Frame()
	if hs.i.Badge() != 3 {
		t.Errorf("badge %d", hs.i.Badge())
	}
}

func TestSetSettingsReplacesQueries(t *testing.T) {
	hs := newHarness(t)
	hs.ready(t)
	hs.i.SetSettings(Settings{Queries: []Query{{Label: "Bugs", Query: "label:bug repo:${owner}/${repository}"}}})
	tt := sideTester(hs.i)
	mustHaveText(t, tt, "Bugs")
	if tt.HasText("My Issues") {
		t.Error("old queries remain")
	}
	hs.h.pump(t, func() bool { return hs.i.side.sections[0].loaded })
	if got := hs.be.searches[len(hs.be.searches)-1]; got != "label:bug repo:acme/widgets" {
		t.Errorf("query %q", got)
	}
}
