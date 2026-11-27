package workspace

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ZerGo0/editor/internal/command"
	"github.com/ZerGo0/editor/internal/session"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	mouseViewW      = 1200
	mouseViewH      = 800
	mouseButtonMid  = 2
	mouseRowInset   = 2
	mouseGutterX    = 5
	mouseTextX      = 90
	mouseWheelStep  = 80
	mouseDragSteps  = 4
	mouseSettle     = 500 * time.Millisecond
	mouseEdgeInset  = 20
	mouseManyTabs   = 30
	mouseLongLines  = 200
	mouseScrollStep = 10
)

type mouseBed struct {
	w    *Workspace
	tt   *ui.Tester
	root string
}

func newMouseBed(t *testing.T, files map[string]string) *mouseBed {
	t.Helper()
	root := t.TempDir()
	writeFiles(t, root, files)
	env := newTestEnv(t)
	b := &mouseBed{root: root}
	env.Commands.Add(&command.Command{ID: navigateBackID, Title: "Go Back", Run: func(*mygo.Window) { b.w.NavigateBack() }})
	env.Commands.Add(&command.Command{ID: navigateFwdID, Title: "Go Forward", Run: func(*mygo.Window) { b.w.NavigateForward() }})
	b.w = newTestWorkspace(t, root, env, session.Project{})
	b.tt = ui.NewTester(b.w.View, mouseViewW, mouseViewH)
	b.tt.Frame()
	b.tt.Frame()
	return b
}

func (b *mouseBed) open(names ...string) {
	for _, n := range names {
		b.w.OpenFile(filepath.Join(b.root, n), false, true)
		b.tt.Frame()
		b.tt.Frame()
	}
}

func (b *mouseBed) tabs() string {
	var out []string
	for _, tab := range b.w.active.Tabs {
		name := filepath.Base(tab.Path)
		if tab == b.w.active.ActiveTab() {
			name = "*" + name
		}
		out = append(out, name)
	}
	return strings.Join(out, ",")
}

func (b *mouseBed) bounds(role ui.Role, name string) ui.Rect {
	for _, a := range b.tt.AccessAll() {
		if a.Role == role && a.Name == name {
			return a.Bounds
		}
	}
	return ui.Rect{}
}

func (b *mouseBed) tab(name string) ui.Rect { return b.bounds(ui.RoleTab, name) }

func (b *mouseBed) editorRect() ui.Rect {
	for _, a := range b.tt.AccessAll() {
		if a.Name == "Code editor" {
			return a.Bounds
		}
	}
	return ui.Rect{}
}

func (b *mouseBed) drag(fx, fy, tx, ty float32) {
	b.tt.Press(fx, fy)
	for k := 1; k <= mouseDragSteps; k++ {
		f := float32(k) / mouseDragSteps
		b.tt.Move(fx+(tx-fx)*f, fy+(ty-fy)*f)
	}
	b.tt.Release(tx, ty)
	b.tt.Frame()
	b.tt.Frame()
}

func center(r ui.Rect) (float32, float32) { return r.X + r.W/2, r.Y + r.H/2 }

func TestMouseBackAndForwardButtonsNavigateHistory(t *testing.T) {
	b := newMouseBed(t, map[string]string{"a.txt": "a\n", "b.txt": "b\n", "c.txt": "c\n"})
	b.open("a.txt", "b.txt", "c.txt")
	b.tt.Key(0, ui.KeyBack)
	if got := b.tabs(); got != "a.txt,*b.txt,c.txt" {
		t.Fatalf("after the back button: %s", got)
	}
	b.tt.Key(0, ui.KeyForward)
	if got := b.tabs(); got != "a.txt,b.txt,*c.txt" {
		t.Fatalf("after the forward button: %s", got)
	}
}

