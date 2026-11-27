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

func paintsDifferently(a, b Tab) bool {
	return !bytes.Equal(tabStripPixels([]Tab{a, {Label: "b.go"}}, 1), tabStripPixels([]Tab{b, {Label: "b.go"}}, 1))
}

func TestPinnedDirtyTabPaintsItsOwnMarks(t *testing.T) {
	for _, tc := range []struct {
		name         string
		plain, shown Tab
	}{
		{"dot", Tab{Label: "a.go", Pinned: true}, Tab{Label: "a.go", Pinned: true, Dirty: true}},
		{"pin beside the dot", Tab{Label: "a.go", Dirty: true}, Tab{Label: "a.go", Pinned: true, Dirty: true}},
	} {
		if !paintsDifferently(tc.plain, tc.shown) {
			t.Errorf("pinned dirty tab: the %s is not painted", tc.name)
		}
	}
}
