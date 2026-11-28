//go:build e2e

package e2e

import "testing"

const matchCaseLabel = "Match Case"

func matchCaseOn(n Node) bool { return n.Label == matchCaseLabel && n.Has("checked") }

func openFind(q *QAApp, name string) {
	q.t.Helper()
	openInEditor(q.t, q.App, q.Win, name)
	q.Key(q.Win, "Cmd+F")
	q.WaitNode(q.Win, "find field in "+name, RoleLabel("textField", findFieldLabel))
}

func TestFindMatchCaseIsSharedAndSurvivesRestart(t *testing.T) {
	t.Parallel()
	q := LaunchQA(t, Project(t, map[string]string{"a.txt": "Cat cat\n", "b.txt": "Cat cat\n"}), QAOptions{})
	openFind(q, "a.txt")
	q.Key(q.Win, "Cmd+Alt+C")
	q.WaitNode(q.Win, "Match Case on in a.txt", matchCaseOn)
	q.Key(q.Win, "Escape")
	openFind(q, "b.txt")
	q.WaitNode(q.Win, "Match Case on in b.txt", matchCaseOn)
	q = q.Restart()
	openFind(q, "b.txt")
	q.WaitNode(q.Win, "Match Case on after a restart", matchCaseOn)
}
