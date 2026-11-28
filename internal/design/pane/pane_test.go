package pane

import (
	"testing"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

const (
	testWidth  = 300
	testHeight = 200
)

func TestHeaderTogglesWithKeyboard(t *testing.T) {
	open := true
	tt := ui.NewTester(func(c *ui.Context) {
		Section(c, "Outline", &open, SectionOptions{}, func(c *ui.Context) { ui.Text(c, "body") })
	}, testWidth, testHeight)
	info, ok := tt.Access("Outline")
	if !ok || !info.Expanded {
		t.Fatalf("header = %+v, want an expanded control", info)
	}
	tt.Key(0, ui.KeyTab)
	tt.Key(0, ui.KeySpace)
	if open {
		t.Fatal("Space does not collapse the section")
	}
	if _, ok := tt.Find("body"); ok {
		t.Fatal("collapsed section still shows its body")
	}
}

func TestToolbarRole(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) {
		Toolbar(c, "Explorer Actions", func(c *ui.Context) { ui.Button(c, "New File") })
	}, testWidth, testHeight)
	if _, ok := tt.AccessByRole(ui.RoleToolbar, "Explorer Actions"); !ok {
		t.Fatal("toolbar has no toolbar role")
	}
}

func stackPanes() []Pane {
	return []Pane{{ID: "a", Title: "Alpha"}, {ID: "b", Title: "Beta"}}
}

func TestStackSashResizesWithKeysAndResetsEvenly(t *testing.T) {
	var s StackState
	tt := ui.NewTester(func(c *ui.Context) { Stack(c, &s, stackPanes()) }, testWidth, testHeight*2)
	tt.Frame()
	info, ok := tt.AccessByRole(ui.RoleSplitter, "Resize Alpha")
	if !ok {
		t.Fatal("the sash between the sections has no splitter label")
	}
	tt.ClickAt(info.Bounds.X+info.Bounds.W/2, info.Bounds.Y+info.Bounds.H/2)
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	if s.Weights["a"] <= s.Weights["b"] {
		t.Fatalf("Down did not grow the upper section: %v", s.Weights)
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if len(s.Weights) != 0 {
		t.Fatalf("reset kept weights %v, want even sections", s.Weights)
	}
}

const (
	splitWidth   = 800
	splitHeight  = 300
	splitStart   = 260
	splitDragOut = 10
	splitDragIn  = 300
)

func splitBed(t *testing.T, size *float32, hidden *bool, o SplitOptions) (*ui.Tester, func() ui.Rect) {
	t.Helper()
	var s SplitState
	o.Label, o.Hidden, o.Snap, o.FirstMin = "Resize Side", hidden, SnapFirst, token.SidebarMinWidth
	tt := ui.NewTester(func(c *ui.Context) {
		Split(c, &s, size, o, func() { ui.Text(c, "side") }, func() { ui.Text(c, "main") }).Fill()
	}, splitWidth, splitHeight)
	tt.Frame()
	tt.Frame()
	sash := func() ui.Rect {
		info, ok := tt.AccessByRole(ui.RoleSplitter, "Resize Side")
		if !ok {
			t.Fatal("no sash")
		}
		return info.Bounds
	}
	return tt, sash
}

func dragSash(tt *ui.Tester, r ui.Rect, toX float32) {
	y := r.Y + r.H/2
	tt.Press(r.X+r.W/2, y)
	tt.Frame()
	tt.Move(toX, y)
	tt.Frame()
}

func TestSplitHidesTheSideDraggedPastHalfItsMinimumAndShowsItAgain(t *testing.T) {
	size, hidden := float32(splitStart), false
	tt, sash := splitBed(t, &size, &hidden, SplitOptions{})
	r := sash()
	dragSash(tt, r, splitDragOut)
	if !hidden || tt.HasText("side") {
		t.Fatalf("dragged to x=%d: hidden=%v, want the side hidden", splitDragOut, hidden)
	}
	tt.Move(splitDragIn, r.Y+r.H/2)
	tt.Frame()
	tt.Frame()
	tt.Release(splitDragIn, r.Y+r.H/2)
	tt.Frame()
	if hidden || !tt.HasText("side") {
		t.Fatal("dragging back did not show the side again")
	}
	if size < splitDragIn-token.SashSize.Px() || size > splitDragIn+token.SashSize.Px() {
		t.Fatalf("size = %v, want about %d", size, splitDragIn)
	}
}

func TestSplitStopsAtTheMinimumWithoutSnap(t *testing.T) {
	size := float32(splitStart)
	var s SplitState
	tt := ui.NewTester(func(c *ui.Context) {
		Split(c, &s, &size, SplitOptions{Label: "Resize Side", FirstMin: token.SidebarMinWidth, Reset: splitStart}, func() { ui.Text(c, "side") }, func() { ui.Text(c, "main") }).Fill()
	}, splitWidth, splitHeight)
	tt.Frame()
	tt.Frame()
	info, _ := tt.AccessByRole(ui.RoleSplitter, "Resize Side")
	dragSash(tt, info.Bounds, splitDragOut)
	tt.Release(splitDragOut, info.Bounds.Y)
	tt.Frame()
	if size != token.SidebarMinWidth.Px() {
		t.Fatalf("size = %v, want the minimum %v", size, token.SidebarMinWidth.Px())
	}
	info, _ = tt.AccessByRole(ui.RoleSplitter, "Resize Side")
	tt.ClickAt(info.Bounds.X+info.Bounds.W/2, info.Bounds.Y+info.Bounds.H/2)
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if size != splitStart {
		t.Fatalf("reset size = %v, want %d", size, splitStart)
	}
}
