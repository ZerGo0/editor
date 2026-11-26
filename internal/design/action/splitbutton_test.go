package action

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestWideSplitButtonFillsWidth(t *testing.T) {
	const width, height = 400, 100
	tt := ui.NewTester(func(c *ui.Context) {
		SplitButton(c, "Commit", SplitOptions{Variant: Primary, Wide: true, Menu: func(m *ui.Menu) { Item(m, "Commit and Push", ItemOptions{}) }})
	}, width, height)
	info, ok := tt.AccessByRole(ui.RoleGroup, "Commit")
	if !ok {
		t.Fatal("no split button group")
	}
	if info.Bounds.W < width/2 {
		t.Fatalf("wide split button is %.0f wide in %d", info.Bounds.W, width)
	}
}
