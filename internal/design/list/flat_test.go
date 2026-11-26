package list

import (
	"testing"

	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

func TestFlatTreeRowsCarryTreeSemantics(t *testing.T) {
	selected := -1
	state := &ui.ListState{Selected: &selected}
	items := []Item{
		{Label: "internal", Branch: true, Open: true, Strong: true, Count: 2},
		{Label: "list.go", Depth: 1, Icon: icons.File, Description: "design", Rich: func(s Style) []ui.Span {
			return []ui.Span{{Text: "list", Color: s.Highlight}, {Text: ".go", Color: s.Text}}
		}, Trailing: func(c *ui.Context) { ui.Text(c, "trailing") }},
	}
	tt := ui.NewTester(func(c *ui.Context) {
		View(c, state, len(items), ViewOptions{Label: "Results", Tree: true}, func(i int) Item { return items[i] })
	}, testWidth, testHeight)
	tt.Frame()
	root, ok := tt.AccessByRole(ui.RoleTreeItem, "")
	if !ok || !root.Expanded {
		t.Fatalf("branch row = %+v", root)
	}
	if !tt.HasText("list") || !tt.HasText("trailing") || !tt.HasText("design") {
		t.Fatalf("texts %q", tt.Texts())
	}
}
