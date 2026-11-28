package list

import (
	"runtime"
	"strconv"
	"testing"

	"github.com/egoist/mygo/ui"
)

const pageTestRows = 60

func TestPageKeysMoveTheSelection(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("MyGo moves the selection with Page Up and Page Down itself outside macOS")
	}
	selected := -1
	state := &ui.ListState{Selected: &selected}
	tt := ui.NewTester(func(c *ui.Context) {
		View(c, state, pageTestRows, ViewOptions{Label: "Rows"}, func(i int) Item { return Item{Label: "row " + strconv.Itoa(i)} })
	}, testWidth, testHeight)
	tt.Key(0, ui.KeyTab)
	tt.Key(0, ui.KeyDown)
	tt.Key(0, ui.KeyPageDown)
	tt.Frame()
	first := selected
	if first <= 1 {
		t.Fatalf("Page Down chose row %d; want the last row in view", first)
	}
	tt.Key(0, ui.KeyPageDown)
	tt.Frame()
	if selected <= first {
		t.Fatalf("a second Page Down chose row %d; want a row past %d", selected, first)
	}
	second := selected
	tt.Key(0, ui.KeyPageUp)
	tt.Frame()
	if selected >= second {
		t.Fatalf("Page Up chose row %d; want a row above %d", selected, second)
	}
}

func TestPageTarget(t *testing.T) {
	for _, tc := range []struct{ cur, first, last, dir, want int }{
		{cur: 0, first: 0, last: 9, dir: 1, want: 9},
		{cur: 9, first: 0, last: 9, dir: 1, want: 18},
		{cur: 9, first: 0, last: 9, dir: -1, want: 0},
		{cur: 0, first: 0, last: 9, dir: -1, want: 0},
		{cur: 55, first: 50, last: 59, dir: 1, want: 59},
		{cur: -1, first: 3, last: 9, dir: 1, want: 3},
	} {
		if got := pageTarget(tc.cur, tc.first, tc.last, pageTestRows, tc.dir); got != tc.want {
			t.Errorf("pageTarget(%d, %d, %d, %d) = %d; want %d", tc.cur, tc.first, tc.last, tc.dir, got, tc.want)
		}
	}
}
