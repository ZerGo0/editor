//go:build e2e

package e2e

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ZerGo0/editor/e2e/qa"
)

const (
	stressGitHubLoadMoreMax = 40
	stressGitHubMoreWait    = 2 * time.Second
	stressGitHubScroll      = 2000
	stressGitHubReadme      = "README.md"
	pullRequestsSearchLabel = "Search pull requests"
	stressGitHubRemoteBase  = "https://github.com/"
	stressGitHubRemoteExt   = ".git"
)

func stressGitHubProjectOf(repo string) func(t *testing.T) DemoProject {
	return func(t *testing.T) DemoProject {
		t.Helper()
		root, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		g := newDemoGit(t, root)
		g.run("init", "-q", "-b", "main")
		g.commitFile(stressGitHubReadme, "# GitHub stress\n", "chore: initial commit")
		g.run("remote", "add", demoRemoteName, stressGitHubRemoteBase+repo+stressGitHubRemoteExt)
		return DemoProject{Root: root}
	}
}

func stressGitHubOptions(d DemoProject) QAOptions {
	return QAOptions{Width: normalWindowW, Height: normalWindowH, SidebarWidth: defaultSidebar, Theme: themeDark, FrameStats: true, QuietTimeout: stressQuietTimeout, WaitTimeout: stressWaitTimeout, GitHub: true}
}

func (a latencyAction) onGitHubRepo(repo, name string, quiet time.Duration, samples int) latencyAction {
	a.Options = stressGitHubOptions
	return a.onProject(stressGitHubProjectOf(repo), name, quiet, samples)
}

func (a latencyAction) onGitHub(name string, quiet time.Duration, samples int) latencyAction {
	return a.onGitHubRepo(githubStressRepo, name, quiet, samples)
}

func rowContaining(text string) func(n qa.Node) bool {
	return func(n qa.Node) bool { return n.Role == "listItem" && strings.Contains(n.Label, text) }
}

func loadAllRows(q *QAApp, searchLabel, query, list, last string) error {
	search, err := q.TryWait(RoleLabel("textField", searchLabel), q.waitFor())
	if err != nil {
		return err
	}
	q.FocusNode(search)
	q.Send(TextEvent(query), KeyEvent("Enter"))
	q.Settle()
	for range stressGitHubLoadMoreMax {
		more, err := q.scrollToRow(list, RoleLabel("listItem", loadMoreLabel), stressGitHubMoreWait)
		if err != nil {
			break
		}
		q.Act(q.Win, more.ID, "press", "")
		q.Settle()
	}
	if _, err := q.TryWait(rowContaining(last), q.waitFor()); err != nil {
		return fmt.Errorf("the row %q of the %s list did not load: %w", last, list, err)
	}
	return nil
}

func loadAllPullRequests(q *QAApp) error {
	return loadAllRows(q, pullRequestsSearchLabel, pullRequestSearch, pullRequestsList, fmt.Sprintf("#%d by", mockStressPRs))
}

func scrollGitHubListAction(pane, list string, dy int, load func(q *QAApp) error) latencyAction {
	k := &actionKit{}
	return latencyAction{
		Owner: qa.OwnerGitHub,
		Prepare: func(q *QAApp, d DemoProject) error {
			if err := q.enterGitHubPane(pane); err != nil {
				return err
			}
			if err := load(q); err != nil {
				return err
			}
			var err error
			if k.target, err = q.TryWait(RoleLabel("list", list), q.waitFor()); err != nil {
				return err
			}
			k.scrollToTop(q, d, 0)
			q.WaitCPUQuiet()
			return nil
		},
		Do: k.wheel(dy),
	}
}

func openLargePullRequestAction() latencyAction {
	k := &actionKit{}
	return latencyAction{
		Owner: qa.OwnerGitHub,
		Prepare: func(q *QAApp, d DemoProject) (err error) {
			pr, _ := mockStressRepoData().pr(mockStressBigPR)
			row := fmt.Sprintf("%s, #%d by %s", pr.Title, pr.Number, pr.Author)
			if err = q.enterGitHubPane(pullRequestsPane, row); err != nil {
				return err
			}
			k.target, err = q.TryWait(RoleNamed("listItem", row), q.waitFor())
			return err
		},
		Reset: closeEditorAfterFirstRun,
		Do:    func(q *QAApp, d DemoProject, i int) { q.Act(q.Win, k.target.ID, "press", "") },
	}
}

func stressGitHubActions() []latencyAction {
	return append(stressIssueActions(), []latencyAction{
		scrollGitHubListAction(pullRequestsPane, pullRequestsList, stressGitHubScroll, loadAllPullRequests).onGitHub(fmt.Sprintf("scroll the pull request list (%d PRs)", mockStressPRs), stressQuietLong, stressSamples),
		openLargePullRequestAction().onGitHub(fmt.Sprintf("open a pull request with %d changed files", mockStressFiles), stressQuietLong, stressBigSamples).cold(),
	}...)
}
