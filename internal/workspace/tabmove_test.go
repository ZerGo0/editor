package workspace

import (
	"path/filepath"
	"testing"

	"github.com/ZerGo0/editor/internal/layout"
)

func (b *mouseBed) settle() {
	b.tt.Frame()
	b.tt.Frame()
}

func (b *mouseBed) focusGroup(i int) { b.w.active = b.w.layout.Groups()[i] }

func (b *mouseBed) wantGroups(t *testing.T, want string) {
	t.Helper()
	b.settle()
	if got := b.groups(); got != want {
		t.Fatalf("groups %s, want %s", got, want)
	}
}

func newFilesBed(t *testing.T) *mouseBed {
	t.Helper()
	return newMouseBed(t, map[string]string{"a.txt": "a\n", "b.txt": "b\n", "c.txt": "c\n"})
}

func TestMoveEditorToTheNextGroupOpensAfterItsActiveEditor(t *testing.T) {
	b := newGroupsBed(t)
	b.focusGroup(0)
	b.w.MoveEditorToAdjacentGroup(1)
	b.wantGroups(t, "a.txt|c.txt,b.txt")
	if b.w.active != b.w.layout.Groups()[1] || filepath.Base(b.w.active.ActiveTab().Path) != "b.txt" {
		t.Fatal("the moved editor must be active in the next group")
	}
}

func TestMoveEditorToThePreviousGroup(t *testing.T) {
	b := newGroupsBed(t)
	b.focusGroup(0)
	b.w.MoveEditorToAdjacentGroup(1)
	b.w.MoveEditorToAdjacentGroup(-1)
	b.wantGroups(t, "a.txt,b.txt|c.txt")
}

func TestMoveEditorToTheNextGroupAddsOneWhenMissing(t *testing.T) {
	b := newFilesBed(t)
	b.open("a.txt", "b.txt")
	b.w.MoveEditorToAdjacentGroup(1)
	b.wantGroups(t, "a.txt|b.txt")
	b.w.MoveEditorToAdjacentGroup(-1)
	b.wantGroups(t, "a.txt,b.txt")
}

func TestMoveEditorToTheFirstAndLastGroup(t *testing.T) {
	b := newGroupsBed(t)
	b.focusGroup(0)
	b.w.MoveEditorToEdgeGroup(true)
	b.wantGroups(t, "a.txt|c.txt,b.txt")
	b.w.MoveEditorToEdgeGroup(false)
	b.wantGroups(t, "a.txt,b.txt|c.txt")
}

func TestMoveEditorToASideUsesTheNeighborOrAddsAGroup(t *testing.T) {
	b := newGroupsBed(t)
	b.focusGroup(0)
	b.w.MoveEditorToSide(layout.Right)
	b.wantGroups(t, "a.txt|c.txt,b.txt")
	b.w.MoveEditorToSide(layout.Down)
	b.wantGroups(t, "a.txt|c.txt|b.txt")
	b.w.MoveEditorToSide(layout.Up)
	b.wantGroups(t, "a.txt|c.txt,b.txt")
	b.w.MoveEditorToSide(layout.Left)
	b.wantGroups(t, "a.txt,b.txt|c.txt")
}

func TestMoveEditorToAnEmptySideClosesTheEmptyGroup(t *testing.T) {
	b := newFilesBed(t)
	b.open("a.txt")
	b.w.MoveEditorToSide(layout.Left)
	b.wantGroups(t, "a.txt")
}

func TestMoveEditorWithinTheGroup(t *testing.T) {
	b := newFilesBed(t)
	b.open("a.txt", "b.txt", "c.txt")
	b.w.MoveEditorInGroup(-1)
	b.wantGroups(t, "a.txt,c.txt,b.txt")
	b.w.MoveEditorInGroup(-1)
	b.w.MoveEditorInGroup(-1)
	b.wantGroups(t, "c.txt,a.txt,b.txt")
	b.w.MoveEditorInGroup(1)
	b.wantGroups(t, "a.txt,c.txt,b.txt")
	b.w.MoveEditorInGroup(1)
	b.w.MoveEditorInGroup(1)
	b.wantGroups(t, "a.txt,b.txt,c.txt")
	if b.w.active.ActiveTab().Path != filepath.Join(b.root, "c.txt") {
		t.Fatal("the moved editor must stay active")
	}
}

func TestSplitEditorCopiesTheEditorIntoANewGroupOnEverySide(t *testing.T) {
	for _, side := range []layout.Direction{layout.Right, layout.Left, layout.Up, layout.Down} {
		b := newFilesBed(t)
		b.open("a.txt")
		first := b.w.layout.Groups()[0]
		b.w.SplitEditor(side)
		b.wantGroups(t, "a.txt|a.txt")
		added := b.w.layout.Neighbor(first, side)
		if added == nil || b.w.active != added || added.ActiveTab() == first.ActiveTab() {
			t.Fatalf("%v: the new group must lie on that side, hold a copy and be active", side)
		}
	}
}
