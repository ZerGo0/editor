package action

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestChipIsASelectedTab(t *testing.T) {
	active := "Files"
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Row(c).Role(ui.RoleTabList).Children(func() {
			for _, name := range []string{"All", "Files"} {
				if Chip(c, name, ChipOptions{Active: active == name, Count: "2", Keys: "⌘2"}).Clicked() {
					active = name
				}
			}
		})
	}, 300, 80)
	if files, ok := tt.AccessByRole(ui.RoleTab, "Files"); !ok || !files.Selected {
		t.Fatalf("Files chip = %+v", files)
	}
	if err := tt.Click("All"); err != nil || active != "All" {
		t.Fatalf("click All: %v active %q", err, active)
	}
}
