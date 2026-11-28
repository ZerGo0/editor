//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ZerGo0/editor/e2e/qa"
)

const (
	checkoutMenuItem     = "Checkout to..."
	stressBranchSwitches = 4
	stressBranchMaxWait  = 60 * time.Second
	stressBranchTimeout  = 5 * time.Minute
	headRefPrefix        = "ref: refs/heads/"
	branchPoll           = 50 * time.Millisecond
)

func stressBranchProject(t *testing.T) DemoProject {
	t.Helper()
	root := filepath.Join(t.TempDir(), "branches")
	if out, err := exec.Command("cp", "-cR", stressRoot(stressFixtureBranches), root).CombinedOutput(); err != nil {
		t.Fatalf("copy the branches fixture: %v\n%s", err, out)
	}
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	return DemoProject{Root: root}
}

func branchTarget(i int) string {
	if i%2 == 0 {
		return stressBranchWide
	}
	return stressBranchMain
}

func currentBranch(root string) string {
	data, _ := os.ReadFile(filepath.Join(root, ".git", "HEAD"))
	return strings.TrimPrefix(strings.TrimSpace(string(data)), headRefPrefix)
}

func waitBranch(root, want string) error {
	for deadline := time.Now().Add(stressBranchMaxWait); time.Now().Before(deadline); time.Sleep(branchPoll) {
		if currentBranch(root) == want {
			return nil
		}
	}
	return fmt.Errorf("the branch is %q, want %q after %s", currentBranch(root), want, stressBranchMaxWait)
}

func switchBranchAction() latencyAction {
	return latencyAction{
		Owner: qa.OwnerGit, IdleFrames: blinkFramesPerSec,
		Reset: func(q *QAApp, d DemoProject, i int) {
			if i > 0 {
				if err := waitBranch(d.Root, branchTarget(i-1)); err != nil {
					q.t.Fatal(err)
				}
			}
			q.WaitCPUQuiet()
			if err := q.TryMenu(gitMenu, checkoutMenuItem); err != nil {
				q.t.Fatal(err)
			}
			q.Send(TextEvent(branchTarget(i)))
			q.Settle()
			if _, err := q.TryWait(RoleLabel("listItem", branchTarget(i)), q.waitFor()); err != nil {
				q.t.Fatalf("branch %s: %v", branchTarget(i), err)
			}
			q.Send(KeyEvent("Enter"))
			if _, err := q.TryWait(RoleLabel("listItem", "Checkout"), q.waitFor()); err != nil {
				q.t.Fatalf("checkout action: %v", err)
			}
		},
		Do: func(q *QAApp, d DemoProject, i int) { q.Send(KeyEvent("Enter")) },
		Verify: func(q *QAApp, d DemoProject) error {
			return waitBranch(d.Root, branchTarget(stressBranchSwitches-1))
		},
	}
}

func stressGitActions() []latencyAction {
	a := switchBranchAction().onProject(stressBranchProject, fmt.Sprintf("switch branch with %dk changed files (%dk commits)", stressBranchFiles/1000, stressBranchCommits/1000), stressQuietLong, stressBranchSwitches).cold()
	a.MaxWait, a.Timeout = stressBranchMaxWait, stressBranchTimeout
	return []latencyAction{a}
}
