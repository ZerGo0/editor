package list

import (
	"testing"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

const (
	narrowWidth  = 200
	longName     = "a-very-long-file-name-that-does-not-fit-in-the-row.go"
	trailingText = "M"
)

func TestLongNameTruncatesBeforeTrailing(t *testing.T) {
	var natural float32
	tt := ui.NewTester(func(c *ui.Context) {
		natural, _ = c.MeasureText(0, ui.Span{Text: longName, Size: token.TextBody.Px()})
		selected := -1
		View(c, &ui.ListState{Selected: &selected}, 1, ViewOptions{Label: "Changes"}, func(int) Item {
			return Item{Label: longName, Trailing: func(c *ui.Context) { ui.Text(c, trailingText) }}
		})
	}, narrowWidth, testHeight)
	tt.Frame()
	name, ok := tt.Find(longName)
	if !ok {
		t.Fatal("row name is not drawn")
	}
	trailing, ok := tt.Find(trailingText)
	if !ok {
		t.Fatal("trailing item is not drawn")
	}
	if name.X+name.W > trailing.X {
		t.Errorf("name ends at %v, under the trailing item at %v", name.X+name.W, trailing.X)
	}
	if trailing.X+trailing.W > narrowWidth {
		t.Errorf("trailing item ends at %v, outside the %v px row", trailing.X+trailing.W, narrowWidth)
	}
	if name.W >= natural {
		t.Errorf("name is %v px wide, want less than its natural %v px (truncated)", name.W, natural)
	}
}
