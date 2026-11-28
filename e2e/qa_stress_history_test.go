//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZerGo0/editor/e2e/qa"
	"github.com/ZerGo0/editor/internal/history"
)

const (
	stressHistoryVersions = 500
	stressHistoryPlain    = 200
	stressHistoryFile     = "history.txt"
	stressHistoryLines    = 200
	historyDataDir        = "data"
	historyStoreDir       = "history"
	historyCommand        = "Show History"
	historyCommandRow     = "History: Show History"
	historyTabPrefix      = "History: "
	historyListLabel      = "Versions"
	historyStepKey        = "Down"
)

func stressHistoryProject(t *testing.T) DemoProject {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, stressHistoryFile), []byte(stressHistoryText(stressHistoryVersions)), shotFilePerm); err != nil {
		t.Fatal(err)
	}
	return DemoProject{Root: root, BigFile: filepath.Join(root, stressHistoryFile)}
}

func stressHistoryText(version int) string {
	var b strings.Builder
	for line := range stressHistoryLines {
		fmt.Fprintf(&b, "line %d of version %d: the quick brown fox jumps over the lazy dog\n", line, version)
	}
	return b.String()
}

func seedStressHistory(t *testing.T, root, userData string) {
	t.Helper()
	store, err := history.Open(filepath.Join(userData, historyDataDir, historyStoreDir), history.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, stressHistoryFile)
	for v := range stressHistoryVersions {
		label := ""
		if v >= stressHistoryPlain {
			label = fmt.Sprintf("Label %d", v)
		}
		if err := store.Record(root, path, []byte(stressHistoryText(v)), label); err != nil {
			t.Fatal(err)
		}
	}
	if versions, err := store.Versions(root, path); err != nil || len(versions) != stressHistoryVersions {
		t.Fatalf("seeded %d history versions, want %d: %v", len(versions), stressHistoryVersions, err)
	}
}

func stressHistoryOptions(d DemoProject) QAOptions {
	o := stressOptions(func(d DemoProject) []string { return []string{d.BigFile} })(d)
	o.Seed = seedStressHistory
	return o
}

func queueShowHistory(q *QAApp) error {
	q.Send(KeyEvent(paletteKey))
	if _, err := q.TryWait(RoleLabel(paletteRole, commandPaletteLabel), q.waitFor()); err != nil {
		return err
	}
	q.Send(TextEvent(historyCommand))
	q.Settle()
	_, err := q.TryWait(RoleLabel("listItem", historyCommandRow), q.waitFor())
	return err
}

func expectHistoryTab(q *QAApp, d DemoProject) error {
	if _, err := q.TryWait(RoleLabel("tab", historyTabPrefix+stressHistoryFile), q.waitFor()); err != nil {
		return err
	}
	list, err := q.TryWait(RoleLabel("list", historyListLabel), q.waitFor())
	if err != nil {
		return err
	}
	_, err = q.TryWait(func(n qa.Node) bool { return n.Role == "listItem" && n.Parent == list.ID }, q.waitFor())
	return err
}

func openHistoryAction() latencyAction {
	return latencyAction{
		Owner: qa.OwnerHistory, IdleFrames: blinkFramesPerSec,
		Reset: func(q *QAApp, d DemoProject, i int) {
			closeEditorAfterFirstRun(q, d, i)
			if err := queueShowHistory(q); err != nil {
				q.t.Fatalf("command palette: %v", err)
			}
		},
		Do:     func(q *QAApp, d DemoProject, i int) { q.Send(KeyEvent("Enter")) },
		Verify: expectHistoryTab,
	}
}

func selectHistoryAction() latencyAction {
	k := &actionKit{}
	return latencyAction{
		Owner: qa.OwnerHistory, IdleFrames: blinkFramesPerSec,
		Prepare: func(q *QAApp, d DemoProject) (err error) {
			if err = queueShowHistory(q); err != nil {
				return err
			}
			q.Send(KeyEvent("Enter"))
			if err = expectHistoryTab(q, d); err != nil {
				return err
			}
			k.target, err = q.TryWait(RoleLabel("list", historyListLabel), q.waitFor())
			if err != nil {
				return err
			}
			first, err := q.TryWait(func(n qa.Node) bool { return n.Role == "listItem" && n.Parent == k.target.ID }, q.waitFor())
			if err != nil {
				return err
			}
			q.ClickNode(first)
			return nil
		},
		Do:     func(q *QAApp, d DemoProject, i int) { q.Send(KeyEvent(historyStepKey)) },
		Verify: expectHistoryTab,
	}
}

func stressHistoryActions() []latencyAction {
	versions := fmt.Sprintf("%d versions", stressHistoryVersions)
	history := func(a latencyAction, name string, samples int) latencyAction {
		return a.onProject(stressHistoryProject, name, stressQuietLong, samples).withOptions(stressHistoryOptions)
	}
	return []latencyAction{
		history(openHistoryAction(), "open the history of a file ("+versions+")", stressBigSamples),
		history(selectHistoryAction(), "select versions in the history of a file ("+versions+")", stressSamples),
	}
}
