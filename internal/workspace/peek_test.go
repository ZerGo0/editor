package workspace

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ZerGo0/editor/internal/command"
	"github.com/ZerGo0/editor/internal/session"
	"github.com/ZerGo0/editor/internal/testutil"
	"github.com/ZerGo0/editor/internal/workspace/symbolnav"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	peekWait    = 5 * time.Second
	peekPoll    = 2 * time.Millisecond
	peekResults = "Peek results"
	peekPreview = "Peek preview"
	useCol      = len("func use() { ")
	defsSource  = "package main\n\n// foo is documented here\nfunc foo() {}\n\nvar text = \"foo\"\n"
	otherSource = "package main\n\nfunc foo() {}\n"
	useSource   = "package main\n\nfunc use() { foo() }\n"
)

type peekBed struct {
	w    *Workspace
	tt   *ui.Tester
	root string
}

func newPeekBed(t *testing.T, files map[string]string) *peekBed {
	t.Helper()
	root := t.TempDir()
	testutil.WriteFiles(t, root, files)
	env := newTestEnv(t)
	b := &peekBed{root: root}
	for _, act := range symbolnav.Actions {
		env.Commands.Add(&command.Command{ID: act.ID, Title: act.Title, Run: func(*mygo.Window) { b.w.RunSymbolNav(act) }})
	}
	b.w = newTestWorkspace(t, root, env, session.Project{})
	ctx := context.Background()
	if err := b.w.search.files.Refresh(ctx, func(string) bool { return false }); err != nil {
		t.Fatal(err)
	}
	paths, _ := b.w.search.files.Paths()
	b.w.search.scanner.Scan(ctx, paths)
	b.tt = ui.NewTester(b.w.View, mouseViewW, mouseViewH)
	b.tt.Frame()
	b.tt.Frame()
	return b
}

func (b *peekBed) open(t *testing.T, name string, pos editor.Pos) *editor.State {
	t.Helper()
	tab, _ := openTab(t, b.w, filepath.Join(b.root, name))
	tab.Editor.GoTo(pos)
	b.w.focusTab = tab
	b.tt.Frame()
	b.tt.Frame()
	return tab.Editor
}

func (b *peekBed) until(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(peekWait)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		b.tt.Frame()
		time.Sleep(peekPoll)
	}
}

func (b *peekBed) key(mods ui.Modifiers, key ui.Key) {
	b.tt.Key(mods, key)
	b.tt.Frame()
	b.tt.Frame()
}

func (b *peekBed) active() string {
	if t := b.w.active.ActiveTab(); t != nil {
		return filepath.Base(t.Path)
	}
	return ""
}

func (b *peekBed) visible() bool { return b.w.peek.view.Visible() }

func (b *peekBed) previewText() string {
	for _, a := range b.tt.AccessAll() {
		if a.Name == peekPreview {
			return a.Value
		}
	}
	return ""
}

func (b *peekBed) rows() []string {
	var out []string
	listed := false
	for _, a := range b.tt.AccessAll() {
		switch {
		case a.Role == ui.RoleList && a.Name == peekResults:
			listed = true
		case listed && a.Role == ui.RoleTreeItem:
			out = append(out, a.Name)
		}
	}
	return out
}

func newPeekedBed(t *testing.T) (*peekBed, *editor.State) {
	t.Helper()
	b := newPeekBed(t, map[string]string{"defs.go": defsSource, "other.go": otherSource, "use.go": useSource})
	st := b.open(t, "use.go", editor.Pos{Line: 2, Col: useCol})
	b.peekOn(t, ui.Alt, ui.KeyF12, "documented here")
	return b, st
}

func (b *peekBed) peekOn(t *testing.T, mods ui.Modifiers, key ui.Key, preview string) {
	t.Helper()
	b.key(mods, key)
	b.until(t, "the peek preview", func() bool { return b.visible() && strings.Contains(b.previewText(), preview) })
}

