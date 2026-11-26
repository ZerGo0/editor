package action

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestSegmentedChoosesWithClick(t *testing.T) {
	layout := 0
	tt := ui.NewTester(func(c *ui.Context) {
		Segmented(c, "Diff layout", &layout, []string{"Split", "Unified"}, SegmentedOptions{})
		MenuButton(c, "More", ButtonOptions{}, func(m *ui.Menu) { Item(m, "Copy Link", ItemOptions{}) })
	}, 400, 120)
	if err := tt.Click("Unified"); err != nil || layout != 1 {
		t.Fatalf("Unified click: %v, layout %d", err, layout)
	}
	if err := tt.Click("More"); err != nil {
		t.Fatal(err)
	}
	if got := tt.Menu(); len(got) != 1 {
		t.Fatalf("menu = %v", got)
	}
}
