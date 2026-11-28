package workspace

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ZerGo0/editor/internal/layout"
	"github.com/egoist/mygo"
)

const (
	dropLeftHalf  = 0.25
	dropRightHalf = 0.75
	dropAreaEdge  = 0.02
	dropAreaMid   = 0.5
	stripGap      = 40
)

func TestDropPlaceAt(t *testing.T) {
	const w, h = 300, 200
	tests := []struct {
		name string
		x, y float32
		want dropPlace
	}{
		{"inner area merges", 150, 100, dropPlace{}},
		{"inner corner just inside the band", 31, 21, dropPlace{}},
		{"left band", 10, 100, dropPlace{true, layout.Left}},
		{"right band", 290, 100, dropPlace{true, layout.Right}},
		{"top band middle third", 150, 10, dropPlace{true, layout.Up}},
		{"bottom band middle third", 150, 190, dropPlace{true, layout.Down}},
		{"top left corner goes left", 10, 10, dropPlace{true, layout.Left}},
		{"top right corner goes right", 290, 10, dropPlace{true, layout.Right}},
		{"bottom left corner goes left", 10, 190, dropPlace{true, layout.Left}},
		{"top band just past the left third", 101, 10, dropPlace{true, layout.Up}},
		{"top band just past the right third", 201, 10, dropPlace{true, layout.Right}},
		{"upper half of the middle band", 150, 19, dropPlace{true, layout.Up}},
		{"lower half of the middle band", 150, 181, dropPlace{true, layout.Down}},
	}
	for _, tt := range tests {
		if got := dropPlaceAt(tt.x, tt.y, w, h); got != tt.want {
			t.Errorf("%s: dropPlaceAt(%v, %v) = %+v, want %+v", tt.name, tt.x, tt.y, got, tt.want)
		}
	}
	if got := dropPlaceAt(5, 5, 0, 0); got.split {
		t.Error("an area without size must not split")
	}
}

func newGroupsBed(t *testing.T) *mouseBed {
	t.Helper()
	return splitOffC(t, map[string]string{"a.txt": "a\n", "b.txt": "b\n", "c.txt": "c\n", "d.txt": "d\n"}, []string{"a.txt", "b.txt"}, "a.txt,b.txt|c.txt")
}

func (b *mouseBed) dropOnTab(from, onto string, fraction float32) {
	fx, fy := center(b.tab(from))
	r := b.tab(onto)
	b.drag(fx, fy, r.X+r.W*fraction, r.Y+r.H/2)
}

func (b *mouseBed) dropOnGroup(from string, group int, x, y float32) {
	fx, fy := center(b.tab(from))
	r := b.editorRects()[group]
	b.drag(fx, fy, r.X+r.W*x, r.Y+r.H*y)
}

func (b *mouseBed) groups() string { return strings.Join(b.groupTabs(), "|") }

func TestDropOnTheHalvesOfATabInsertsBeforeOrAfter(t *testing.T) {
	for _, tt := range []struct {
		onto     string
		fraction float32
		want     string
	}{
		{"a.txt", dropLeftHalf, "c.txt,a.txt,b.txt"},
		{"a.txt", dropRightHalf, "a.txt,c.txt,b.txt"},
		{"b.txt", dropLeftHalf, "a.txt,c.txt,b.txt"},
		{"b.txt", dropRightHalf, "a.txt,b.txt,c.txt"},
	} {
		b := newMouseBed(t, map[string]string{"a.txt": "a\n", "b.txt": "b\n", "c.txt": "c\n"})
		b.open("a.txt", "b.txt", "c.txt")
		b.dropOnTab("c.txt", tt.onto, tt.fraction)
		if got := b.groups(); got != tt.want {
			t.Errorf("c.txt onto %s at %v: %s, want %s", tt.onto, tt.fraction, got, tt.want)
		}
	}
}

func TestDropOnTheEmptyPartOfTheStripAppends(t *testing.T) {
	b := newMouseBed(t, map[string]string{"a.txt": "a\n", "b.txt": "b\n", "c.txt": "c\n"})
	b.open("a.txt", "b.txt", "c.txt")
	fx, fy := center(b.tab("a.txt"))
	last := b.tab("c.txt")
	b.drag(fx, fy, last.X+last.W+stripGap, fy)
	if got := b.groups(); got != "b.txt,c.txt,a.txt" {
		t.Fatalf("after dropping on the empty strip: %s", got)
	}
}

func TestDropOnATabOfAnotherGroupInsertsByHalf(t *testing.T) {
	for _, tt := range []struct {
		fraction float32
		want     string
	}{
		{dropLeftHalf, "b.txt|a.txt,c.txt"},
		{dropRightHalf, "b.txt|c.txt,a.txt"},
	} {
		b := newGroupsBed(t)
		b.dropOnTab("a.txt", "c.txt", tt.fraction)
		if got := b.groups(); got != tt.want {
			t.Errorf("at %v: %s, want %s", tt.fraction, got, tt.want)
		}
		if tab := b.w.active.ActiveTab(); filepath.Base(tab.Path) != "a.txt" {
			t.Errorf("the moved tab is not active")
		}
	}
}