func TestF12GoesToTheDefinition(t *testing.T) {
	b := newPeekBed(t, map[string]string{"defs.go": "package main\n\nfunc foo() {}\n", "use.go": useSource})
	b.open(t, "use.go", editor.Pos{Line: 2, Col: useCol})
	b.key(0, ui.KeyF12)
	if got := b.active(); got != "defs.go" {
		t.Fatalf("F12 opened %s", got)
	}
	if pos := b.w.active.ActiveTab().Editor.CursorPos(); pos != (editor.Pos{Line: 2, Col: len("func ")}) {
		t.Fatalf("F12 put the caret at %+v", pos)
	}
}

func TestF12ReportsAnUnknownName(t *testing.T) {
	b := newPeekBed(t, map[string]string{"use.go": useSource})
	b.open(t, "use.go", editor.Pos{Line: 2, Col: useCol})
	b.key(0, ui.KeyF12)
	if b.active() != "use.go" || b.visible() {
		t.Fatalf("F12 on a name without a definition moved to %s, peek %v", b.active(), b.visible())
	}
}

func TestF12DoesNothingWhereTheLanguageHasNoSymbols(t *testing.T) {
	b := newPeekBed(t, map[string]string{"notes.txt": "foo\n", "defs.go": "package main\n\nfunc foo() {}\n"})
	b.open(t, "notes.txt", editor.Pos{})
	b.key(0, ui.KeyF12)
	if b.active() != "notes.txt" {
		t.Fatalf("F12 in a plain text file opened %s", b.active())
	}
}

func TestChordCmdKF12OpensTheDefinitionToTheSide(t *testing.T) {
	b := newPeekBed(t, map[string]string{"defs.go": "package main\n\nfunc foo() {}\n", "use.go": useSource})
	b.open(t, "use.go", editor.Pos{Line: 2, Col: useCol})
	b.tt.Key(ui.Cmd, ui.KeyK)
	b.key(0, ui.KeyF12)
	groups := b.w.layout.Groups()
	if len(groups) != 2 {
		t.Fatalf("Cmd+K F12 left %d groups", len(groups))
	}
	if got := filepath.Base(groups[0].ActiveTab().Path); got != "use.go" {
		t.Fatalf("the first group shows %s", got)
	}
	if got := filepath.Base(groups[1].ActiveTab().Path); got != "defs.go" || b.w.active != groups[1] {
		t.Fatalf("the side group shows %s (active %v)", got, b.w.active == groups[1])
	}
}

func TestAltF12PeeksTheDefinitionsAndEnterOpensOne(t *testing.T) {
	b := newPeekBed(t, map[string]string{"defs.go": defsSource, "other.go": otherSource, "use.go": useSource})
	b.open(t, "use.go", editor.Pos{Line: 2, Col: useCol})
	b.peekOn(t, ui.Alt, ui.KeyF12, "documented here")
	if got := b.active(); got != "use.go" {
		t.Fatalf("the peek moved the editor to %s", got)
	}
	if rows := b.rows(); len(rows) != 4 || !strings.HasPrefix(rows[0], "defs.go") || !strings.HasPrefix(rows[2], "other.go") {
		t.Fatalf("peek rows %q", rows)
	}
	b.key(0, ui.KeyF4)
	b.until(t, "the second definition", func() bool { return !strings.Contains(b.previewText(), "documented here") && b.previewText() != "" })
	if b.previewText() != otherSource {
		t.Fatalf("F4 shows %q", b.previewText())
	}
	b.key(ui.Shift, ui.KeyF4)
	b.until(t, "the first definition", func() bool { return strings.Contains(b.previewText(), "documented here") })
	b.key(0, ui.KeyF12)
	b.until(t, "the second definition by F12", func() bool { return b.previewText() == otherSource })
	b.key(0, ui.KeyEnter)
	if b.visible() {
		t.Fatal("Enter left the peek open")
	}
	tab := b.w.active.ActiveTab()
	if filepath.Base(tab.Path) != "other.go" || tab.Editor.CursorPos() != (editor.Pos{Line: 2, Col: len("func foo")}) {
		t.Fatalf("Enter opened %s at %+v", tab.Path, tab.Editor.CursorPos())
	}
}

