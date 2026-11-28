//go:build e2e

package e2e

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ZerGo0/editor/e2e/qa"
)

const (
	githubMenu             = "GitHub"
	pullRequestsPane       = "Pull Requests"
	issuesPane             = "Issues"
	notificationsPane      = "Notifications"
	githubDemoRepoPath     = mockOwner + "/" + mockDemoRepo
	githubStressRepo       = mockOwner + "/" + mockStressRepo
	githubStressIssuesRepo = mockOwner + "/" + mockStressIssuesRepo
	githubChecksLabel      = "lint"
	githubDetailsGroup     = "Details"
	loadMoreLabel          = "Load more"
	pullRequestsList       = "Pull requests"
	pullRequestSearch      = "e"
)

var githubPanes = []string{pullRequestsPane, issuesPane, notificationsPane}

var githubDemoData = sync.OnceValue(mockDemoRepoData)

func githubPRRow() string {
	pr, _ := githubDemoData().pr(mockDetailPR)
	return fmt.Sprintf("%s, #%d by %s", pr.Title, pr.Number, pr.Author)
}

func githubIssueRow() string {
	is, _ := githubDemoData().issue(mockDetailIssue)
	return fmt.Sprintf("%s, #%d by %s", is.Title, is.Number, is.Author)
}

func githubNotificationRow() string {
	n := githubDemoData().Notifications[0]
	return fmt.Sprintf("%s, #%d, %s", n.Title, n.Number, n.Reason)
}

func githubOptions(d DemoProject, o QAOptions) QAOptions {
	o.GitHub = true
	if len(o.Tabs) == 0 {
		o.Tabs = d.TabFiles
	}
	return o
}

func openGitHubView(q *QAApp) error {
	q.Escape()
	return q.Command("Show GitHub", viewMenu, githubMenu)
}

func (q *QAApp) openGitHubDetail(row string, loaded func(qa.Node) bool) error {
	n, err := q.TryWait(RoleNamed("listItem", row), navTimeout)
	if err != nil {
		return fmt.Errorf("row %q: %w", row, err)
	}
	q.ClickNode(n)
	if _, err := q.TryWait(loaded, navTimeout); err != nil {
		return fmt.Errorf("detail of %q: %w", row, err)
	}
	return nil
}

func (q *QAApp) showGitHubPane(title string) error {
	for _, name := range githubPanes {
		pane, err := q.TryWait(func(n qa.Node) bool { return n.Role == "disclosure" && n.Label == name && n.Bounds.X == 0 }, navTimeout)
		if err != nil {
			return fmt.Errorf("pane %q: %w", name, err)
		}
		if pane.Has("expanded") != (name == title) {
			q.Act(q.Win, pane.ID, "press", "")
			q.Settle()
		}
	}
	return nil
}

func (q *QAApp) enterGitHubPane(pane string, rows ...string) error {
	if err := openGitHubView(q); err != nil {
		return err
	}
	if err := q.showGitHubPane(pane); err != nil {
		return err
	}
	for _, row := range rows {
		if _, err := q.TryWait(RoleNamed("listItem", row), navTimeout); err != nil {
			return fmt.Errorf("row %q: %w", row, err)
		}
	}
	return nil
}

func githubViewSteps() []viewStep {
	return []viewStep{
		{"github-pull-requests", func(q *QAApp, d DemoProject) error { return q.enterGitHubPane(pullRequestsPane, githubPRRow()) }},
		{"github-issues", func(q *QAApp, d DemoProject) error { return q.enterGitHubPane(issuesPane, githubIssueRow()) }},
		{"github-notifications", func(q *QAApp, d DemoProject) error {
			return q.enterGitHubPane(notificationsPane, githubNotificationRow())
		}},
		{"github-pr-detail", func(q *QAApp, d DemoProject) error {
			if err := q.enterGitHubPane(pullRequestsPane, githubPRRow()); err != nil {
				return err
			}
			return q.openGitHubDetail(githubPRRow(), RoleLabel("text", githubChecksLabel))
		}},
		{"github-issue-detail", func(q *QAApp, d DemoProject) error {
			if err := q.enterGitHubPane(issuesPane, githubIssueRow()); err != nil {
				return err
			}
			return q.openGitHubDetail(githubIssueRow(), RoleLabel("group", githubDetailsGroup))
		}},
	}
}

func allViewSteps() []viewStep {
	return append(demoViewSteps(), githubViewSteps()...)
}

