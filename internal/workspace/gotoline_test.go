package workspace

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZerGo0/editor/internal/command"
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/gotoline"
	"github.com/ZerGo0/editor/internal/keymap"
	"github.com/ZerGo0/editor/internal/layout"
	"github.com/ZerGo0/editor/internal/session"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	quickOpenID    = "workbench.action.quickOpen"
	goToLineFile   = "a.txt"
	goToLineLines  = 100
	accentedLine   = 3
	accentedText   = "héllo wörld"
	oldBuiltInName = "Go to line"
	typedMarker    = "Q"
)

type commandStub struct{ run func(id string) }

func (commandStub) Lookup(string) (action.CommandInfo, bool) { return action.CommandInfo{}, true }

func (s commandStub) Run(id string) { s.run(id) }

type goToLineBed struct {
	w   *Workspace
	tt  *ui.Tester
	tab *layout.Tab
}

func newGoToLineBed(t *testing.T) *goToLineBed {
	t.Helper()
	env := newTestEnv(t)
	bed := &goToLineBed{}
	env.Commands.Add(&command.Command{ID: gotoLineID, Title: goToLineLabel, Run: func(*mygo.Window) { bed.w.GoToLine() }})
	root := t.TempDir()
	var text strings.Builder
	for i := 1; i <= goToLineLines; i++ {
		if i == accentedLine {
			text.WriteString(accentedText + "\n")
			continue
		}
		fmt.Fprintf(&text, "line %d\n", i)
	}
	writeFiles(t, root, map[string]string{goToLineFile: text.String(), "b.txt": "b\n"})
	bed.w = newTestWorkspace(t, root, env, session.Project{})
	bed.tt = ui.NewTester(bed.w.View, testWideW, testViewH)
	bed.tt.Frame()
	bed.tt.Frame()
	bed.tab, _ = openTab(t, bed.w, filepath.Join(root, goToLineFile))
	bed.w.focusTab = bed.tab
	bed.settle()
	return bed
}

func (b *goToLineBed) settle() {
	b.tt.Frame()
	b.tt.Frame()
}

func (b *goToLineBed) runCommand(id string) {
	b.w.run(id)
	b.w.runPendingUpdates()
	b.settle()
}

func (b *goToLineBed) type_(s string) {
	b.tt.Type(s)
	b.settle()
}

func (b *goToLineBed) press(mods ui.Modifiers, key ui.Key) {
	b.tt.Key(mods, key)
	b.settle()
}

func (b *goToLineBed) quickInput() (ui.AccessInfo, bool) {
	return b.tt.AccessByRole(ui.RoleDialog, goToLineLabel)
}

func (b *goToLineBed) open(t *testing.T) {
	t.Helper()
	b.runCommand(gotoLineID)
	if _, ok := b.quickInput(); !ok {
		t.Fatal("Go to Line did not open the quick input")
	}
}

func (b *goToLineBed) hasText(s string) bool {
	return b.tt.HasText(s)
}

func (b *goToLineBed) cursor() editor.Pos {
	return b.tab.Editor.CursorPos()
}

func (b *goToLineBed) lineVisible(line int) bool {
	first, last := b.tab.Editor.VisibleLines()
	return first <= line && line <= last
}

func (b *goToLineBed) bufferText() string {
	return b.tab.Editor.Buffer.String()
}

func (b *goToLineBed) builtInBarShown() bool {
	_, ok := b.tt.AccessByRole(ui.RoleDialog, oldBuiltInName)
	return ok
}

func TestGoToLineOpensTheQuickInputFromEveryEntryPoint(t *testing.T) {
	entries := map[string]func(b *goToLineBed){
		"key with the editor focused": func(b *goToLineBed) { b.press(ui.Ctrl, ui.KeyG) },
		"key with the explorer focused": func(b *goToLineBed) {
			b.w.tree.Focus()
			b.settle()
			b.press(ui.Ctrl, ui.KeyG)
		},
		"menu command": func(b *goToLineBed) { b.runCommand(gotoLineID) },
		"status bar item": func(b *goToLineBed) {
			action.SetCommands(commandStub{run: func(id string) { b.w.env.Commands.Run(id, nil) }})
			t.Cleanup(func() { action.SetCommands(nil) })
			for _, a := range b.tt.AccessAll() {
				if a.Description == "Go to Line/Column" {
					b.tt.ClickAt(a.Bounds.X+a.Bounds.W/2, a.Bounds.Y+a.Bounds.H/2)
					b.w.runPendingUpdates()
					b.settle()
					return
				}
			}
		},
		"quick open with the colon prefix": func(b *goToLineBed) {
			id, err := keymap.ResolveCommand(quickOpenID, []byte(`":"`))
			if err != nil {
				panic(err)
			}
			b.runCommand(id)
		},
	}
	for name, open := range entries {
		t.Run(name, func(t *testing.T) {
			b := newGoToLineBed(t)
			open(b)
			if _, ok := b.quickInput(); !ok {
				t.Fatal("the quick input is not open")
			}
			if b.builtInBarShown() {
				t.Fatal("the built-in go to line bar is shown")
			}
			b.type_("40")
			if !strings.HasPrefix(b.bufferText(), "line 1\n") {
				t.Fatalf("the digits went into the document: %q", b.bufferText()[:12])
			}
			b.press(0, ui.KeyEnter)
			if got := b.cursor(); got.Line != 39 {
				t.Fatalf("cursor at %v, want line 40", got)
			}
			if _, ok := b.quickInput(); ok {
				t.Fatal("Enter left the quick input open")
			}
			b.type_(typedMarker)
			if !strings.Contains(b.bufferText(), typedMarker+"line 40\n") {
				t.Fatal("the editor did not get the keyboard back after Enter")
			}
		})
	}
}

