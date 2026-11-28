package workspace

import (
	"path/filepath"
	"testing"

	"github.com/ZerGo0/editor/internal/command"
	"github.com/ZerGo0/editor/internal/findwidget"
	"github.com/ZerGo0/editor/internal/keymap"
	"github.com/ZerGo0/editor/internal/session"
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

func TestFindOptionsAreSharedAcrossEditorsAndSavedPerProject(t *testing.T) {
	w, tt, _ := codeKeyHarness(t, nil, nil)
	testutil.WriteFiles(t, w.Root, map[string]string{"a.txt": "Cat cat\n", "b.txt": "Cat cat\n"})
	find := func(name string) *editor.State {
		tab, _ := openTab(t, w, filepath.Join(w.Root, name))
		w.focusTab = tab
		tt.Frame()
		tt.Key(ui.Cmd, ui.KeyF)
		tt.Frame()
		tt.Frame()
		return tab.Editor
	}
	a := find("a.txt")
	tt.Key(findwidget.ToggleMods, ui.KeyC)
	tt.Frame()
	tt.Frame()
	if n, _ := a.FindMatches(); n != 1 || !w.search.widget.MatchCase {
		t.Fatalf("%d matches in a.txt, Match Case %v: want 1 match with Match Case on", n, w.search.widget.MatchCase)
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	b := find("b.txt")
	if n, _ := b.FindMatches(); n != 1 {
		t.Fatalf("%d matches in b.txt, want 1: Match Case is not shared", n)
	}
	again := newTestWorkspace(t, w.Root, newTestEnv(t), session.Project{Views: w.views.Snapshot()})
	if sh := again.search.widget; !sh.MatchCase || len(sh.History.Entries()) == 0 {
		t.Fatalf("restored find state %+v, want Match Case on and the query history", sh)
	}
}