func TestEscapeClosesThePeekAndFocusesTheEditor(t *testing.T) {
	b, st := newPeekedBed(t)
	if st.Focused() {
		t.Fatal("the editor kept the focus under the peek")
	}
	b.key(0, ui.KeyEscape)
	b.tt.Frame()
	if b.visible() || !st.Focused() {
		t.Fatalf("after Escape: peek %v, editor focused %v", b.visible(), st.Focused())
	}
}

func TestEscapeInTheEditorClosesThePeek(t *testing.T) {
	b, st := newPeekedBed(t)
	b.w.focusTab = b.w.active.ActiveTab()
	b.tt.Frame()
	b.tt.Frame()
	if !st.Focused() {
		t.Fatal("the editor did not take the focus back")
	}
	b.key(0, ui.KeyEscape)
	if b.visible() {
		t.Fatal("Escape in the editor left the peek open")
	}
}

func TestPeekClosesWhenThePointerGoesDownOutsideIt(t *testing.T) {
	b := newPeekBed(t, map[string]string{"defs.go": defsSource, "other.go": otherSource, "use.go": useSource})
	b.open(t, "use.go", editor.Pos{Line: 2, Col: useCol})
	b.peekOn(t, ui.Alt, ui.KeyF12, "documented here")
	b.tt.ClickAt(mouseGutterX, mouseRowInset)
	b.tt.Frame()
	if b.visible() {
		t.Fatal("a click outside left the peek open")
	}
}

func TestPeekOfASingleDefinitionHasNoList(t *testing.T) {
	b := newPeekBed(t, map[string]string{"defs.go": otherSource, "use.go": useSource})
	b.open(t, "use.go", editor.Pos{Line: 2, Col: useCol})
	b.peekOn(t, ui.Alt, ui.KeyF12, "func foo")
	if rows := b.rows(); len(rows) != 0 {
		t.Fatalf("a single result shows the rows %q", rows)
	}
	b.key(0, ui.KeyEscape)
	if b.visible() {
		t.Fatal("Escape left the single-result peek open")
	}
}

func TestMultipleDefinitionsOpenThePeekForF12(t *testing.T) {
	b := newPeekBed(t, map[string]string{"defs.go": defsSource, "other.go": otherSource, "use.go": useSource})
	b.open(t, "use.go", editor.Pos{Line: 2, Col: useCol})
	b.peekOn(t, 0, ui.KeyF12, "documented here")
	if b.w.searchOpen() {
		t.Fatal("F12 with several definitions opened Go to Symbol in Workspace")
	}
}

func TestShiftF12ListsReferencesWithoutCommentsAndStrings(t *testing.T) {
	b := newPeekBed(t, map[string]string{"defs.go": defsSource, "use.go": useSource})
	b.open(t, "use.go", editor.Pos{Line: 2, Col: useCol})
	b.peekOn(t, ui.Shift, ui.KeyF12, "use")
	rows := b.rows()
	want := []string{"use.go 1", "func use() { foo() } line 3", "defs.go 1", "func foo() {} line 4 Definition"}
	if len(rows) != len(want) {
		t.Fatalf("reference rows %q, want %q", rows, want)
	}
	for i := range want {
		if rows[i] != want[i] {
			t.Fatalf("reference rows %q, want %q", rows, want)
		}
	}
}

func TestReferencesPutTheDeclarationFirstInItsFile(t *testing.T) {
	src := "package main\n\nfunc early() { foo() }\n\nfunc foo() {}\n\nfunc late() { foo() }\n"
	b := newPeekBed(t, map[string]string{"a.go": src})
	b.open(t, "a.go", editor.Pos{Line: 2, Col: len("func early() { ")})
	b.peekOn(t, ui.Shift, ui.KeyF12, "func foo")
	rows := b.rows()
	if len(rows) != 4 || rows[1] != "func foo() {} line 5 Definition" || rows[2] != "func early() { foo() } line 3" {
		t.Fatalf("rows %q", rows)
	}
}

func TestF12OnTheDeclarationListsItsReferences(t *testing.T) {
	b := newPeekBed(t, map[string]string{"defs.go": otherSource, "use.go": useSource})
	b.open(t, "defs.go", editor.Pos{Line: 2, Col: len("func ")})
	b.peekOn(t, 0, ui.KeyF12, "func foo")
	if rows := b.rows(); len(rows) != 4 {
		t.Fatalf("F12 on a definition showed %q, want its references", rows)
	}
}

