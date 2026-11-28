//go:build e2e

package e2e

import (
	"fmt"
	"testing"

	"github.com/ZerGo0/editor/e2e/qa"
	"github.com/ZerGo0/editor/internal/design/token"
)

const (
	dragSteps     = 6
	sidebarTolPx  = 1.0
	splitterLabel = "Resize Primary Side Bar"
)

func sidebarWidth(t *qa.Tree) float64 {
	region, ok := t.Find(func(n qa.Node) bool { return n.Role == "region" && n.Label == sidebarRegion })
	if !ok {
		return -1
	}
	return region.Bounds.Width
}

func widthText(w float64) string { return fmt.Sprintf("%.0fpx", w) }

func (q *QAApp) DragSplitter(toX float64) {
	q.t.Helper()
	q.dragSplitterNamed(splitterLabel, toX)
}

func TestSidebarWidthStaysWithinLimits(t *testing.T) {
	t.Parallel()
	demo := NewDemoProject(t)
	min := float64(token.SidebarMinWidth.Px())
	q := LaunchQA(t, demo.Root, QAOptions{Width: normalWindowW, Height: normalWindowH, SidebarWidth: defaultSidebar, Theme: themeDark, Tabs: demo.TabFiles[:fewTabs]})
	var flags []qa.Flag
	add := func(measured, expected string) {
		flags = append(flags, qa.Flag{Rule: "sidebar-width", View: "explorer", Owner: qa.OwnerWorkspace, Role: "splitter", Label: splitterLabel, Measured: measured, Expected: expected, Fix: "clamp the sidebar width in the split (MinSize) to token.SidebarMinWidth and keep the editor area usable"})
	}
	q.DragSplitter(min * 3 / 4)
	if got := sidebarWidth(q.Snapshot()); got < min-sidebarTolPx {
		add(widthText(got), "at least token.SidebarMinWidth ("+widthText(min)+")")
	}
	q.dragSplitterNamed(splitterLabel, 0, defaultSidebar)
	if got := sidebarWidth(q.Snapshot()); got < min-sidebarTolPx {
		add("dragged to the window edge and back in one drag: "+widthText(got), "shown again at the pointer")
	}
	q.DragSplitter(0)
	if got := sidebarWidth(q.Snapshot()); got >= 0 {
		add("dragged to the window edge: "+widthText(got), "hidden, as VS Code hides a side bar dragged below half its minimum")
	}
	q.Menu(q.Win, "View", "Primary Side Bar")
	q.DragSplitter(normalWindowW)
	tree := q.Snapshot()
	if editorMin := float64(token.EditorMinWidth.Px()); sidebarWidth(tree) > tree.Window.Width-editorMin {
		add(widthText(sidebarWidth(tree))+" of a "+widthText(tree.Window.Width)+" window", "the editor area keeps at least token.EditorMinWidth ("+widthText(editorMin)+")")
	}
	small := LaunchQA(t, demo.Root, QAOptions{Width: smallestWindowW, Height: smallestWindowH, SidebarWidth: 1, Theme: themeDark, Tabs: demo.TabFiles[:fewTabs]})
	if got := sidebarWidth(small.Snapshot()); got < min-sidebarTolPx {
		add("a saved width of 1px opens at "+widthText(got), "a saved width below the minimum opens at token.SidebarMinWidth ("+widthText(min)+")")
	}
	if len(flags) > 0 {
		t.Errorf("%d flags:\n%s", len(flags), qa.Format(flags))
	}
}
