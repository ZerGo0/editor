//go:build e2e

package e2e

import (
	"math"
	"strings"
	"testing"
)

const (
	savedSettingsQuery = "folding"
	historySplitDrag   = 120
	historyTabLabel    = "History: a.txt"
	historyWidthTolPx  = 1.0
)

func TestSettingsSearchSurvivesRestart(t *testing.T) {
	t.Parallel()
	q := LaunchQA(t, Project(t, map[string]string{"a.txt": "alpha\n"}), QAOptions{})
	q.Menu(q.Win, "Editor", "Settings")
	search := q.WaitNode(q.Win, "settings search", RoleLabel("textField", settingsSearchField))
	q.Act(q.Win, search.ID, "focus", "")
	q.Type(q.Win, savedSettingsQuery)
	waitFieldValue(q.App, q.Win, settingsSearchField, savedSettingsQuery)
	q = q.Restart()
	q.WaitNode(q.Win, "settings tab", RoleLabel("tab", "Settings"))
	waitFieldValue(q.App, q.Win, settingsSearchField, savedSettingsQuery)
}

func showHistory(q *QAApp) Node {
	q.t.Helper()
	q.Menu(q.Win, "View", "Command Palette...")
	q.WaitNode(q.Win, "palette", RoleLabel("dialog", "Command Palette"))
	q.Type(q.Win, "show history")
	q.WaitNode(q.Win, "history command", func(n Node) bool { return strings.HasSuffix(n.Label, "Show History") })
	q.Key(q.Win, "Enter")
	q.WaitNode(q.Win, "history tab", RoleLabel("tab", historyTabLabel))
	return q.WaitNode(q.Win, "versions list", RoleLabel("list", historyListLabel))
}

func historySplitter(q *QAApp, list Node) Node {
	q.t.Helper()
	return q.WaitNode(q.Win, "history splitter", func(n Node) bool {
		return n.Role == "splitter" && n.Bounds.Height > n.Bounds.Width && math.Abs(n.Bounds.X-(list.Bounds.X+list.Bounds.Width)) <= list.Bounds.Width
	})
}

func TestLocalHistorySplitSurvivesRestart(t *testing.T) {
	t.Parallel()
	q := LaunchQA(t, Project(t, map[string]string{"a.txt": "alpha\n"}), QAOptions{Width: normalWindowW, Height: normalWindowH})
	q.Click(q.Win, q.WaitNode(q.Win, "a.txt row", RoleLabel("treeItem", "a.txt")), 2)
	field := q.WaitNode(q.Win, "editor", codeEditor("a.txt"))
	q.Act(q.Win, field.ID, "focus", "")
	q.Type(q.Win, "one ")
	q.Menu(q.Win, "File", "Save")
	list := showHistory(q)
	splitter := historySplitter(q, list)
	before := list.Bounds.Width
	x, y := splitter.Bounds.X+splitter.Bounds.Width/2, splitter.Bounds.Y+splitter.Bounds.Height/2
	q.Send(map[string]any{"type": "pointerMove", "x": x, "y": y})
	q.Send(map[string]any{"type": "pointerDown", "x": x, "y": y})
	for i := 1; i <= dragSteps; i++ {
		q.Send(map[string]any{"type": "pointerMove", "x": x + historySplitDrag*float64(i)/dragSteps, "y": y})
	}
	q.Send(map[string]any{"type": "pointerUp", "x": x + historySplitDrag, "y": y})
	q.Settle()
	var dragged float64
	q.Eventually("versions list wider", func() bool {
		n, ok := q.Find(q.Win, RoleLabel("list", historyListLabel))
		dragged = n.Bounds.Width
		return ok && dragged > before+historyWidthTolPx
	})
	q = q.Restart()
	q.WaitNode(q.Win, "a.txt tab", RoleLabel("tab", "a.txt"))
	q.Click(q.Win, q.WaitNode(q.Win, "a.txt tab", RoleLabel("tab", "a.txt")), 1)
	if got := showHistory(q).Bounds.Width; math.Abs(got-dragged) > historyWidthTolPx {
		t.Fatalf("versions list width %.0f after restart, want %.0f", got, dragged)
	}
}
