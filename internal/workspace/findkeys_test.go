package workspace

import (
	"path/filepath"
	"testing"

	"github.com/ZerGo0/editor/internal/command"
	"github.com/ZerGo0/editor/internal/keymap"
	"github.com/ZerGo0/editor/internal/testutil"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

func TestFindKeysStepMatchesWhileTheFindInputHasFocus(t *testing.T) {
	w, tt, _ := codeKeyHarness(t, nil, nil)
	for _, cmd := range []editor.Command{editor.FindNext, editor.FindPrevious} {
		w.env.Commands.Add(&command.Command{ID: keymap.EditorCommandID(cmd), Title: cmd.String(), Run: func(*mygo.Window) { w.RunEditor(cmd) }})
	}
	testutil.WriteFiles(t, w.Root, map[string]string{"m.txt": "foo\nfoo\nfoo\nfoo\n"})
	tab, _ := openTab(t, w, filepath.Join(w.Root, "m.txt"))
	w.focusTab = tab
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyF)
	tt.Frame()
	tt.Type("foo")
	tt.Frame()
	press := func(mods ui.Modifiers, key ui.Key) int {
		tt.Key(mods, key)
		tt.Frame()
		w.runPendingUpdates()
		tt.Frame()
		return tab.Editor.CursorPos().Line
	}
	for i, step := range []struct {
		mods ui.Modifiers
		key  ui.Key
		line int
	}{{0, ui.KeyF3, 1}, {0, ui.KeyF3, 2}, {ui.Shift, ui.KeyF3, 1}, {0, ui.KeyEnter, 2}} {
		if got := press(step.mods, step.key); got != step.line {
			t.Fatalf("step %d: cursor line %d, want %d", i, got, step.line)
		}
		if v, ok := focusedText(tt); !ok || v != "foo" {
			t.Fatalf("step %d: focused field %q (%v), want the find input", i, v, ok)
		}
	}
}