func TestMiddleClickClosesTheTabOnRelease(t *testing.T) {
	b := newMouseBed(t, map[string]string{"a.txt": "a\n", "b.txt": "b\n", "c.txt": "c\n"})
	b.open("a.txt", "b.txt", "c.txt")
	x, y := center(b.tab("b.txt"))
	b.tt.PressButton(mouseButtonMid, x, y)
	b.tt.Frame()
	if got := b.tabs(); got != "a.txt,b.txt,*c.txt" {
		t.Fatalf("while the middle button is down: %s", got)
	}
	b.tt.ReleaseButton(mouseButtonMid, x, y)
	b.tt.Frame()
	if got := b.tabs(); got != "a.txt,*c.txt" {
		t.Fatalf("after the middle click: %s", got)
	}
}

func TestMiddleClickDraggedOffTheTabKeepsIt(t *testing.T) {
	b := newMouseBed(t, map[string]string{"a.txt": "a\n", "b.txt": "b\n"})
	b.open("a.txt", "b.txt")
	x, y := center(b.tab("a.txt"))
	b.tt.PressButton(mouseButtonMid, x, y)
	b.tt.Move(x, y+mouseViewH/2)
	b.tt.ReleaseButton(mouseButtonMid, x, y+mouseViewH/2)
	b.tt.Frame()
	if got := b.tabs(); got != "a.txt,*b.txt" {
		t.Fatalf("after releasing away from the tab: %s", got)
	}
}

func TestDoubleClickKeepsPreviewTab(t *testing.T) {
	b := newMouseBed(t, map[string]string{"a.txt": "a\n"})
	b.w.OpenFile(filepath.Join(b.root, "a.txt"), true, true)
	b.tt.Frame()
	tab := b.w.active.ActiveTab()
	if !tab.Preview {
		t.Fatal("a file opened as a preview is not a preview tab")
	}
	x, y := center(b.tab("a.txt"))
	b.tt.ClickAt(x, y)
	b.tt.ClickAt(x, y)
	b.tt.Frame()
	if tab.Preview {
		t.Fatal("double click on a preview tab leaves it a preview")
	}
}

func TestDoubleClickOnEmptyTabBarOpensUntitledFile(t *testing.T) {
	b := newMouseBed(t, map[string]string{"a.txt": "a\n"})
	b.open("a.txt")
	tab := b.tab("a.txt")
	_, y := center(tab)
	x := tab.X + tab.W + mouseViewW/4
	b.tt.ClickAt(x, y)
	b.tt.ClickAt(x, y)
	b.tt.Frame()
	if got := b.tabs(); !strings.Contains(got, "*untitled:Untitled-1") {
		t.Fatalf("after double click on the empty tab bar: %s", got)
	}
}

func TestWheelOverTabStripScrollsItSideways(t *testing.T) {
	files := map[string]string{}
	names := make([]string, mouseManyTabs)
	for i := range names {
		names[i] = fmt.Sprintf("many%02d.txt", i)
		files[names[i]] = "x\n"
	}
	b := newMouseBed(t, files)
	b.open(names...)
	first := b.tab(names[0])
	b.tt.Wheel(mouseViewW/2, first.Y+first.H/2, 0, -mouseWheelStep)
	b.tt.Advance(mouseSettle)
	b.tt.Frame()
	if moved := b.tab(names[0]).X - first.X; moved != mouseWheelStep {
		t.Fatalf("the first tab moved %v DIPs for a %d DIP wheel turn", moved, mouseWheelStep)
	}
}

func TestDraggingATabReordersIt(t *testing.T) {
	b := newMouseBed(t, map[string]string{"a.txt": "a\n", "b.txt": "b\n", "c.txt": "c\n"})
	b.open("a.txt", "b.txt", "c.txt")
	fx, fy := center(b.tab("c.txt"))
	tx, ty := center(b.tab("a.txt"))
	b.drag(fx, fy, tx, ty)
	if got := b.tabs(); got != "*c.txt,a.txt,b.txt" {
		t.Fatalf("after dragging c.txt onto a.txt: %s", got)
	}
}

