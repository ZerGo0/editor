package workspace

import (
	"path/filepath"
	"testing"

	"github.com/ZerGo0/editor/internal/layout"
	"github.com/ZerGo0/editor/internal/session"
	"github.com/ZerGo0/editor/internal/testutil"
	"github.com/egoist/mygo/ui"
)

const (
	resizeViewW = 1200
	resizeViewH = 700
)

func resizeBed(t *testing.T) (*Workspace, *ui.Tester) {
	t.Helper()
	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{"a.txt": "a\n"})
	w := newTestWorkspace(t, root, newTestEnv(t), session.Project{})
	w.OpenFile(filepath.Join(root, "a.txt"), false, true)
	tt := ui.NewTester(w.View, resizeViewW, resizeViewH)
	tt.Frame()
	tt.Frame()
	return w, tt
}

func TestToggleEditorWidthsExpandsTheActiveGroupThenEvensThem(t *testing.T) {
	t.Parallel()
	w, tt := resizeBed(t)
	w.SplitEditor(layout.Right)
	tt.Frame()
	tt.Frame()
	if len(w.layout.Groups()) != 2 {
		t.Fatal("no second group")
	}
	w.ToggleEditorWidths()
	if w.layout.Percent != 0 {
		t.Fatalf("the right group is active, the split is %v%%, want 0 (the left group at its minimum)", w.layout.Percent)
	}
	w.ToggleEditorWidths()
	if w.layout.Percent != halfPercent {
		t.Fatalf("second toggle = %v%%, want even", w.layout.Percent)
	}
}

func TestIncreaseViewSizeGrowsTheActiveGroupBySixtyDIPs(t *testing.T) {
	t.Parallel()
	w, tt := resizeBed(t)
	w.SplitEditor(layout.Right)
	tt.Frame()
	tt.Frame()
	total := w.groupSash(w.layout).Total()
	if total <= 0 {
		t.Fatal("the group split was not laid out")
	}
	w.ResizeView(ViewSizeStep, true, true)
	want := halfPercent - ViewSizeStep*fullPercent/total
	if d := w.layout.Percent - want; d > 0.01 || d < -0.01 {
		t.Fatalf("split = %v%%, want %v%% (the right group 60 DIPs wider)", w.layout.Percent, want)
	}
}

func TestDraggingThePanelBelowHalfItsMinimumClosesIt(t *testing.T) {
	t.Parallel()
	w, tt := resizeBed(t)
	w.ShowPanel(ProblemsPanel)
	tt.Frame()
	tt.Frame()
	var sash ui.Rect
	for _, a := range tt.AccessAll() {
		if a.Role == ui.RoleSplitter && a.Name == resizePanelLabel {
			sash = a.Bounds
		}
	}
	if sash.W == 0 {
		t.Fatal("no panel sash")
	}
	x, y := sash.X+sash.W/2, sash.Y+sash.H/2
	tt.Press(x, y)
	tt.Frame()
	tt.Move(x, resizeViewH)
	tt.Frame()
	tt.Release(x, resizeViewH)
	tt.Frame()
	if w.tools.panelOpen {
		t.Fatal("the panel stayed open after a drag to the bottom edge")
	}
}
