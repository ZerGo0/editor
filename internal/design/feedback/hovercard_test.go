package feedback

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

const (
	testWidth  = 400
	testHeight = 300
)

func TestHoverCardHiddenUntilHover(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) {
		row := ui.Text(c, "Fix layout").Description("Shows the commit details")
		HoverCard(c, row, func() { ui.Text(c, "Ada Lovelace") })
	}, testWidth, testHeight)
	if !tt.HasText("Fix layout") {
		t.Fatalf("texts = %v", tt.Texts())
	}
	if tt.HasText("Ada Lovelace") {
		t.Fatal("hover card shows without hover or focus")
	}
}
