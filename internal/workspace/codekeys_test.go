package workspace

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZerGo0/editor/internal/command"
	"github.com/ZerGo0/editor/internal/keymap"
	"github.com/ZerGo0/editor/internal/session"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	blankX               = 700
	blankY               = 795
	editorX              = 700
	editorY              = 300
	hoverOffset          = 5
	firstWhenFunctionKey = 14
	logicalCommand       = "test.logical"
	codeCommand          = "test.code"
)

func codeKeyHarness(t *testing.T, ids []string, entries []keymap.Entry) (*Workspace, *ui.Tester, map[string]int) {
	t.Helper()
	env := newTestEnv(t)
	ran := map[string]int{}
	for _, id := range ids {
		env.Commands.Add(&command.Command{ID: id, Title: id, Run: func(*mygo.Window) { ran[id]++ }})
	}
	env.Keys.Current = keymap.New(keymap.Defaults(keymap.PresetVSCode), entries)
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"a.go": "package a\n"})
	w := newTestWorkspace(t, root, env, session.Project{})
	tt := ui.NewTester(w.View, 1200, 800)
	tt.Frame()
	tt.Frame()
	return w, tt, ran
}

func pressCode(tt *ui.Tester, code ui.KeyCode) {
	tt.CodeDown(0, code)
	tt.CodeUp(0, code)
}

func focusedText(tt *ui.Tester) (string, bool) {
	for _, a := range tt.AccessAll() {
		if a.Focused && a.Role == ui.RoleTextField {
			return a.Value, true
		}
	}
	return "", false
}

type keyContext struct {
	name string
	when string
	open func(w *Workspace, tt *ui.Tester)
}

func keyContexts() []keyContext {
	return []keyContext{
		{"editor", "editorTextFocus && !inQuickOpen", func(w *Workspace, tt *ui.Tester) { tt.ClickAt(editorX, editorY) }},
		{"explorer", "filesExplorerFocus", func(w *Workspace, tt *ui.Tester) { w.tree.Focus() }},
		{"panel", "panelFocus", func(w *Workspace, tt *ui.Tester) {
			w.ShowPanel(FindPanel)
			w.focusPart(partPanel)
		}},
		{"command palette", "inQuickOpen", func(w *Workspace, tt *ui.Tester) { w.showSwitcher() }},
		{"go to line", "inQuickOpen", func(w *Workspace, tt *ui.Tester) { w.GoToLine() }},
		{"search everywhere", "inQuickOpen", func(w *Workspace, tt *ui.Tester) { w.QuickOpen() }},
		{"recent files", "inQuickOpen", func(w *Workspace, tt *ui.Tester) { w.ShowRecentFiles() }},
		{"find widget", "textInputFocus && !inQuickOpen", func(w *Workspace, tt *ui.Tester) {
			tt.ClickAt(editorX, editorY)
			tt.Frame()
			tt.Key(ui.Cmd, ui.KeyF)
		}},
	}
}

func TestCodeKeyRunsOnceInEveryContextLikeALogicalKey(t *testing.T) {
	contexts := keyContexts()
	ids := []string{codeCommand, logicalCommand}
	entries := []keymap.Entry{{Key: "f13", Command: codeCommand}, {Key: "ctrl+alt+f9", Command: logicalCommand}}
	whenKeys := make([]ui.KeyCode, len(contexts))
	for i, kc := range contexts {
		id := "test.when." + kc.name
		whenKeys[i] = ui.CodeF14 + ui.KeyCode(i)
		ids = append(ids, id)
		entries = append(entries, keymap.Entry{Key: fmt.Sprintf("f%d", firstWhenFunctionKey+i), Command: id, When: kc.when})
	}
	w, tt, ran := codeKeyHarness(t, ids, entries)
	openTab(t, w, filepath.Join(w.Root, "a.go"))
	tt.Frame()
	for i, kc := range contexts {
		w.closeSearchPopups()
		w.palette.Close()
		tt.Key(0, ui.KeyEscape)
		tt.Frame()
		kc.open(w, tt)
		tt.Frame()
		tt.Frame()
		code, logical, when := ran[codeCommand], ran[logicalCommand], ran["test.when."+kc.name]
		pressCode(tt, ui.CodeF13)
		tt.Key(ui.Ctrl|ui.Alt, ui.KeyF9)
		pressCode(tt, whenKeys[i])
		if got := ran[codeCommand] - code; got != 1 {
			t.Errorf("%s: F13 ran %d times, want 1", kc.name, got)
		}
		if got := ran[logicalCommand] - logical; got != 1 {
			t.Errorf("%s: the logical key ran %d times, want 1", kc.name, got)
		}
		if got := ran["test.when."+kc.name] - when; got != 1 {
			t.Errorf("%s: its when-clause key with when %q ran %d times, want 1", kc.name, kc.when, got)
		}
	}
}

