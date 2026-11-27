package chrome

import (
	"bytes"
	"testing"

	"github.com/egoist/mygo/ui"
)

func tabStripPixels(tabs []Tab, selected int) []byte {
	tt := ui.NewTester(func(c *ui.Context) {
		TabStrip(c, &selected, tabs, TabStripOptions{Label: "Editors"})
	}, testWidth, testHeight)
	tt.Frame()
	return tt.Image().Pix
}

func TestDirtyTabPaintsDot(t *testing.T) {
	for _, selected := range []int{0, 1} {
		clean := tabStripPixels([]Tab{{Label: "a.go"}, {Label: "b.go"}}, selected)
		for i := range 2 {
			tabs := []Tab{{Label: "a.go"}, {Label: "b.go"}}
			tabs[i].Dirty = true
			if bytes.Equal(clean, tabStripPixels(tabs, selected)) {
				t.Fatalf("selected %d, dirty tab %d paints no dot", selected, i)
			}
		}
	}
}

func TestPinnedDirtyTabPaintsDot(t *testing.T) {
	clean := tabStripPixels([]Tab{{Label: "a.go", Pinned: true}, {Label: "b.go"}}, 1)
	dirty := tabStripPixels([]Tab{{Label: "a.go", Pinned: true, Dirty: true}, {Label: "b.go"}}, 1)
	if bytes.Equal(clean, dirty) {
		t.Fatal("pinned dirty tab paints no dot")
	}
}

func TestPinnedDirtyTabKeepsThePinBesideTheDot(t *testing.T) {
	dirty := tabStripPixels([]Tab{{Label: "a.go", Dirty: true}, {Label: "b.go"}}, 1)
	pinnedDirty := tabStripPixels([]Tab{{Label: "a.go", Pinned: true, Dirty: true}, {Label: "b.go"}}, 1)
	if bytes.Equal(dirty, pinnedDirty) {
		t.Fatal("pinned dirty tab paints the same as a dirty tab, so the pin is missing")
	}
}
