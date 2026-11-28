package workspace

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func openEditorsBed(t *testing.T) *mouseBed {
	t.Helper()
	b := newMouseBed(t, map[string]string{"a.txt": "a\n", "b.txt": "b\n", "c.txt": "c\n"})
	b.w.tools.openEditors = true
	b.open("a.txt", "b.txt", "c.txt")
	return b
}

func TestOpenEditorsMiddleClickCloses(t *testing.T) {
	t.Parallel()
	b := openEditorsBed(t)
	x, y := center(b.bounds(ui.RoleListItem, "b.txt"))
	if x == 0 && y == 0 {
		t.Fatal("no Open Editors row for b.txt")
	}
	b.tt.PressButton(mouseButtonMid, x, y)
	b.tt.Frame()
	b.tt.ReleaseButton(mouseButtonMid, x, y)
	b.tt.Frame()
	if got := b.tabs(); got != "a.txt,*c.txt" {
		t.Fatalf("after the middle click: %s", got)
	}
}

func TestOpenEditorsKeys(t *testing.T) {
	t.Parallel()
	b := openEditorsBed(t)
	b.tt.Frame()
	if b.w.tools.openRow != 2 {
		t.Fatalf("the Open Editors row is %d; want the active editor, 2", b.w.tools.openRow)
	}
	focus := true
	b.tt = ui.NewTester(func(c *ui.Context) {
		b.w.View(c)
		if focus && b.w.tools.openList.Focus(c) {
			focus = false
		}
	}, mouseViewW, mouseViewH)
	b.tt.Frame()
	b.tt.Frame()
	b.tt.Key(0, ui.KeyUp)
	b.tt.Frame()
	b.tt.Key(0, ui.KeyBackspace)
	b.tt.Frame()
	if got := b.tabs(); got != "a.txt,*c.txt" {
		t.Fatalf("after Backspace on b.txt: %s", got)
	}
	b.tt.Key(0, ui.KeyHome)
	b.tt.Key(0, ui.KeyEnter)
	b.tt.Frame()
	if got := b.tabs(); got != "*a.txt,c.txt" {
		t.Fatalf("after Enter on a.txt: %s", got)
	}
}
