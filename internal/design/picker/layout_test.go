package picker

import (
	"math"
	"testing"

	"github.com/ZerGo0/editor/internal/design/list"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

const (
	layoutLabel   = "Layout dialog"
	dragStep      = 40
	hugeDrag      = 2000
	previewLabel  = "Preview"
	resultsLabel  = "Results"
	edgeTolerance = 1
)

type layoutHarness struct {
	layout  Layout
	results ResultsState
	closed  bool
	preview bool
}

func (h *layoutHarness) view(c *ui.Context) {
	rows := make([]Row, 40)
	for i := range rows {
		rows[i] = Row{Item: list.Item{Label: "row"}}
	}
	if Dialog(c, DialogOptions{Label: layoutLabel, ID: "layout", Layout: &h.layout}, func() {
		Results(c, &h.results, rows, ResultsOptions{Label: resultsLabel, Fill: true})
		if h.preview {
			Preview(c, nil, PreviewOptions{Label: previewLabel, Message: "No preview", Layout: &h.layout, Results: &h.results})
		}
	}) {
		h.closed = true
	}
}

func newLayoutTester(t *testing.T, h *layoutHarness, w, ht int) *ui.Tester {
	t.Helper()
	tt := ui.NewTester(h.view, w, ht)
	tt.Frame()
	tt.Frame()
	return tt
}

func dialogRect(t *testing.T, tt *ui.Tester) ui.Rect {
	t.Helper()
	a, ok := tt.AccessByRole(ui.RoleDialog, layoutLabel)
	if !ok {
		t.Fatal("no dialog")
	}
	return a.Bounds
}

func isWhole(v float32) bool { return v == float32(math.Trunc(float64(v))) }

func drag(t *testing.T, tt *ui.Tester, name string, dx, dy float32) {
	t.Helper()
	a, ok := tt.AccessByRole(ui.RoleSplitter, name)
	if !ok {
		t.Fatalf("no splitter %q", name)
	}
	x, y := a.Bounds.X+a.Bounds.W/2, a.Bounds.Y+a.Bounds.H/2
	tt.Press(x, y)
	tt.Move(x+dx/2, y+dy/2)
	tt.Frame()
	tt.Move(x+dx, y+dy)
	tt.Frame()
	tt.Release(x+dx, y+dy)
	tt.Frame()
	tt.Frame()
}

func TestDialogOpensCenteredInTheWindowOnWholePixels(t *testing.T) {
	for _, size := range [][2]int{{1000, 900}, {1280, 800}, {1001, 701}, {1440, 877}} {
		h := &layoutHarness{}
		tt := newLayoutTester(t, h, size[0], size[1])
		r := dialogRect(t, tt)
		win := ui.Rect{W: float32(size[0]), H: float32(size[1])}
		if gx, gy := r.X+r.W/2-win.W/2, r.Y+r.H/2-win.H/2; math.Abs(float64(gx)) > edgeTolerance || math.Abs(float64(gy)) > edgeTolerance {
			t.Errorf("%v: dialog %+v is off the window centre by %v, %v", size, r, gx, gy)
		}
		for _, v := range []float32{r.X, r.Y, r.W, r.H} {
			if !isWhole(v) {
				t.Errorf("%v: dialog %+v is not on whole pixels", size, r)
				break
			}
		}
		if r.X-(win.W-r.X-r.W) != 0 || r.Y-(win.H-r.Y-r.H) != 0 {
			t.Errorf("%v: dialog %+v is not centered exactly", size, r)
		}
	}
}

func TestDialogStaysInsideSmallWindows(t *testing.T) {
	margin := token.GapControl.Px()
	for _, size := range [][2]int{{600, 400}, {500, 300}, {320, 240}} {
		h := &layoutHarness{}
		tt := newLayoutTester(t, h, size[0], size[1])
		r := dialogRect(t, tt)
		if r.X < margin || r.Y < margin || r.X+r.W > float32(size[0])-margin || r.Y+r.H > float32(size[1])-margin {
			t.Errorf("%v: dialog %+v leaves the window", size, r)
		}
	}
}

func TestDialogUsesItsSavedSizeWithinTheLimits(t *testing.T) {
	h := &layoutHarness{layout: Layout{Width: 700, Height: 400}}
	tt := newLayoutTester(t, h, 1000, 900)
	if r := dialogRect(t, tt); r.W != 700 || r.H != 400 || r.X != 150 || r.Y != 250 {
		t.Fatalf("dialog %+v, want 700x400 at 150,250", r)
	}
	h = &layoutHarness{layout: Layout{Width: 100, Height: 50}}
	tt = newLayoutTester(t, h, 1000, 900)
	if r := dialogRect(t, tt); r.W != token.SearchPopupMinWidth.Px() || r.H != token.SearchPopupMinHeight.Px() {
		t.Fatalf("dialog %+v, want the minimum size", r)
	}
	h = &layoutHarness{layout: Layout{Width: 5000, Height: 5000}}
	tt = newLayoutTester(t, h, 1000, 900)
	if r := dialogRect(t, tt); r.W > 1000-2*token.GapControl.Px() || r.H > 900-2*token.GapControl.Px() {
		t.Fatalf("dialog %+v is larger than the window", r)
	}
}

func TestDraggingEdgesResizesTheDialog(t *testing.T) {
	cases := []struct {
		name   string
		dx, dy float32
		dw, dh int
	}{
		{"Right", dragStep, 0, 2 * dragStep, 0},
		{"Left", -dragStep, 0, 2 * dragStep, 0},
		{"Bottom", 0, dragStep, 0, 2 * dragStep},
		{"Top", 0, -dragStep, 0, 2 * dragStep},
		{"Bottom Right", dragStep, dragStep, 2 * dragStep, 2 * dragStep},
		{"Top Left", -dragStep, -dragStep, 2 * dragStep, 2 * dragStep},
	}
	for _, tc := range cases {
		h := &layoutHarness{layout: Layout{Width: 600, Height: 400}}
		tt := newLayoutTester(t, h, 1000, 900)
		drag(t, tt, resizeLabel+tc.name, tc.dx, tc.dy)
		if h.layout.Width != 600+tc.dw || h.layout.Height != 400+tc.dh {
			t.Errorf("%s: layout %+v, want %dx%d", tc.name, h.layout, 600+tc.dw, 400+tc.dh)
		}
		if h.closed {
			t.Errorf("%s: dragging an edge closed the dialog", tc.name)
		}
		r := dialogRect(t, tt)
		if r.W != float32(h.layout.Width) || r.H != float32(h.layout.Height) {
			t.Errorf("%s: dialog %+v does not follow the layout %+v", tc.name, r, h.layout)
		}
	}
}

func TestResizeStopsAtTheMinimumAndTheWindow(t *testing.T) {
	h := &layoutHarness{layout: Layout{Width: 600, Height: 400}}
	tt := newLayoutTester(t, h, 1000, 900)
	drag(t, tt, resizeLabel+"Bottom Right", -hugeDrag, -hugeDrag)
	if h.layout.Width != int(token.SearchPopupMinWidth.Px()) || h.layout.Height != int(token.SearchPopupMinHeight.Px()) {
		t.Fatalf("layout %+v, want the minimum size", h.layout)
	}
	drag(t, tt, resizeLabel+"Bottom Right", hugeDrag, hugeDrag)
	maxW, maxH := 1000-2*int(token.GapControl.Px()), 900-2*int(token.GapControl.Px())
	if h.layout.Width != maxW || h.layout.Height != maxH {
		t.Fatalf("layout %+v, want the window size %dx%d", h.layout, maxW, maxH)
	}
	if r := dialogRect(t, tt); r.X < token.GapControl.Px() || r.Y < token.GapControl.Px() {
		t.Fatalf("dialog %+v leaves the window", r)
	}
}

func TestPreviewSplitterMovesWithinLimits(t *testing.T) {
	h := &layoutHarness{layout: Layout{Width: 700, Height: 600}, preview: true}
	tt := newLayoutTester(t, h, 1000, 900)
	pane, ok := tt.AccessByRole(ui.RoleGroup, previewLabel)
	if !ok || pane.Bounds.H != token.SearchPreviewHeight.Px() {
		t.Fatalf("preview %+v, want the default height %v", pane.Bounds, token.SearchPreviewHeight.Px())
	}
	drag(t, tt, "Resize "+previewLabel, 0, -dragStep)
	if want := int(token.SearchPreviewHeight.Px()) + dragStep; h.layout.Preview != want {
		t.Fatalf("preview height %d, want %d", h.layout.Preview, want)
	}
	if h.closed {
		t.Fatal("dragging the splitter closed the dialog")
	}
	drag(t, tt, "Resize "+previewLabel, 0, hugeDrag)
	if h.layout.Preview != int(token.PaneMinHeight.Px()) {
		t.Fatalf("preview height %d, want the minimum %v", h.layout.Preview, token.PaneMinHeight.Px())
	}
	drag(t, tt, "Resize "+previewLabel, 0, -hugeDrag)
	results := h.results.height
	if results < token.PaneMinHeight.Px() || h.layout.Preview <= int(token.SearchPreviewHeight.Px()) {
		t.Fatalf("preview height %d leaves %v px for the results", h.layout.Preview, results)
	}
}

func TestPreviewSplitterIsAbsentWithoutALayout(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) { Preview(c, nil, PreviewOptions{Label: previewLabel, Message: "No preview"}) }, testWidth, testHeight)
	tt.Frame()
	if _, ok := tt.AccessByRole(ui.RoleSplitter, "Resize "+previewLabel); ok {
		t.Fatal("a preview without a layout has a splitter")
	}
}
