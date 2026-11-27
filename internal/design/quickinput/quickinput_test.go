package quickinput

import (
	"math"
	"testing"

	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/chrome"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

const (
	testWidth  = 800
	testHeight = 400
)

func TestKeyboard(t *testing.T) {
	items := []Item{{Label: "a.go"}, {Label: "b.go"}, {Label: "c.go"}}
	s := &State{}
	var last Result
	tt := ui.NewTester(func(c *ui.Context) {
		if r := View(c, "Go to File", s, items, Options{}); r.Accepted >= 0 || r.Closed {
			last = r
		}
	}, testWidth, testHeight)
	if _, ok := tt.AccessByRole(ui.RoleDialog, "Go to File"); !ok {
		t.Fatal("quick input has no dialog role")
	}
	tt.Key(0, ui.KeyDown)
	tt.Key(0, ui.KeyEnter)
	if last.Accepted != 1 {
		t.Fatalf("accepted %d, want 1", last.Accepted)
	}
	tt.Key(0, ui.KeyEscape)
	if !last.Closed {
		t.Fatal("Escape does not close the quick input")
	}
}

func TestRowReadsKeys(t *testing.T) {
	items := []Item{{Label: "Go to File...", Keys: "⌘P"}}
	tt := ui.NewTester(func(c *ui.Context) { View(c, "Command Palette", &State{}, items, Options{}) }, testWidth, testHeight)
	for _, a := range tt.AccessAll() {
		if a.Name == "Go to File..." && a.Description == "⌘P" {
			return
		}
	}
	t.Fatal("no row reads its keys as its description")
}

func TestEmptyList(t *testing.T) {
	s := &State{Selected: 4}
	tt := ui.NewTester(func(c *ui.Context) { View(c, "Go to File", s, nil, Options{}) }, testWidth, testHeight)
	if _, ok := tt.Find(defaultEmpty); !ok {
		t.Fatal("empty quick input shows no message")
	}
	if s.Selected != none {
		t.Fatalf("selected = %d on an empty list", s.Selected)
	}
}

const (
	centerCommand = "workbench.action.showCommands"
	panelLabel    = "Change Language Mode"
	sideWidth     = 140
)

var seenBar ui.Rect

func titleBarWithPick(c *ui.Context) {
	ui.Column(c).Fill().Children(func() {
		ui.Row(c).FillWidth().Height(token.TitleBarHeight.Px()).AlignItems(ui.Center).Padding(0, token.PadRow.Px()).Gap(token.GapControl.Px()).Children(func() {
			ui.Box(c).Width(sideWidth).Shrink(0)
			chrome.CommandCenter(c, "editor", centerCommand)
			ui.Box(c).Width(sideWidth).Shrink(0)
		})
	})
	seenBar = chrome.CommandCenterBounds(c)
	View(c, panelLabel, &State{}, []Item{{Label: "Go"}}, Options{})
}

func panelAndBar(t *testing.T, width int) (panel, bar ui.Rect) {
	t.Helper()
	action.SetCommands(nil)
	tt := ui.NewTester(titleBarWithPick, width, testHeight)
	tt.Frame()
	tt.Frame()
	p, ok := tt.AccessByRole(ui.RoleDialog, panelLabel)
	if !ok {
		t.Fatal("no quick input panel")
	}
	return p.Bounds, seenBar
}

func TestPanelMatchesTheCommandCenterBar(t *testing.T) {
	for _, width := range []int{1400, 1000, 901} {
		panel, bar := panelAndBar(t, width)
		if math.Abs(float64(panel.X-bar.X)) > 0.5 || math.Abs(float64(panel.W-bar.W)) > 1 {
			t.Errorf("%d: panel x %v width %v, bar x %v width %v; want the same edges", width, panel.X, panel.W, bar.X, bar.W)
		}
		if gap := panel.Y - (bar.Y + bar.H); gap != token.OffsetPopover.Px() {
			t.Errorf("%d: panel is %v px below the bar, want %v", width, gap, token.OffsetPopover.Px())
		}
		for _, v := range []float32{panel.X, panel.Y, panel.W} {
			if v != float32(math.Trunc(float64(v))) {
				t.Errorf("%d: panel %+v is not on whole pixels", width, panel)
				break
			}
		}
	}
}

func TestPanelUsesItsMinimumWidthCenteredOnANarrowBar(t *testing.T) {
	width := 2*sideWidth + 2*int(token.PadRow.Px()) + 2*int(token.GapControl.Px()) + int(token.CommandCenterMinWidth.Px())
	panel, bar := panelAndBar(t, width)
	if bar.W >= token.QuickInputMinWidth.Px() {
		t.Fatalf("bar is %v wide; the test needs it narrower than %v", bar.W, token.QuickInputMinWidth.Px())
	}
	if panel.W != token.QuickInputMinWidth.Px() {
		t.Fatalf("panel is %v wide, want the minimum %v", panel.W, token.QuickInputMinWidth.Px())
	}
	if d := (panel.X + panel.W/2) - (bar.X + bar.W/2); math.Abs(float64(d)) > 1 {
		t.Fatalf("panel is off the bar centre by %v", d)
	}
	if panel.X < 0 || panel.X+panel.W > float32(width) {
		t.Fatalf("panel %+v leaves the window", panel)
	}
}

func TestAnchoredPanelStaysInsideTheWindow(t *testing.T) {
	f, ok := anchored(ui.Rect{X: 10, Y: 6, W: 100, H: 22}, ui.Rect{W: 400, H: 300})
	if !ok || f.x < 0 || f.x+f.w > 400 || f.w != token.QuickInputMinWidth.Px() {
		t.Fatalf("frame %+v ok %v", f, ok)
	}
	if _, ok := anchored(ui.Rect{}, ui.Rect{W: 400, H: 300}); ok {
		t.Fatal("a missing bar anchors the panel")
	}
}
