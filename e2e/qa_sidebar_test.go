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
	splitterLabel = "Divider"
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
	splitter, err := q.TryWait(func(n qa.Node) bool { return n.Role == "splitter" && n.Bounds.Height > n.Bounds.Width }, navTimeout)
	if err != nil {
		q.t.Fatalf("splitter: %v", err)
	}
	x, y := splitter.Bounds.CenterX(), splitter.Bounds.CenterY()
	q.Send(map[string]any{"type": "pointerMove", "x": x, "y": y})
	q.Send(map[string]any{"type": "pointerDown", "x": x, "y": y})
	for i := 1; i <= dragSteps; i++ {
		q.Send(map[string]any{"type": "pointerMove", "x": x + (toX-x)*float64(i)/dragSteps, "y": y})
	}
	q.Send(map[string]any{"type": "pointerUp", "x": toX, "y": y})
	q.Settle()
}

func TestSidebarWidthStaysWithinLimits(t *testing.T) {
	demo := NewDemoProject(t)
	min := float64(token.SidebarMinWidth.Px())
	q := LaunchQA(t, demo.Root, QAOptions{Width: normalWindowW, Height: normalWindowH, SidebarWidth: defaultSidebar, Theme: themeDark, Tabs: demo.TabFiles[:fewTabs]})
	var flags []qa.Flag
	add := func(measured, expected string) {
		flags = append(flags, qa.Flag{Rule: "sidebar-width", View: "explorer", Owner: qa.OwnerWorkspace, Role: "splitter", Label: splitterLabel, Measured: measured, Expected: expected, Fix: "clamp the sidebar width in the split (MinSize) to token.SidebarMinWidth and keep the editor area usable"})
	}
	q.DragSplitter(0)
	if got := sidebarWidth(q.Snapshot()); got < min-sidebarTolPx {
		add(widthText(got), "at least token.SidebarMinWidth ("+widthText(min)+")")
	}
	q.DragSplitter(normalWindowW)
	tree := q.Snapshot()
	if got := sidebarWidth(tree); got > tree.Window.Width-min {
		add(widthText(got)+" of a "+widthText(tree.Window.Width)+" window", "the editor area keeps at least token.SidebarMinWidth ("+widthText(min)+")")
	}
	small := LaunchQA(t, demo.Root, QAOptions{Width: smallestWindowW, Height: smallestWindowH, SidebarWidth: 1, Theme: themeDark, Tabs: demo.TabFiles[:fewTabs]})
	if got := sidebarWidth(small.Snapshot()); got < min-sidebarTolPx {
		add("a saved width of 1px opens at "+widthText(got), "a saved width below the minimum opens at token.SidebarMinWidth ("+widthText(min)+")")
	}
	if len(flags) > 0 {
		t.Errorf("%d flags:\n%s", len(flags), qa.Format(flags))
	}
}
