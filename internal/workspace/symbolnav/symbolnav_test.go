package symbolnav

import (
	"testing"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/search/finder"
	"github.com/ZerGo0/editor/internal/search/textsearch"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

func match(line, col, end int) textsearch.Match {
	return textsearch.Match{Start: textsearch.Pos{Line: line, Col: col}, End: textsearch.Pos{Line: line, Col: end}}
}

func TestMissingNamesTheKindAndTheWord(t *testing.T) {
	for _, c := range []struct {
		kind Kind
		word string
		want string
	}{
		{Definitions, "foo", "No definition found for 'foo'"},
		{References, "foo", "No references found for 'foo'"},
		{Implementations, "", "No implementation found"},
		{TypeDefinitions, "T", "No type definition found for 'T'"},
	} {
		if got := c.kind.Missing(c.word); got != c.want {
			t.Errorf("Missing(%q) = %q, want %q", c.word, got, c.want)
		}
	}
}

func TestNearestPrefersTheMatchUnderTheCaretThenTheClosestLine(t *testing.T) {
	res := Result{Files: []*finder.FileNode{
		{Path: "a.go", Matches: []textsearch.Match{match(1, 0, 3), match(10, 4, 7), match(20, 0, 3)}},
		{Path: "b.go", Matches: []textsearch.Match{match(10, 4, 7)}},
	}}
	if f, m := res.Nearest(Near{Path: "a.go", Pos: editor.Pos{Line: 10, Col: 5}}); f != 0 || m != 1 {
		t.Fatalf("under the caret: file %d match %d", f, m)
	}
	if f, m := res.Nearest(Near{Path: "a.go", Pos: editor.Pos{Line: 18}}); f != 0 || m != 2 {
		t.Fatalf("closest line: file %d match %d", f, m)
	}
	if f, m := res.Nearest(Near{Path: "c.go"}); f != 0 || m != 0 {
		t.Fatalf("another file: file %d match %d", f, m)
	}
}

func TestPlaceUsesTheRoomBelowThenAboveTheLine(t *testing.T) {
	area := ui.Rect{X: 0, Y: 100, W: 900, H: 700}
	want := token.ReferencePeekHeight.Px()
	if y, h := place(Anchor{Area: area, Top: 200, Bottom: 220}, want); y != 220 || h != want {
		t.Fatalf("room below: y %v h %v", y, h)
	}
	if y, h := place(Anchor{Area: area, Top: 700, Bottom: 720}, want); y != 700-want || h != want {
		t.Fatalf("room above: y %v h %v", y, h)
	}
	short := ui.Rect{Y: 0, W: 900, H: 120}
	if y, h := place(Anchor{Area: short, Top: 60, Bottom: 80}, want); y != 0 || h != 120 {
		t.Fatalf("a short editor: y %v h %v", y, h)
	}
}

func TestListWidthKeepsBothPanesUsable(t *testing.T) {
	if got := listWidth(1000, 0); got != 300 {
		t.Fatalf("list width %v of 1000", got)
	}
	if got := listWidth(400, 0); got != 120 {
		t.Fatalf("list width %v of 400", got)
	}
	if got := listWidth(200, 0); got != token.PeekListMinWidth.Px() {
		t.Fatalf("list width %v of 200 is under its minimum", got)
	}
}
