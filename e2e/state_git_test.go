//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"testing"
)

const (
	gitStateFile       = "a.txt"
	commitMessageLabel = "Commit Message"
	savedCommitDraft   = "feat: keep this draft"
	changesTabLabel    = "Changes"
	unifiedLayout      = "Unified"
)

func changedStateProject(t *testing.T) string {
	t.Helper()
	root := gitProject(t, map[string]string{gitStateFile: "old\n"})
	if err := os.WriteFile(filepath.Join(root, gitStateFile), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func (q *QAApp) showSourceControl() {
	q.t.Helper()
	if _, ok := q.Find(q.Win, Label(commitMessageLabel)); !ok {
		q.Menu(q.Win, "View", "Source Control")
	}
	q.WaitNode(q.Win, "commit message", Label(commitMessageLabel))
}

func (q *QAApp) openChangesStack() {
	q.t.Helper()
	q.showSourceControl()
	row := q.WaitNode(q.Win, "changed file", func(n Node) bool { return hasName(n.Label, gitStateFile) && n.Role != "treeItem" })
	q.Click(q.Win, row, 1)
	q.WaitNode(q.Win, "stacked diff toolbar", Label("Diff Toolbar"))
}

func TestGitCommitDraftSurvivesARestart(t *testing.T) {
	t.Parallel()
	q := LaunchQA(t, changedStateProject(t), QAOptions{})
	q.showSourceControl()
	field := q.WaitNode(q.Win, "commit message", Label(commitMessageLabel))
	q.Act(q.Win, field.ID, "focus", "")
	q.Type(q.Win, savedCommitDraft)
	q.WaitNode(q.Win, "typed draft", func(n Node) bool { return n.Label == commitMessageLabel && n.Value == savedCommitDraft })
	q = q.Restart()
	q.showSourceControl()
	q.WaitNode(q.Win, "restored draft", func(n Node) bool { return n.Label == commitMessageLabel && n.Value == savedCommitDraft })
}

func TestGitWorkingTreeChangesTabIsRestored(t *testing.T) {
	t.Parallel()
	q := LaunchQA(t, changedStateProject(t), QAOptions{})
	q.openChangesStack()
	q.WaitNode(q.Win, "changes tab", RoleLabel("tab", changesTabLabel))
	q = q.Restart()
	q.WaitNode(q.Win, "restored changes tab", RoleLabel("tab", changesTabLabel))
	q.WaitNode(q.Win, "stacked diff toolbar", Label("Diff Toolbar"))
	q.WaitNode(q.Win, "file header", RoleLabel("group", gitStateFile))
}

func TestGitDiffLayoutChoiceIsRemembered(t *testing.T) {
	t.Parallel()
	q := LaunchQA(t, changedStateProject(t), QAOptions{})
	q.openChangesStack()
	q.Click(q.Win, q.WaitNode(q.Win, "unified layout", func(n Node) bool { return n.Label == unifiedLayout && n.Role != "tab" }), 1)
	q.WaitNode(q.Win, "unified chosen", func(n Node) bool { return n.Label == unifiedLayout && n.Has("checked") })
	q = q.Restart()
	q.WaitNode(q.Win, "stacked diff toolbar", Label("Diff Toolbar"))
	q.WaitNode(q.Win, "unified still chosen", func(n Node) bool { return n.Label == unifiedLayout && n.Has("checked") })
}
