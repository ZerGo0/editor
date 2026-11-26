package workspace

import (
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
