package pane

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

const (
	testWidth  = 300
	testHeight = 200
)

func TestHeaderTogglesWithKeyboard(t *testing.T) {
	open := true
	tt := ui.NewTester(func(c *ui.Context) {
		Section(c, "Outline", &open, SectionOptions{}, func(c *ui.Context) { ui.Text(c, "body") })
	}, testWidth, testHeight)
	info, ok := tt.Access("Outline")
	if !ok || !info.Expanded {
		t.Fatalf("header = %+v, want an expanded control", info)
	}
	tt.Key(0, ui.KeyTab)
	tt.Key(0, ui.KeySpace)
	if open {
		t.Fatal("Space does not collapse the section")
	}
	if _, ok := tt.Find("body"); ok {
		t.Fatal("collapsed section still shows its body")
	}
}

func TestToolbarRole(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) {
		Toolbar(c, "Explorer Actions", func(c *ui.Context) { ui.Button(c, "New File") })
	}, testWidth, testHeight)
	if _, ok := tt.AccessByRole(ui.RoleToolbar, "Explorer Actions"); !ok {
		t.Fatal("toolbar has no toolbar role")
	}
}
