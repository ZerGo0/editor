package symbolnav

import (
	"path/filepath"
	"testing"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/ghshared/ghtest"
	"github.com/ZerGo0/editor/internal/search/finder"
	"github.com/ZerGo0/editor/internal/search/textsearch"
	"github.com/ZerGo0/editor/internal/testutil"
	"github.com/egoist/mygo/ui"
)

const (
	peekTestW    = 900
	peekTestH    = 700
	peekLineTop  = 20
	peekLineEnd  = 40
	peekDrag     = 50
	peekListDrag = 40
)

func peekBed(t *testing.T, h *ghtest.FakeHost) (*Peek, *ui.Tester) {
	t.Helper()
	path := filepath.Join(h.Root, "a.go")
	p := New(Env{Host: h, Open: func(finder.Location, finder.OpenOptions) {}, Focus: func() {}})
	p.Begin(References, "foo", 0)
	p.Show(Result{Files: []*finder.FileNode{finder.NewFileNode(path, "a.go", []textsearch.Match{match(0, 0, 3), match(1, 0, 3)})}, Total: 2}, Near{Path: path})
	area := ui.Rect{W: peekTestW, H: peekTestH}
	tt := ui.NewTester(func(c *ui.Context) { p.View(c, Anchor{Area: area, Top: peekLineTop, Bottom: peekLineEnd}) }, peekTestW, peekTestH)
	tt.Frame()
	h.Pump(t, func() bool { return !p.preview.loading })
	tt.Frame()
	tt.Frame()
	return p, tt
}

func bounds(t *testing.T, tt *ui.Tester, role ui.Role, label string) ui.Rect {
	t.Helper()
	a, ok := tt.AccessByRole(role, label)
	if !ok {
		t.Fatalf("no %v %q", role, label)
	}
	return a.Bounds
}

func dragBy(t *testing.T, tt *ui.Tester, label string, dx, dy float32) {
	t.Helper()
	r := bounds(t, tt, ui.RoleSplitter, label)
	dragTo(t, tt, label, r.X+r.W/2+dx, r.Y+r.H/2+dy)
}

func dragTo(t *testing.T, tt *ui.Tester, label string, toX, toY float32) {
	t.Helper()
	r := bounds(t, tt, ui.RoleSplitter, label)
	x, y := r.X+r.W/2, r.Y+r.H/2
	tt.Press(x, y)
	tt.Frame()
	tt.Move(toX, toY)
	tt.Frame()
	tt.Release(toX, toY)
	tt.Frame()
	tt.Frame()
}

func TestPeekHeightAndListWidthResizeAndPersist(t *testing.T) {
	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{"a.go": "foo\nfoo\n"})
	h := ghtest.NewFakeHost(root, t.TempDir())
	p, tt := peekBed(t, h)
	if got := bounds(t, tt, ui.RoleDialog, peekLabel).H; got != token.ReferencePeekHeight.Px() {
		t.Fatalf("peek %v high, want the default %v", got, token.ReferencePeekHeight.Px())
	}
	list := bounds(t, tt, ui.RoleList, resultsLabel).W
	dragBy(t, tt, heightLabel, 0, peekDrag)
	want := token.ReferencePeekHeight.Px() + peekDrag
	if got := bounds(t, tt, ui.RoleDialog, peekLabel).H; got != want || p.layout.Height != int(want) {
		t.Fatalf("peek %v high (saved %d) after dragging its bottom edge, want %v", got, p.layout.Height, want)
	}
	dragBy(t, tt, splitLabel, -peekListDrag, 0)
	if got := bounds(t, tt, ui.RoleList, resultsLabel).W; got != list+peekListDrag {
		t.Fatalf("results %v wide after dragging the divider, want %v", got, list+peekListDrag)
	}
	split := bounds(t, tt, ui.RoleSplitter, splitLabel)
	dragTo(t, tt, splitLabel, peekTestW-1, split.Y+split.H/2)
	if p.layout.List != int(token.PeekListMinWidth.Px()) {
		t.Fatalf("results width %d, want the minimum %v", p.layout.List, token.PeekListMinWidth.Px())
	}
	edge := bounds(t, tt, ui.RoleSplitter, heightLabel)
	dragTo(t, tt, heightLabel, edge.X+edge.W/2, 1)
	if got := bounds(t, tt, ui.RoleDialog, peekLabel).H; got != token.PaneMinHeight.Px() {
		t.Fatalf("peek %v high, want the minimum %v", got, token.PaneMinHeight.Px())
	}
	h.ProjectState().Snapshot()
	again, _ := peekBed(t, h)
	if again.layout != p.layout {
		t.Fatalf("layout %+v after reload, want %+v", again.layout, p.layout)
	}
	tt.Click(heightLabel)
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	tt.Click(splitLabel)
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if p.layout != (peekLayout{}) {
		t.Fatalf("layout %+v after reset, want the defaults", p.layout)
	}
}
