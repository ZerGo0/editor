package list

import (
	"testing"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

const (
	narrowWidth   = 200
	longName      = "a-very-long-file-name-that-does-not-fit-in-the-row.go"
	trailingText  = "M"
	asideWidth    = 400
	longAside     = "a-very-long-folder-name/another-long-folder-name/and-a-file-name.svelte 38"
	shortAside    = "post-row.svelte 38"
	minLabelShare = 0.55
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

func asideRow(t *testing.T, label, aside string) (name, side ui.Rect) {
	t.Helper()
	tt := ui.NewTester(func(c *ui.Context) {
		selected := -1
		View(c, &ui.ListState{Selected: &selected}, 1, ViewOptions{Label: "Matches"}, func(int) Item { return Item{Label: label, Aside: aside} })
	}, asideWidth, testHeight)
	tt.Frame()
	var ok bool
	if name, ok = tt.Find(label); !ok {
		t.Fatal("row label is not drawn")
	}
	if side, ok = tt.Find(aside); !ok {
		t.Fatal("row aside is not drawn")
	}
	return name, side
}

func TestAsideNeverTakesMoreThanItsShareOfTheRow(t *testing.T) {
	name, side := asideRow(t, longName+longName, longAside)
	if limit := asideWidth * token.AsideMaxPercent / 100; side.W > limit {
		t.Errorf("aside is %v px wide, want at most %v", side.W, limit)
	}
	if name.W < asideWidth*minLabelShare {
		t.Errorf("label is %v px wide, want at least %v of the %v px row", name.W, minLabelShare, asideWidth)
	}
	if name.X+name.W > side.X {
		t.Errorf("label ends at %v, under the aside at %v", name.X+name.W, side.X)
	}
}

func TestAsideSitsAtTheRightEdge(t *testing.T) {
	_, side := asideRow(t, "x", shortAside)
	if want := float32(asideWidth) - token.PadRow.Px(); side.X+side.W != want {
		t.Errorf("aside ends at %v, want %v", side.X+side.W, want)
	}
}