func TestDraggingATabToTheEdgeSplitsTheEditor(t *testing.T) {
	b := newMouseBed(t, map[string]string{"a.txt": "a\n", "b.txt": "b\n"})
	b.open("a.txt", "b.txt")
	fx, fy := center(b.tab("a.txt"))
	area := b.editorRect()
	b.drag(fx, fy, area.X+area.W-mouseEdgeInset, area.Y+area.H/2)
	groups := b.w.layout.Groups()
	if len(groups) != 2 || len(groups[1].Tabs) != 1 || filepath.Base(groups[1].Tabs[0].Path) != "a.txt" {
		t.Fatalf("after dragging a tab to the right edge: %d groups", len(groups))
	}
}

func TestClickAndDragOnLineNumbersSelectsLines(t *testing.T) {
	b := newMouseBed(t, map[string]string{"a.txt": "l1\nl2\nl3\nl4\n"})
	b.open("a.txt")
	area := b.editorRect()
	st := b.w.active.ActiveTab().Editor
	x := area.X + mouseGutterX
	row := func(line int) float32 { return area.Y + float32(st.LineTop(line)) + mouseRowInset }
	b.tt.ClickAt(x, row(1))
	if got := st.SelectedText(); got != "l2\n" {
		t.Fatalf("click on a line number selected %q", got)
	}
	b.drag(x, row(0), x, row(2))
	if got := st.SelectedText(); got != "l1\nl2\nl3\n" {
		t.Fatalf("drag on line numbers selected %q", got)
	}
	b.drag(x, row(3), x, row(1))
	if got := st.SelectedText(); got != "l2\nl3\nl4\n" {
		t.Fatalf("drag up on line numbers selected %q", got)
	}
}

func TestAltWheelScrollsFasterThanTheWheel(t *testing.T) {
	b := newMouseBed(t, map[string]string{"a.txt": strings.Repeat("line\n", mouseLongLines)})
	b.open("a.txt")
	st := b.w.active.ActiveTab().Editor
	plain := ui.InputEvent{Kind: ui.InputScroll, DY: mouseScrollStep}
	if b.w.editorPointer(st, plain); st.ScrollY() != 0 {
		t.Fatalf("a plain wheel turn scrolled %v before the editor", st.ScrollY())
	}
	fast := plain
	fast.Mods = ui.Alt
	if b.w.editorPointer(st, fast); st.ScrollY() != mouseScrollStep*(fastScrollSensitivity-1) {
		t.Fatalf("Alt wheel scrolled %v ahead of the editor", st.ScrollY())
	}
}

func TestCmdClickGoesToTheDefinition(t *testing.T) {
	defs := "package main\n\nfunc foo() {}\n"
	b := newMouseBed(t, map[string]string{"defs.go": defs, "use.go": "foo\n", "other.go": "zzz\n"})
	b.w.search.symbols.Update(filepath.Join(b.root, "defs.go"), "Go", defs)
	b.open("other.go")
	area := b.editorRect()
	b.tt.ClickAtWith(ui.Cmd, area.X+mouseTextX, area.Y+mouseRowInset)
	if got := b.tabs(); got != "*other.go" {
		t.Fatalf("a symbol without a definition opened %s", got)
	}
	b.open("use.go")
	b.tt.ClickAtWith(ui.Cmd, area.X+mouseTextX, area.Y+mouseRowInset)
	b.tt.Frame()
	tab := b.w.active.ActiveTab()
	if filepath.Base(tab.Path) != "defs.go" {
		t.Fatalf("Cmd+click opened %s", b.tabs())
	}
	if pos := tab.Editor.CursorPos(); pos != (editor.Pos{Line: 2, Col: len("func ")}) {
		t.Fatalf("Cmd+click put the caret at %+v", pos)
	}
}

func TestWordAround(t *testing.T) {
	for _, c := range []struct {
		line string
		col  int
		want string
	}{
		{"foo(bar_1)", 0, "foo"}, {"foo(bar_1)", 3, "foo"}, {"foo(bar_1)", 6, "bar_1"}, {"foo( )", 4, ""}, {"", 0, ""}, {"héllo wörld", 8, "wörld"},
	} {
		if got := wordAround(c.line, c.col); got != c.want {
			t.Errorf("wordAround(%q, %d) = %q, want %q", c.line, c.col, got, c.want)
		}
	}
}