func githubDemoProject(t *testing.T) DemoProject {
	t.Helper()
	return NewDemoProject(t).WithGitHubRemote(t, githubDemoRepoPath)
}

func requireNoUnhandled(t *testing.T) {
	t.Helper()
	if miss := sharedGitHubMock().Unhandled(); len(miss) > 0 {
		t.Errorf("the app called routes the GitHub mock does not serve:\n%s", strings.Join(miss, "\n"))
	}
}

func TestGitHubViews(t *testing.T) {
	t.Parallel()
	demo := githubDemoProject(t)
	q := LaunchQA(t, demo.Root, githubOptions(demo, QAOptions{Width: normalWindowW, Height: normalWindowH, SidebarWidth: defaultSidebar, Theme: themeDark}))
	for _, step := range githubViewSteps() {
		if err := step.Enter(q, demo); err != nil {
			t.Fatalf("%s: %v", step.Name, err)
		}
		q.Settle()
	}
	if _, err := q.TryWait(RoleLabel("tab", detailTabTitle(mockDetailPR)), navTimeout); err != nil {
		t.Errorf("pull request tab: %v", err)
	}
	requireNoUnhandled(t)
}

func detailTabTitle(number int) string {
	r := githubDemoData()
	if pr, ok := r.pr(number); ok {
		return fmt.Sprintf("#%d %s", number, pr.Title)
	}
	is, _ := r.issue(number)
	return fmt.Sprintf("#%d %s", number, is.Title)
}

func (q *QAApp) scrollToRow(label string, match func(qa.Node) bool, timeout time.Duration) (qa.Node, error) {
	deadline := time.Now().Add(timeout)
	for {
		tree := q.Snapshot()
		if n, ok := tree.Find(match); ok {
			return n, nil
		}
		if list, ok := tree.Find(RoleLabel("list", label)); ok {
			q.Send(map[string]any{"type": "wheel", "x": list.Bounds.CenterX(), "y": list.Bounds.CenterY(), "dy": scrollToTop})
		}
		if time.Now().After(deadline) {
			return qa.Node{}, fmt.Errorf("not found within %s", timeout)
		}
		time.Sleep(navPoll)
	}
}

func TestGitHubPullRequestPaging(t *testing.T) {
	t.Parallel()
	demo := githubDemoProject(t)
	q := LaunchQA(t, demo.Root, githubOptions(demo, QAOptions{Width: normalWindowW, Height: normalWindowH, SidebarWidth: defaultSidebar, Theme: themeDark}))
	if err := openGitHubView(q); err != nil {
		t.Fatal(err)
	}
	search, err := q.TryWait(RoleLabel("textField", "Search pull requests"), navTimeout)
	if err != nil {
		t.Fatal(err)
	}
	q.FocusNode(search)
	q.Send(TextEvent(pullRequestSearch), KeyEvent("Enter"))
	q.Settle()
	more, err := q.scrollToRow(pullRequestsList, RoleLabel("listItem", loadMoreLabel), navTimeout)
	if err != nil {
		t.Fatalf("first page of the search: %v", err)
	}
	q.Act(q.Win, more.ID, "press", "")
	q.Settle()
	last, _ := githubDemoData().pr(mockDemoIssues + mockDemoPRs)
	if _, err := q.TryWait(func(n qa.Node) bool {
		return n.Role == "listItem" && strings.Contains(n.Label, fmt.Sprintf("#%d by", last.Number))
	}, navTimeout); err != nil {
		t.Errorf("second page of the search: %v", err)
	}
	requireNoUnhandled(t)
}

func TestGitHubLargePullRequest(t *testing.T) {
	t.Parallel()
	demo := NewDemoProject(t).WithGitHubRemote(t, githubStressRepo)
	q := LaunchQA(t, demo.Root, githubOptions(demo, QAOptions{Width: normalWindowW, Height: normalWindowH, SidebarWidth: defaultSidebar, Theme: themeDark}))
	if err := openGitHubView(q); err != nil {
		t.Fatal(err)
	}
	pr, _ := mockStressRepoData().pr(mockStressBigPR)
	row := fmt.Sprintf("%s, #%d by %s", pr.Title, pr.Number, pr.Author)
	changed := fmt.Sprintf("across %d files", mockStressFiles)
	if err := q.openGitHubDetail(row, func(n qa.Node) bool { return n.Role == "text" && strings.Contains(n.Label, changed) }); err != nil {
		t.Fatal(err)
	}
	requireNoUnhandled(t)
}