func TestGoToLineShowsThePromptAsPlaceholderAndRow(t *testing.T) {
	b := newGoToLineBed(t)
	b.tab.Editor.GoTo(editor.Pos{Line: 6, Col: 2})
	b.open(t)
	want := fmt.Sprintf("Current Line: 7, Character: 3. Type a line number between 1 and %d to navigate to.", goToLineLines+1)
	if !b.hasText(want) {
		t.Fatalf("no prompt %q in %v", want, b.tt.Texts())
	}
}

func TestGoToLinePreviewsWhileTyping(t *testing.T) {
	b := newGoToLineBed(t)
	b.tab.Editor.GoTo(editor.Pos{Line: 5, Col: 1})
	b.settle()
	before, scrollBefore := b.cursor(), b.tab.Editor.ScrollY()
	b.open(t)
	b.type_("80")
	if !b.lineVisible(79) || b.tab.Editor.ScrollY() == scrollBefore {
		t.Fatalf("line 80 is not in view: scroll %v, was %v", b.tab.Editor.ScrollY(), scrollBefore)
	}
	if bg := b.tab.Editor.LineBackgrounds; len(bg) != 1 || bg[0].First != 79 || bg[0].Last != 79 {
		t.Fatalf("highlight %+v, want line 80", bg)
	}
	if b.cursor() != before {
		t.Fatalf("the preview moved the cursor to %v", b.cursor())
	}
	b.type_("1")
	if bg := b.tab.Editor.LineBackgrounds; len(bg) != 0 {
		t.Fatalf("line 801 is out of range but %+v is highlighted", bg)
	}
}

func TestGoToLineEnterKeepsThePositionAndClearsThePreview(t *testing.T) {
	b := newGoToLineBed(t)
	b.open(t)
	b.type_("60")
	b.press(0, ui.KeyEnter)
	if got := b.cursor(); got != (editor.Pos{Line: 59}) {
		t.Fatalf("cursor %v, want line 60 column 1", got)
	}
	if !b.lineVisible(59) {
		t.Fatal("line 60 is not in view")
	}
	if len(b.tab.Editor.LineBackgrounds) != 0 {
		t.Fatalf("highlight %+v stayed after Enter", b.tab.Editor.LineBackgrounds)
	}
}

func TestGoToLineEscapeRestoresSelectionAndScroll(t *testing.T) {
	b := newGoToLineBed(t)
	b.tab.Editor.Select(2, 6)
	b.settle()
	selection, scroll := b.tab.Editor.Selection(), b.tab.Editor.ScrollY()
	b.open(t)
	b.type_("90")
	if b.tab.Editor.ScrollY() == scroll {
		t.Fatal("the preview did not scroll")
	}
	b.press(0, ui.KeyEscape)
	if _, ok := b.quickInput(); ok {
		t.Fatal("Escape left the quick input open")
	}
	if got := b.tab.Editor.Selection(); got != selection {
		t.Fatalf("selection %v, want %v", got, selection)
	}
	if got := b.tab.Editor.ScrollY(); got != scroll {
		t.Fatalf("scroll %v, want %v", got, scroll)
	}
	if len(b.tab.Editor.LineBackgrounds) != 0 {
		t.Fatalf("highlight %+v stayed after Escape", b.tab.Editor.LineBackgrounds)
	}
	b.type_(typedMarker)
	if !strings.HasPrefix(b.bufferText(), "li"+typedMarker+"\nline 2") {
		t.Fatalf("the editor did not get the keyboard back after Escape: %q", b.bufferText()[:16])
	}
}

