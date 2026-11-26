package badge

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestHexColor(t *testing.T) {
	if _, ok := hexColor("zzzzzz"); ok {
		t.Error("invalid hex accepted")
	}
	if _, ok := hexColor(""); ok {
		t.Error("empty hex accepted")
	}
	if _, ok := hexColor("fff"); ok {
		t.Error("short hex accepted")
	}
	if c, ok := hexColor("d73a4a"); !ok || c != ui.RGB(0xd7, 0x3a, 0x4a) {
		t.Errorf("hexColor = %v %v", c, ok)
	}
}

func TestTagShowsTextAndCount(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Row(c).Children(func() {
			Tag(c, "bug", TagOptions{Hex: "d73a4a"})
			Count(c, 1200, CountOptions{})
		})
	}, 200, 60)
	if !tt.HasText("bug") || !tt.HasText("999+") {
		t.Errorf("texts %q", tt.Texts())
	}
}
