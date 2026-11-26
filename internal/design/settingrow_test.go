package design

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

const (
	rowTestWidth    = 400
	rowTestHeight   = 300
	rowTestViewport = 80
	rowTestOffset   = 50
)

func TestSettingRowShowsWholeRowOnFocus(t *testing.T) {
	menu := func(m *ui.Menu) { m.Item("Reset Setting") }
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Scroll(c).Height(rowTestViewport).FillWidth().Children(func() {
			ui.Box(c).Height(rowTestOffset)
			SettingRow(c, "Tab Size", "The number of spaces a tab is equal to.", SettingOptions{Menu: menu}, func() {
				ui.Button(c, "Control")
			})
		})
	}, rowTestWidth, rowTestHeight)
	tt.Key(0, ui.KeyTab)
	tt.Frame()
	gear, _ := tt.Access(settingMenuLabel + "Tab Size")
	tt.Key(0, ui.KeyTab)
	tt.Frame()
	if !tt.Focused("Control") {
		t.Fatal("Tab does not reach the control after the gear")
	}
	control, _ := tt.Access("Control")
	if control.Bounds.Y <= gear.Bounds.Y {
		t.Fatalf("gear at %+v, then control at %+v: the row scrolled between its own controls", gear.Bounds, control.Bounds)
	}
}