func TestReferencesOfALocalNameStayInTheCurrentFile(t *testing.T) {
	local := "package main\n\nfunc f() {\n\tcount := 1\n\t_ = count\n}\n"
	b := newPeekBed(t, map[string]string{"a.go": local, "b.go": "package main\n\nvar _ = count\n"})
	b.open(t, "a.go", editor.Pos{Line: 3, Col: 2})
	b.peekOn(t, ui.Shift, ui.KeyF12, "count := 1")
	if rows := b.rows(); len(rows) != 3 || !strings.HasPrefix(rows[0], "a.go") {
		t.Fatalf("local reference rows %q", rows)
	}
}

func TestFindAllReferencesFillsTheFindPanel(t *testing.T) {
	b := newPeekBed(t, map[string]string{"defs.go": defsSource, "use.go": useSource})
	b.open(t, "use.go", editor.Pos{Line: 2, Col: useCol})
	b.key(ui.Shift|ui.Alt, ui.KeyF12)
	b.until(t, "the references in the Find panel", func() bool { return b.w.search.find.Count() == 2 })
	if b.visible() {
		t.Fatal("Find All References opened the peek")
	}
	if b.w.tools.panel != FindPanel {
		t.Fatalf("the shown panel is %q", b.w.tools.panel)
	}
}

func TestContextMenuOffersDefinitionReferencesAndPeek(t *testing.T) {
	b := newPeekBed(t, map[string]string{"defs.go": otherSource, "use.go": "foo()\n", "notes.txt": "foo\n"})
	b.open(t, "use.go", editor.Pos{})
	var area ui.Rect
	for _, a := range b.tt.AccessAll() {
		if a.Name == "Code editor" {
			area = a.Bounds
		}
	}
	b.tt.RightClickAt(area.X+mouseTextX, area.Y+mouseRowInset)
	b.tt.Frame()
	menu := strings.Join(b.tt.Menu(), "|")
	for _, want := range []string{"Go to Definition", "Go to References", "Peek"} {
		if !strings.Contains(menu, want) {
			t.Fatalf("the context menu %q lacks %q", menu, want)
		}
	}
	if err := b.tt.ChooseMenuItem("Peek", "Peek Definition"); err != nil {
		t.Fatal(err)
	}
	b.until(t, "the peek from the menu", func() bool { return b.visible() && strings.Contains(b.previewText(), "func foo") })
	b.key(0, ui.KeyEscape)
	b.open(t, "notes.txt", editor.Pos{})
	b.tt.RightClickAt(area.X+mouseTextX, area.Y+mouseRowInset)
	b.tt.Frame()
	if menu := strings.Join(b.tt.Menu(), "|"); strings.Contains(menu, "Go to References") || strings.Contains(menu, "Go to Definition") {
		t.Fatalf("a plain text file offers symbol navigation: %q", menu)
	}
}

func (b *peekBed) bounds(name string) ui.Rect {
	for _, a := range b.tt.AccessAll() {
		if a.Name == name {
			return a.Bounds
		}
	}
	return ui.Rect{}
}

func TestPeekSitsUnderTheCursorLineAcrossTheEditor(t *testing.T) {
	b, st := newPeekedBed(t)
	b.tt.Frame()
	area, panel, preview, results := b.bounds("Code editor"), b.bounds("Peek"), b.bounds(peekPreview), b.bounds(peekResults)
	lineBottom := area.Y + float32(st.LineTop(3)-st.ScrollY())
	if panel.X != area.X || panel.W != area.W || panel.Y != lineBottom {
		t.Fatalf("the peek is at %+v, want the width of the editor %+v under y %v", panel, area, lineBottom)
	}
	if preview.X != panel.X || preview.X+preview.W > results.X || results.X+results.W != panel.X+panel.W {
		t.Fatalf("preview %+v and results %+v do not fill the peek %+v", preview, results, panel)
	}
	if preview.W <= results.W || preview.W != float32(int(preview.W)) {
		t.Fatalf("the preview is %v wide and the results %v", preview.W, results.W)
	}
}