func TestCodeKeyWaitsBehindAModalLikeALogicalKey(t *testing.T) {
	entries := []keymap.Entry{{Key: "f13", Command: codeCommand}, {Key: "ctrl+alt+f9", Command: logicalCommand}, {Key: "numpad1", Command: "test.numpad"}}
	w, tt, ran := codeKeyHarness(t, []string{codeCommand, logicalCommand, "test.numpad"}, entries)
	w.ask("Rename", "Rename", "x", func(string) error { return nil })
	tt.Frame()
	tt.Frame()
	pressCode(tt, ui.CodeF13)
	tt.Key(ui.Ctrl|ui.Alt, ui.KeyF9)
	if ran[codeCommand] != 0 || ran[logicalCommand] != 0 {
		t.Fatalf("behind the dialog: code ran %d, logical ran %d; want 0 and 0", ran[codeCommand], ran[logicalCommand])
	}
	tt.CodeDown(0, ui.CodeNumpad1)
	tt.Type("1")
	tt.CodeUp(0, ui.CodeNumpad1)
	if text, ok := focusedText(tt); !ok || text != "x1" {
		t.Fatalf("the dialog's field holds %q (found %v); a key that is not bound there types as usual", text, ok)
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	tt.Frame()
	pressCode(tt, ui.CodeF13)
	if ran[codeCommand] != 1 {
		t.Fatalf("after the dialog closed: code ran %d times, want 1", ran[codeCommand])
	}
}

func TestCodeKeyDoesNotRunWhileTheKeybindingEditorRecordsIt(t *testing.T) {
	w, tt, ran := codeKeyHarness(t, []string{codeCommand, "test.target"}, []keymap.Entry{{Key: "f13", Command: codeCommand}})
	w.OpenKeybindings()
	tt.Frame()
	tt.Frame()
	row, _ := tt.Find("test.target")
	tt.Move(row.X+hoverOffset, row.Y+hoverOffset)
	tt.Frame()
	if err := tt.Click("Add keybinding of test.target"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	pressCode(tt, ui.CodeF13)
	tt.Frame()
	if ran[codeCommand] != 0 {
		t.Fatalf("recording F13 ran its command %d times", ran[codeCommand])
	}
	if !tt.HasText("F13") {
		t.Fatal("the recorder does not show F13")
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	b, ok := w.env.Keys.Current.Lookup(keymap.Chord{{Code: ui.CodeF13}})
	if !ok || b.Command != "test.target" {
		t.Fatalf("F13 resolves to %+v after recording, want test.target", b)
	}
}

func TestCodeKeyTypesNothingWhereItIsBound(t *testing.T) {
	entries := []keymap.Entry{
		{Key: "numpad1", Command: codeCommand},
		{Key: "numpad3", Command: logicalCommand, When: "!inputFocus"},
	}
	w, tt, ran := codeKeyHarness(t, []string{codeCommand, logicalCommand}, entries)
	tab, _ := openTab(t, w, filepath.Join(w.Root, "a.go"))
	for _, name := range []string{"search everywhere", "go to line", "recent files", "command palette"} {
		for _, kc := range keyContexts() {
			if kc.name != name {
				continue
			}
			tt.Key(0, ui.KeyEscape)
			w.closeSearchPopups()
			w.palette.Close()
			kc.open(w, tt)
			tt.Frame()
			tt.Frame()
			before := ran[codeCommand]
			tt.CodeDown(0, ui.CodeNumpad1)
			tt.Type("1")
			tt.CodeUp(0, ui.CodeNumpad1)
			if text, ok := focusedText(tt); !ok || text != "" {
				t.Errorf("%s: the bound numpad key typed %q (field found %v)", name, text, ok)
			}
			if ran[codeCommand] != before+1 {
				t.Errorf("%s: the bound numpad key ran %d times, want 1", name, ran[codeCommand]-before)
			}
			tt.CodeDown(0, ui.CodeNumpad3)
			tt.Type("3")
			tt.CodeUp(0, ui.CodeNumpad3)
			if text, _ := focusedText(tt); text != "3" || ran[logicalCommand] != 0 {
				t.Errorf("%s: numpad3 is bound only outside inputs; field %q, ran %d", name, text, ran[logicalCommand])
			}
		}
	}
	tt.Key(0, ui.KeyEscape)
	w.closeSearchPopups()
	w.palette.Close()
	tt.ClickAt(editorX, editorY)
	tt.Frame()
	tt.Frame()
	before := tab.Editor.Buffer.String()
	tt.CodeDown(0, ui.CodeNumpad1)
	tt.Type("1")
	tt.CodeUp(0, ui.CodeNumpad1)
	if got := tab.Editor.Buffer.String(); got != before {
		t.Errorf("the editor typed the bound numpad key: %q", got)
	}
}

func TestCodeKeyChordsMixCodeAndLogicalStrokes(t *testing.T) {
	entries := []keymap.Entry{
		{Key: "cmd+k numpad_add", Command: "test.codeSecond"},
		{Key: "numpad_subtract cmd+k", Command: "test.codeFirst"},
		{Key: "numpad_add", Command: "test.single"},
	}
	for _, kc := range keyContexts()[:3] {
		w, tt, ran := codeKeyHarness(t, []string{"test.codeSecond", "test.codeFirst", "test.single"}, entries)
		openTab(t, w, filepath.Join(w.Root, "a.go"))
		tt.Frame()
		kc.open(w, tt)
		tt.Frame()
		tt.Frame()
		tt.Key(ui.Cmd, ui.KeyK)
		pressCode(tt, ui.CodeNumpadAdd)
		if ran["test.codeSecond"] != 1 || ran["test.single"] != 0 || len(w.keys.pending) != 0 {
			t.Errorf("%s: cmd+k numpad_add ran %v, pending %v", kc.name, ran, w.keys.pending)
		}
		pressCode(tt, ui.CodeNumpadSubtract)
		tt.Key(ui.Cmd, ui.KeyK)
		if ran["test.codeFirst"] != 1 || len(w.keys.pending) != 0 {
			t.Errorf("%s: numpad_subtract cmd+k ran %v, pending %v", kc.name, ran, w.keys.pending)
		}
		pressCode(tt, ui.CodeNumpadAdd)
		if ran["test.single"] != 1 {
			t.Errorf("%s: numpad_add alone ran %v", kc.name, ran)
		}
	}
}

func TestCodeKeyTakenByTheEditorChordRunsOnce(t *testing.T) {
	entries := []keymap.Entry{{Key: "cmd+k numpad_add", Command: "test.chord"}, {Key: "numpad_add", Command: "test.single"}}
	w, tt, ran := codeKeyHarness(t, []string{"test.chord", "test.single"}, entries)
	openTab(t, w, filepath.Join(w.Root, "a.go"))
	tt.ClickAt(editorX, editorY)
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyK)
	ev := ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyEqual, Code: ui.CodeNumpadAdd}
	w.observeKey(ev)
	if !w.editorInput(ev) {
		t.Fatal("the editor did not take the second stroke of the chord")
	}
	tt.Frame()
	if ran["test.chord"] != 1 || ran["test.single"] != 0 {
		t.Fatalf("chord taken by the editor ran %v", ran)
	}
}

func TestCodeKeyFollowsTheLiveWhenContext(t *testing.T) {
	entries := []keymap.Entry{
		{Key: "f15", Command: "test.pin", When: "!activeEditorIsPinned"},
		{Key: "f15", Command: "test.unpin", When: "activeEditorIsPinned"},
		{Key: "f13", Command: "test.editorOnly", When: "editorTextFocus"},
		{Key: "f14", Command: "test.outsideEditor", When: "!editorTextFocus"},
	}
	w, tt, ran := codeKeyHarness(t, []string{"test.pin", "test.unpin", "test.editorOnly", "test.outsideEditor"}, entries)
	openTab(t, w, filepath.Join(w.Root, "a.go"))
	tt.Frame()
	w.tree.Focus()
	tt.Frame()

	pressCode(tt, ui.CodeF15)
	w.active.ActiveTab().Pinned = true
	tt.Frame()
	pressCode(tt, ui.CodeF15)
	pressCode(tt, ui.CodeF13)
	pressCode(tt, ui.CodeF14)
	if ran["test.pin"] != 1 || ran["test.unpin"] != 1 {
		t.Errorf("F15 ran pin %d, unpin %d; want 1 and 1", ran["test.pin"], ran["test.unpin"])
	}
	if ran["test.editorOnly"] != 0 || ran["test.outsideEditor"] != 1 {
		t.Errorf("focus in the side bar: editor-only %d, outside-editor %d; want 0 and 1", ran["test.editorOnly"], ran["test.outsideEditor"])
	}
}

func TestCodeKeyWithModifiersMatchesOnlyItsOwnStroke(t *testing.T) {
	entries := []keymap.Entry{{Key: "numpad1", Command: "test.numpad"}, {Key: "cmd+1", Command: "test.digit"}}
	w, tt, ran := codeKeyHarness(t, []string{"test.numpad", "test.digit"}, entries)
	openTab(t, w, filepath.Join(w.Root, "a.go"))
	tt.ClickAt(editorX, editorY)
	tt.Frame()
	tt.CodeDown(ui.Super, ui.CodeNumpad1)
	tt.CodeUp(ui.Super, ui.CodeNumpad1)
	tt.Key(ui.Super, ui.Key1)
	if ran["test.numpad"] != 0 || ran["test.digit"] != 1 {
		t.Fatalf("Cmd+Numpad1 and Cmd+1 ran %v; want only Cmd+1 to run test.digit", ran)
	}
}

func TestCodeKeyLabelsShowInPaletteRowsAndShortcutHints(t *testing.T) {
	w, _, _ := codeKeyHarness(t, []string{codeCommand}, []keymap.Entry{{Key: "cmd+numpad_add", Command: codeCommand}})
	if got := w.keyLabel(codeCommand); got != "⌘Num+" {
		t.Fatalf("shortcut hint = %q", got)
	}
	recent, other := w.commandItems()
	for _, it := range append(recent, other...) {
		if it.ID == codeCommand {
			if it.Keys != w.keyLabel(codeCommand) || it.Keys == "" {
				t.Fatalf("palette row keys = %q, hint = %q", it.Keys, w.keyLabel(codeCommand))
			}
			return
		}
	}
	t.Fatal("the palette lists no row for the command")
}

func TestCodeKeyMenuItemShowsItsKeyWithoutClaimingAShortcut(t *testing.T) {
	w, _, ran := codeKeyHarness(t, []string{codeCommand}, []keymap.Entry{{Key: "f13", Command: codeCommand}})
	open := true
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Button(c, "menu").PopoverMenuButton(func(m *ui.Menu) { w.commandItem(m, "Run It", codeCommand) }, ui.PopoverMenuButtonOptions{Open: &open})
	}, testWideW, testViewH)
	tt.Frame()
	tt.Frame()
	if !tt.HasText("F13") {
		t.Fatalf("the menu item does not show its key; texts %v", tt.Texts())
	}
	tt.Key(0, ui.KeyUnknown)
	if ran[codeCommand] != 0 {
		t.Fatal("a menu item with a code key took an unrelated key")
	}
}

