package workspace

import "testing"

func TestTrafficLightsCentreInTitleBar(t *testing.T) {
	p := TrafficLightPosition()
	if p == nil || 2*p.Y+trafficLightSize != TitleBarHeight || p.X != p.Y {
		t.Fatalf("traffic lights at %+v in a %d px title bar, want them centred with an equal left inset", p, TitleBarHeight)
	}
}
