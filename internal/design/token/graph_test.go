package token

import "testing"

func TestGraphLanesReadable(t *testing.T) {
	for name, p := range palettes() {
		for i := range GraphLaneCount {
			lane := p.GraphLane(i)
			if got := Contrast(lane, p.Layer.SideBar); got < ContrastLarge {
				t.Errorf("%s lane %d on sidebar: contrast %.2f", name, i, got)
			}
			if got := Contrast(lane, p.Layer.Panel); got < ContrastLarge {
				t.Errorf("%s lane %d on panel: contrast %.2f", name, i, got)
			}
		}
		if p.GraphLane(-1) != p.GraphLane(GraphLaneCount-1) || p.GraphLane(GraphLaneCount) != p.GraphLane(0) {
			t.Errorf("%s: lanes do not wrap", name)
		}
	}
}