func TestCodeKeyBoundToAnEditorCommandRuns(t *testing.T) {
	const editorCommand = "cursorHome"
	if _, ok := keymap.EditorCommand(editorCommand); !ok {
		t.Fatalf("%s is not an editor command", editorCommand)
	}
	w, tt, ran := codeKeyHarness(t, []string{editorCommand}, []keymap.Entry{{Key: "f13", Command: editorCommand}})
	openTab(t, w, filepath.Join(w.Root, "a.go"))
	tt.ClickAt(editorX, editorY)
	tt.Frame()
	pressCode(tt, ui.CodeF13)
	if ran[editorCommand] != 1 {
		t.Fatalf("F13 bound to %s ran %d times, want 1", editorCommand, ran[editorCommand])
	}
}

func TestBoundKeyRunsOrTypesButNeverBoth(t *testing.T) {
	w, tt, ran := codeKeyHarness(t, []string{logicalCommand}, []keymap.Entry{{Key: "alt+j", Command: logicalCommand}})
	tab, _ := openTab(t, w, filepath.Join(w.Root, "a.go"))
	for _, name := range []string{"search everywhere", "go to line", "recent files", "command palette", "find widget"} {
		for _, kc := range keyContexts() {
			if kc.name != name {
				continue
			}
			tt.Key(0, ui.KeyEscape)
			w.closeSearchPopups()
			w.palette.Close()
			kc.open(w, tt)
			tt.Frame()
			tt.Frame()
			before := ran[logicalCommand]
			start, _ := focusedText(tt)
			tt.TypeKey(ui.Alt, ui.KeyJ, "∆")
			tt.Frame()
			w.runPendingUpdates()
			text, _ := focusedText(tt)
			typed, runs := text == start+"∆", ran[logicalCommand]-before
			if typed == (runs == 1) || runs > 1 {
				t.Errorf("%s: Alt+J typed %v (field %q) and ran %d times; want one of them", name, typed, text, runs)
			}
		}
	}
	tt.Key(0, ui.KeyEscape)
	w.closeSearchPopups()
	w.palette.Close()
	tt.ClickAt(editorX, editorY)
	tt.Frame()
	tt.Frame()
	before, ranBefore := tab.Editor.Buffer.String(), ran[logicalCommand]
	tt.TypeKey(ui.Alt, ui.KeyJ, "∆")
	tt.Frame()
	w.runPendingUpdates()
	if got := tab.Editor.Buffer.String(); got != before || ran[logicalCommand] != ranBefore+1 {
		t.Errorf("the editor typed the bound Alt+J (ran %d): %q", ran[logicalCommand]-ranBefore, got)
	}
	tt.TypeKey(ui.Alt, ui.KeyL, "¬")
	tt.Frame()
	if got := tab.Editor.Buffer.String(); !strings.Contains(got, "¬") {
		t.Errorf("an unbound Alt+L must still type its character, got %q", got)
	}
}
