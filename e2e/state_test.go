//go:build e2e

package e2e

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/ZerGo0/editor/e2e/qa"
	"github.com/ZerGo0/editor/internal/session"
)

const (
	draggedSidebar   = 330.0
	panelFromRight   = 480.0
	widthTolerancePx = 3.0
	reopenLine       = 51
	longFileLines    = 120
)

func near(a, b float64) bool { return math.Abs(a-b) <= widthTolerancePx }

func regionWidth(q *QAApp, label string) float64 {
	n, ok := q.Snapshot().Find(func(n qa.Node) bool { return n.Role == "region" && n.Label == label })
	if !ok {
		return -1
	}
	return n.Bounds.Width
}

func (q *QAApp) dragSplitterNamed(label string, path ...float64) {
	q.t.Helper()
	s := q.WaitFor(label, func(n qa.Node) bool { return n.Role == "splitter" && n.Label == label })
	x, y := s.Bounds.CenterX(), s.Bounds.CenterY()
	q.Send(map[string]any{"type": "pointerMove", "x": x, "y": y})
	q.Send(map[string]any{"type": "pointerDown", "x": x, "y": y})
	for _, toX := range path {
		for i := 1; i <= dragSteps; i++ {
			q.Send(map[string]any{"type": "pointerMove", "x": x + (toX-x)*float64(i)/dragSteps, "y": y})
		}
		q.Settle()
		x = toX
	}
	q.Send(map[string]any{"type": "pointerUp", "x": x, "y": y})
	q.Settle()
}

func (q *QAApp) clickButton(label string) {
	q.t.Helper()
	q.ClickNode(q.WaitFor(label, func(n qa.Node) bool { return n.Role == "button" && n.Label == label }))
}

func TestLayoutChangesSurviveACrash(t *testing.T) {
	t.Parallel()
	root := Project(t, map[string]string{"a.txt": "a\n"})
	q := LaunchQA(t, root, QAOptions{Width: normalWindowW, Height: normalWindowH, SidebarWidth: defaultSidebar})
	q.DragSplitter(draggedSidebar)
	want := regionWidth(q, sidebarRegion)
	if near(want, defaultSidebar) {
		t.Fatalf("the drag did not resize the side bar: %.0f", want)
	}
	q.WaitSaved("the new side bar width", func(p session.Project) bool { return near(float64(p.SidebarWidth), want) })
	q = q.CrashAndRelaunch()
	if got := regionWidth(q, sidebarRegion); !near(got, want) {
		t.Fatalf("after a crash the side bar is %.0f px wide, want %.0f", got, want)
	}
}

func TestPanelOnTheRightKeepsItsWidthAndMaximizedStateAfterARestart(t *testing.T) {
	t.Parallel()
	root := Project(t, map[string]string{"a.txt": "a\n"})
	q := LaunchQA(t, root, QAOptions{Width: normalWindowW, Height: normalWindowH, SidebarWidth: defaultSidebar, PanelOpen: true, Panel: "problems"})
	q.clickButton("Move Panel Right")
	q.dragSplitterNamed("Resize Panel", normalWindowW-panelFromRight)
	want := regionWidth(q, panelRegion)
	if want < 0 {
		t.Fatal("no panel on the right")
	}
	q.clickButton("Maximize Panel Size")
	q = q.Restart()
	q.clickButton("Restore Panel Size")
	if got := regionWidth(q, panelRegion); !near(got, want) {
		t.Fatalf("after a restart the panel is %.0f px wide, want %.0f", got, want)
	}
}

func TestAClosedFileOpensAgainWhereItWasAfterARestart(t *testing.T) {
	t.Parallel()
	root := Project(t, map[string]string{"a.txt": strings.Repeat("line\n", longFileLines)})
	q := LaunchQA(t, root, QAOptions{Width: normalWindowW, Height: normalWindowH, SidebarWidth: defaultSidebar})
	openInEditor(t, q.App, q.Win, "a.txt")
	q.Key(q.Win, "Ctrl+G")
	q.WaitNode(q.Win, "go to line", RoleLabel("dialog", goToLineTitle))
	q.Type(q.Win, fmt.Sprint(reopenLine))
	q.Key(q.Win, "Enter")
	at := fmt.Sprintf("Ln %d, Col 1", reopenLine)
	q.WaitNode(q.Win, "caret on line 51", func(n Node) bool { return strings.HasPrefix(n.Label, at) })
	q.Key(q.Win, "Cmd+W")
	q.Eventually("the editor to close", func() bool {
		_, open := q.Find(q.Win, codeEditor("a.txt"))
		return !open
	})
	q = q.Restart()
	openInEditor(t, q.App, q.Win, "a.txt")
	q.WaitNode(q.Win, "caret back on line 51", func(n Node) bool { return strings.HasPrefix(n.Label, at) })
}
