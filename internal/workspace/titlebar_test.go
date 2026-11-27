package workspace

import (
	"testing"

	"github.com/ZerGo0/editor/internal/design/chrome"
	"github.com/egoist/mygo/ui"
)

func TestTrafficLightsCentreInTitleBar(t *testing.T) {
	p := TrafficLightPosition()
	if p == nil || 2*p.Y+trafficLightSize != TitleBarHeight || p.X != p.Y {
		t.Fatalf("traffic lights at %+v in a %d px title bar, want them centred with an equal left inset", p, TitleBarHeight)
	}
}

func TestCommandCenterIsCenteredFromTheFirstFrame(t *testing.T) {
	w, _ := newGuardBed(t)
	var laidOut []ui.Rect
	tt := ui.NewTester(func(c *ui.Context) {
		w.View(c)
		laidOut = append(laidOut, chrome.CommandCenterBounds(c))
	}, testWideW, testViewH)
	tt.Frame()
	first, settled := laidOut[1], laidOut[len(laidOut)-1]
	if first != settled {
		t.Fatalf("command center after the first frame at %+v, settles at %+v", first, settled)
	}
}
