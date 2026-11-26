package viewer

import (
	"math"
	"testing"
)

func near(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-4 }

func TestFitScale(t *testing.T) {
	tests := []struct {
		name           string
		vw, vh, iw, ih float32
		enlarge        bool
		want           float32
	}{
		{"fits already", 800, 600, 100, 50, false, 1},
		{"fits already but enlarges", 800, 600, 100, 50, true, 8},
		{"limited by width", 400, 600, 800, 400, false, 0.5},
		{"limited by height", 800, 300, 800, 600, false, 0.5},
		{"no view yet", 0, 0, 800, 600, false, 1},
		{"no image size", 800, 600, 0, 0, false, 1},
		{"tiny result is clamped", 10, 10, 100000, 100000, false, minScale},
	}
	for _, tt := range tests {
		if got := fitScale(tt.vw, tt.vh, tt.iw, tt.ih, tt.enlarge); !near(got, tt.want) {
			t.Errorf("%s: fitScale = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestStepStop(t *testing.T) {
	tests := []struct {
		scale float32
		dir   int
		want  float32
	}{
		{1, 1, 1.5}, {1, -1, 0.75}, {0.9, 1, 1}, {0.9, -1, 0.75}, {1.2, 1, 1.5}, {1.2, -1, 1},
		{maxScale, 1, maxScale}, {minScale, -1, minScale}, {0.06, -1, 0.05}, {0.04, -1, minScale},
	}
	for _, tt := range tests {
		if got := stepStop(tt.scale, tt.dir); !near(got, tt.want) {
			t.Errorf("stepStop(%v, %d) = %v, want %v", tt.scale, tt.dir, got, tt.want)
		}
	}
}

func TestZoomStopsAreSortedAndWithinLimits(t *testing.T) {
	for i, s := range zoomStops {
		if s < minScale || s > maxScale || (i > 0 && s <= zoomStops[i-1]) {
			t.Errorf("stop %d = %v is out of order or range", i, s)
		}
	}
}

func TestClampPan(t *testing.T) {
	tests := []struct{ pan, content, view, want float32 }{
		{50, 100, 100, 0}, {50, 80, 100, 0}, {50, 300, 100, 50}, {150, 300, 100, 100}, {-150, 300, 100, -100},
	}
	for _, tt := range tests {
		if got := clampPan(tt.pan, tt.content, tt.view); !near(got, tt.want) {
			t.Errorf("clampPan(%v, %v, %v) = %v, want %v", tt.pan, tt.content, tt.view, got, tt.want)
		}
	}
}

func TestZoomAroundKeepsThePointerFixed(t *testing.T) {
	const center, pointer, oldScale, newScale = float32(400), float32(550), float32(1.5), float32(3)
	for _, pan := range []float32{-80, 0, 35} {
		imagePoint := (pointer - center - pan) / oldScale
		newPan := zoomAround(pan, oldScale, newScale, pointer, center)
		if got := center + newPan + imagePoint*newScale; !near(got, pointer) {
			t.Errorf("pan %v: the point under the pointer moved to %v", pan, got)
		}
	}
	if got := zoomAround(0, 1, 2, 400, 400); got != 0 {
		t.Errorf("zooming at the center panned by %v", got)
	}
}

func TestWheelFactor(t *testing.T) {
	if !near(wheelFactor(0), 1) {
		t.Error("no scroll must not zoom")
	}
	if wheelFactor(-50) <= 1 || wheelFactor(50) >= 1 {
		t.Error("scrolling up must zoom in and down must zoom out")
	}
	if !near(wheelFactor(30)*wheelFactor(-30), 1) {
		t.Error("opposite scrolls do not cancel")
	}
}

func TestClampScaleAndPercent(t *testing.T) {
	if clampScale(0) != minScale || clampScale(1000) != maxScale || clampScale(1) != 1 {
		t.Error("clampScale")
	}
	if percentLabel(1.5) != 150 || percentLabel(0.125) != 13 {
		t.Errorf("percentLabel: %d %d", percentLabel(1.5), percentLabel(0.125))
	}
}
