package workspace

import (
	"path/filepath"
	"testing"

	"github.com/ZerGo0/editor/internal/command"
	"github.com/ZerGo0/editor/internal/keymap"
	"github.com/ZerGo0/editor/internal/session"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func TestCodeKeysRunBindings(t *testing.T) {
	env := newTestEnv(t)
	ran := map[string]int{}
	for _, id := range []string{"test.f13", "test.numpad", "test.chord", "test.digit"} {
		env.Commands.Add(&command.Command{ID: id, Title: id, Run: func(*mygo.Window) { ran[id]++ }})
	}
	env.Keys.Current = keymap.New(keymap.Defaults(keymap.PresetVSCode), []keymap.Entry{
		{Key: "f13", Command: "test.f13"},
		{Key: "numpad1", Command: "test.numpad"},
		{Key: "cmd+k numpad_add", Command: "test.chord"},
		{Key: "cmd+1", Command: "test.digit"},
	})
	w := newTestWorkspace(t, t.TempDir(), env, session.Project{})
	down := func(mods ui.Modifiers, key ui.Key, code ui.KeyCode) {
		w.observeCodeKey(ui.InputEvent{Kind: ui.InputKeyDown, Mods: mods, Key: key, Code: code})
		w.runPendingUpdates()
	}

	down(0, ui.KeyUnknown, ui.CodeF13)
	if ran["test.f13"] != 1 || w.keys.swallowText {
		t.Fatalf("F13: ran %v, swallow %v", ran, w.keys.swallowText)
	}
	down(0, ui.Key1, ui.CodeNumpad1)
	if ran["test.numpad"] != 1 || !w.keys.swallowText {
		t.Fatalf("Numpad1: ran %v, swallow %v", ran, w.keys.swallowText)
	}
	w.keys.pending = keymap.Chord{{Mods: ui.Super, Key: ui.KeyK}}
	down(0, ui.KeyEqual, ui.CodeNumpadAdd)
	if ran["test.chord"] != 1 || len(w.keys.pending) != 0 {
		t.Fatalf("Cmd+K Num+: ran %v, pending %v", ran, w.keys.pending)
	}
	down(ui.Super, ui.Key1, ui.CodeNumpad1)
	if ran["test.digit"] != 0 || !w.keys.byCode || w.keys.swallowText {
		t.Fatalf("Cmd+Numpad1: ran %v, byCode %v, swallow %v", ran, w.keys.byCode, w.keys.swallowText)
	}
	down(ui.Super, ui.Key1, ui.CodeDigit1)
	if ran["test.digit"] != 0 || w.keys.swallowText {
		t.Fatalf("Cmd+1 ran through the code path: %v", ran)
	}
}

const (
	blankX = 700
	blankY = 795
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

func TestCodeKeyWithoutWhenRunsWhereverTheFocusIs(t *testing.T) {
	w, tt, ran := codeKeyHarness(t, []string{"test.f20"}, []keymap.Entry{{Key: "f20", Command: "test.f20"}})
	for _, tc := range []struct {
		name  string
		focus func()
	}{
		{"nothing focused without an editor", func() {}},
		{"click on a blank area", func() { tt.ClickAt(blankX, blankY) }},
		{"side bar", w.tree.Focus},
		{"search dialog", w.QuickOpen},
		{"command palette", w.ShowCommands},
	} {
		tc.focus()
		tt.Frame()
		before := ran["test.f20"]
		pressCode(tt, ui.CodeF20)
		if ran["test.f20"] != before+1 {
			t.Errorf("%s: F20 ran %d times, want 1", tc.name, ran["test.f20"]-before)
		}
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
