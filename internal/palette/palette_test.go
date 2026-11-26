package palette

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

const (
	testViewW = 800
	testViewH = 600
	testTop   = 10
)

func TestHighlightSelectedUsesTextColor(t *testing.T) {
	th := &ui.Theme{Text: ui.RGBA(1, 2, 3, 255), Accent: ui.RGBA(9, 8, 7, 255)}
	for _, tc := range []struct {
		selected bool
		want     ui.Color
	}{{false, th.Accent}, {true, th.Text}} {
		spans := Highlight("abc", []int{1}, th, tc.selected)
		if len(spans) != 3 || spans[1].Text != "b" || spans[1].Weight != matchWeight || spans[1].Color != tc.want {
			t.Fatalf("selected %v: spans %+v", tc.selected, spans)
		}
	}
}

func TestPaletteHintsIconsAndAccept(t *testing.T) {
	var p Palette
	var icons []string
	var accepted []string
	p.Icon = func(_ *ui.Context, item Item) { icons = append(icons, item.Label) }
	p.Show("Pick", Tab{Search: Static([]Item{{ID: "1", Label: "one"}, {ID: "2", Label: "two"}}), Hint: "Enter to run"})
	tt := ui.NewTester(func(c *ui.Context) {
		p.View(c, testTop, func(item Item) { accepted = append(accepted, item.ID) })
	}, testViewW, testViewH)
	for _, hint := range []string{moveHint, "Enter to run", closeHint} {
		if !tt.HasText(hint) {
			t.Fatalf("hint %q is not shown", hint)
		}
	}
	if len(icons) == 0 {
		t.Fatal("the icon hook is not called")
	}
	p.Move(1)
	p.Accept()
	if len(accepted) != 1 || accepted[0] != "2" || p.IsOpen() {
		t.Fatalf("accepted %v, open %v", accepted, p.IsOpen())
	}
}