func TestDropInTheCenterOpensAfterTheActiveEditor(t *testing.T) {
	b := newGroupsBed(t)
	second := b.w.layout.Groups()[1]
	b.w.openIn(second, filepath.Join(b.root, "d.txt"), false, true)
	second.Active = 0
	b.tt.Frame()
	b.tt.Frame()
	b.dropOnGroup("a.txt", 1, dropAreaMid, dropAreaMid)
	if got := b.groups(); got != "b.txt|c.txt,a.txt,d.txt" {
		t.Fatalf("after the center drop: %s", got)
	}
	if b.w.active != second || filepath.Base(second.ActiveTab().Path) != "a.txt" {
		t.Fatal("the dropped editor must be active in the target group")
	}
}

func TestMovingTheOnlyTabOutClosesTheEmptyGroup(t *testing.T) {
	b := newGroupsBed(t)
	b.dropOnTab("c.txt", "a.txt", dropLeftHalf)
	if got := b.groups(); got != "c.txt,a.txt,b.txt" {
		t.Fatalf("after moving the only tab: %s", got)
	}
}

func TestDropOfAFileTheTargetHasActivatesTheExistingTab(t *testing.T) {
	b := newMouseBed(t, map[string]string{"a.txt": "a\n", "b.txt": "b\n"})
	b.open("a.txt", "b.txt")
	first := b.w.layout.Groups()[0]
	first.Active = 0
	b.w.SplitEditor(layout.Right)
	second := b.w.layout.Groups()[1]
	b.w.openIn(second, filepath.Join(b.root, "b.txt"), false, true)
	second.Active = 0
	b.tt.Frame()
	b.tt.Frame()
	if got := b.groups(); got != "a.txt,b.txt|a.txt,b.txt" {
		t.Fatalf("setup groups: %s", got)
	}
	existing := second.Tabs[1]
	fx, fy := center(b.tab("b.txt"))
	r := b.editorRects()[1]
	b.drag(fx, fy, r.X+r.W*dropAreaMid, r.Y+r.H*dropAreaMid)
	if got := b.groups(); got != "a.txt|a.txt,b.txt" {
		t.Fatalf("after the drop: %s", got)
	}
	if second.ActiveTab() != existing || b.w.active != second {
		t.Fatal("the existing tab must become active")
	}
}

func TestCopyDropKeepsTheSourceEditor(t *testing.T) {
	b := newGroupsBed(t)
	groups := b.w.layout.Groups()
	d := tabDrag{groups[0], groups[0].Tabs[0]}
	b.w.drop(d, dropRequest{group: groups[1], at: layout.AfterActive, copying: true})
	b.tt.Frame()
	b.tt.Frame()
	if got := b.groups(); got != "a.txt,b.txt|c.txt,a.txt" {
		t.Fatalf("after the copy drop: %s", got)
	}
	if b.w.active != groups[1] || groups[1].ActiveTab().Path != d.tab.Path || groups[1].ActiveTab() == d.tab {
		t.Fatal("the copy must be a new active tab of the target group")
	}
}

func TestDroppingTheOnlyTabOnItsOwnGroupDoesNothing(t *testing.T) {
	b := newMouseBed(t, map[string]string{"a.txt": "a\n"})
	b.open("a.txt")
	b.dropOnGroup("a.txt", 0, 1-dropAreaEdge, dropAreaMid)
	if got := b.groups(); got != "a.txt" {
		t.Fatalf("after the drop: %s", got)
	}
	g := b.w.layout.Groups()[0]
	b.w.drop(tabDrag{g, g.Tabs[0]}, dropRequest{group: g, at: layout.AfterActive, place: dropPlace{true, layout.Right}, copying: true})
	b.tt.Frame()
	b.tt.Frame()
	if got := b.groups(); got != "a.txt|a.txt" {
		t.Fatalf("after the copy drop: %s", got)
	}
}

func TestDroppingFilesOnTheEditorOpensThem(t *testing.T) {
	b := newMouseBed(t, map[string]string{"a.txt": "a\n", "b.txt": "b\n"})
	b.open("a.txt")
	r := b.editorRects()[0]
	if !b.tt.DropFiles(r.X+r.W*dropAreaMid, r.Y+r.H*dropAreaMid, filepath.Join(b.root, "b.txt")) {
		t.Fatal("the editor area did not take the files")
	}
	b.tt.Frame()
	b.tt.Frame()
	if got := b.groups(); got != "a.txt,b.txt" {
		t.Fatalf("after the file drop: %s", got)
	}
}

func TestDroppingAFolderOnTheEditorOpensTheFolder(t *testing.T) {
	b := newMouseBed(t, map[string]string{"a.txt": "a\n", "sub/c.txt": "c\n"})
	b.open("a.txt")
	var opened []string
	b.w.env.OpenFolder = func(_ *mygo.Window, root string, replace bool) {
		if replace {
			t.Error("a dropped folder replaced the window")
		}
		opened = append(opened, root)
	}
	r := b.editorRects()[0]
	sub := filepath.Join(b.root, "sub")
	if !b.tt.DropFiles(r.X+r.W*dropAreaMid, r.Y+r.H*dropAreaMid, sub) {
		t.Fatal("the editor area did not take the folder")
	}
	b.tt.Frame()
	b.tt.Frame()
	if !slices.Equal(opened, []string{sub}) || b.groups() != "a.txt" {
		t.Fatalf("opened %v, groups %s", opened, b.groups())
	}
}