func TestGoToLineParsesLineAndColumn(t *testing.T) {
	for _, tt := range []struct {
		input string
		want  editor.Pos
	}{
		{"10:4", editor.Pos{Line: 9, Col: 3}},
		{"10,4", editor.Pos{Line: 9, Col: 3}},
		{"3:3", editor.Pos{Line: 2, Col: 3}},
		{"10:99", editor.Pos{Line: 9, Col: 0}},
		{"-2", editor.Pos{Line: goToLineLines - 1}},
		{"-1:2", editor.Pos{Line: goToLineLines, Col: 0}},
		{"+5", editor.Pos{Line: 4}},
	} {
		b := newGoToLineBed(t)
		b.open(t)
		b.type_(tt.input)
		b.press(0, ui.KeyEnter)
		if got := b.cursor(); got != tt.want {
			t.Errorf("%q: cursor %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestGoToLineRejectsInvalidInputLikeVSCode(t *testing.T) {
	for _, input := range []string{"999", "0", "-500", "abc", "1000:1"} {
		b := newGoToLineBed(t)
		b.tab.Editor.GoTo(editor.Pos{Line: 4})
		b.settle()
		start := b.cursor()
		b.open(t)
		b.type_(input)
		prompt := gotoline.Prompt(start.Line+1, start.Col+1, goToLineLines+1)
		if !b.hasText(prompt) {
			t.Errorf("%q: no VS Code message %q in %v", input, prompt, b.tt.Texts())
		}
		if len(b.tab.Editor.LineBackgrounds) != 0 {
			t.Errorf("%q: a line is highlighted", input)
		}
		b.press(0, ui.KeyEnter)
		if _, ok := b.quickInput(); !ok {
			t.Errorf("%q: Enter closed the quick input", input)
		}
		if b.cursor() != start {
			t.Errorf("%q: Enter moved the cursor to %v", input, b.cursor())
		}
		b.press(0, ui.KeyEscape)
	}
}

func TestGoToLineWithoutEditorShowsVSCodeMessage(t *testing.T) {
	b := newGoToLineBed(t)
	b.w.CloseAllEditors()
	b.settle()
	for _, open := range []func(){func() { b.press(ui.Ctrl, ui.KeyG) }, func() { b.runCommand(gotoLineID) }} {
		open()
		if _, ok := b.quickInput(); !ok || !b.hasText(gotoline.NoEditor) {
			t.Fatalf("no message %q in %v", gotoline.NoEditor, b.tt.Texts())
		}
		b.type_("10")
		b.press(0, ui.KeyEnter)
		if _, ok := b.quickInput(); ok {
			t.Fatal("Enter did not close the message")
		}
	}
}

func TestGoToLineTargetsTheActiveSplitEditor(t *testing.T) {
	b := newGoToLineBed(t)
	b.w.Split(false)
	b.settle()
	second := b.w.active.ActiveTab()
	if second == b.tab || second.Editor == nil {
		t.Fatal("the split did not make a second editor")
	}
	b.tab.Editor.GoTo(editor.Pos{Line: 1})
	second.Editor.GoTo(editor.Pos{Line: 2})
	b.w.focusTab = second
	b.settle()
	b.press(ui.Ctrl, ui.KeyG)
	b.type_("50")
	b.press(0, ui.KeyEnter)
	if got := second.Editor.CursorPos().Line; got != 49 {
		t.Fatalf("the focused split is at line %d, want 50", got+1)
	}
	if got := b.tab.Editor.CursorPos().Line; got != 1 {
		t.Fatalf("the other split moved to line %d", got+1)
	}
	if b.w.focusedTab != second {
		t.Fatal("focus did not return to the split")
	}
}

func TestGoToLineKeyInADiffEditorDoesNotOpenTheBuiltInBar(t *testing.T) {
	b := newGoToLineBed(t)
	d := &editor.DiffState{Old: editor.NewBuffer("a\nb\nc\n"), New: editor.NewBuffer("a\nB\nc\nd\n")}
	b.w.ConfigureDiff(d, "x.txt")
	tt := ui.NewTester(func(c *ui.Context) { editor.DiffView(c, d).Fill() }, testWideW, testViewH)
	tt.Frame()
	d.Focus()
	tt.Frame()
	tt.Key(ui.Ctrl, ui.KeyG)
	tt.Frame()
	if _, ok := tt.AccessByRole(ui.RoleDialog, oldBuiltInName); ok {
		t.Fatal("Ctrl+G opened the built-in go to line bar in a diff editor")
	}
}

func TestOpeningAnotherPopupDuringTheGoToLinePreviewRestoresTheView(t *testing.T) {
	b := newGoToLineBed(t)
	scroll := b.tab.Editor.ScrollY()
	b.open(t)
	b.type_("90")
	b.w.QuickOpen()
	b.settle()
	if got := b.tab.Editor.ScrollY(); got != scroll {
		t.Fatalf("scroll %v, want %v", got, scroll)
	}
	if len(b.tab.Editor.LineBackgrounds) != 0 || b.w.goTo != nil {
		t.Fatal("the preview stayed after another popup opened")
	}
}
