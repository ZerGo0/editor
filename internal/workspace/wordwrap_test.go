package workspace

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZerGo0/editor/internal/command"
	"github.com/ZerGo0/editor/internal/session"
	"github.com/ZerGo0/editor/internal/settings"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	toggleWordWrapID = "editor.action.toggleWordWrap"
	wrapFile         = "long.txt"
	wrapLineRepeats  = 200
	wrapFramesAfter  = 5
)

var wrapText = strings.Repeat("word ", wrapLineRepeats) + "\nshort\nmore\nend\n"

func newWrapWorkspace(t *testing.T, set map[string]any) *Workspace {
	t.Helper()
	env := newTestEnv(t)
	for key, value := range set {
		if err := env.Settings.Set(key, value); err != nil {
			t.Fatal(err)
		}
	}
	var w *Workspace
	env.Commands.Add(&command.Command{ID: toggleWordWrapID, Title: "Toggle Word Wrap", Run: func(*mygo.Window) { w.RunEditor(editor.ToggleSoftWrap) }})
	root := t.TempDir()
	writeFiles(t, root, map[string]string{wrapFile: wrapText})
	w = newTestWorkspace(t, root, env, session.Project{})
	return w
}

func frames(tt *ui.Tester, n int) {
	for range n {
		tt.Frame()
	}
}

func editorWraps(st *editor.State) bool {
	return st.LineTop(1)-st.LineTop(0) > st.LineTop(2)-st.LineTop(1)
}

func diffWraps(d *editor.DiffState) bool {
	sides := []editor.DiffSide{editor.DiffNew}
	if d.Layout == editor.DiffSplit {
		sides = append(sides, editor.DiffOld)
	}
	for _, s := range sides {
		if d.LineTop(s, 1)-d.LineTop(s, 0) <= d.LineTop(s, 2)-d.LineTop(s, 1) {
			return false
		}
	}
	return true
}

func openWrapEditor(t *testing.T, set map[string]any) (*Workspace, *ui.Tester, *editor.State) {
	t.Helper()
	w := newWrapWorkspace(t, set)
	tt := ui.NewTester(w.View, testWideW, testViewH)
	frames(tt, 2)
	tab, _ := openTab(t, w, filepath.Join(w.Root, wrapFile))
	w.focusTab = tab
	frames(tt, wrapFramesAfter)
	return w, tt, tab.Editor
}

func newWrapDiff(w *Workspace, layout editor.DiffLayout, fit bool) (*editor.DiffState, *ui.Tester) {
	d := &editor.DiffState{Old: editor.NewBuffer(wrapText), New: editor.NewBuffer(strings.Replace(wrapText, "end", "END", 1)), Layout: layout, FitContent: fit}
	tt := ui.NewTester(func(c *ui.Context) {
		w.shortcuts(c)
		w.ConfigureDiff(d, wrapFile)
		editor.DiffView(c, d).Fill()
	}, testWideW, testViewH)
	frames(tt, wrapFramesAfter)
	return d, tt
}

func TestEditorWrapFollowsTheSetting(t *testing.T) {
	t.Parallel()
	for mode, want := range map[string]bool{settings.WrapOff: false, settings.WrapOn: true, settings.WrapColumn: true, settings.WrapBounded: true} {
		_, _, st := openWrapEditor(t, map[string]any{settings.WordWrap: mode})
		if got := editorWraps(st); got != want {
			t.Errorf("editor.wordWrap %q wraps %v, want %v", mode, got, want)
		}
	}
}

func TestWrapColumnWrapsNarrowerThanTheEditor(t *testing.T) {
	t.Parallel()
	_, _, wide := openWrapEditor(t, map[string]any{settings.WordWrap: settings.WrapOn})
	_, _, narrow := openWrapEditor(t, map[string]any{settings.WordWrap: settings.WrapBounded, settings.WordWrapColumn: 20})
	if narrow.LineTop(1) <= wide.LineTop(1) {
		t.Fatalf("bounded at 20 columns is %v high, wrapping at the width is %v", narrow.LineTop(1), wide.LineTop(1))
	}
}

func TestAltZTogglesTheEditorAndStaysToggled(t *testing.T) {
	t.Parallel()
	for _, start := range []string{settings.WrapOff, settings.WrapOn} {
		w, tt, st := openWrapEditor(t, map[string]any{settings.WordWrap: start})
		before := editorWraps(st)
		tt.Key(ui.Alt, ui.KeyZ)
		w.runPendingUpdates()
		frames(tt, wrapFramesAfter)
		if got := editorWraps(st); got == before {
			t.Fatalf("Alt+Z left wrap at %v from %q", got, start)
		}
		frames(tt, wrapFramesAfter)
		if got := editorWraps(st); got == before {
			t.Fatalf("wrap went back to %v from %q on later frames", got, start)
		}
		tt.Key(ui.Alt, ui.KeyZ)
		w.runPendingUpdates()
		frames(tt, wrapFramesAfter)
		if got := editorWraps(st); got != before {
			t.Fatalf("a second Alt+Z left wrap at %v from %q", got, start)
		}
	}
}

func TestDiffWrapFollowsTheSettings(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		set  map[string]any
		want bool
	}{
		{"inherits off", map[string]any{}, false},
		{"inherits on", map[string]any{settings.WordWrap: settings.WrapOn}, true},
		{"diff on over editor off", map[string]any{settings.DiffWordWrap: settings.WrapOn}, true},
		{"diff off over editor on", map[string]any{settings.WordWrap: settings.WrapOn, settings.DiffWordWrap: settings.WrapOff}, false},
	}
	layouts := map[string]editor.DiffLayout{"split": editor.DiffSplit, "unified": editor.DiffUnified}
	for _, c := range cases {
		for name, layout := range layouts {
			for _, fit := range []bool{false, true} {
				d, _ := newWrapDiff(newWrapWorkspace(t, c.set), layout, fit)
				if got := diffWraps(d); got != c.want {
					t.Errorf("%s, %s, fit %v: wraps %v, want %v", c.name, name, fit, got, c.want)
				}
			}
		}
	}
}

func TestAltZTogglesADiffAndStaysToggled(t *testing.T) {
	t.Parallel()
	layouts := map[string]editor.DiffLayout{"split": editor.DiffSplit, "unified": editor.DiffUnified}
	for name, layout := range layouts {
		for _, fit := range []bool{false, true} {
			w := newWrapWorkspace(t, nil)
			d, tt := newWrapDiff(w, layout, fit)
			d.Focus()
			frames(tt, wrapFramesAfter)
			if diffWraps(d) {
				t.Fatalf("%s fit %v wraps before the toggle", name, fit)
			}
			tt.Key(ui.Alt, ui.KeyZ)
			w.runPendingUpdates()
			frames(tt, wrapFramesAfter)
			if !diffWraps(d) {
				t.Fatalf("%s fit %v: Alt+Z did not wrap the diff", name, fit)
			}
			frames(tt, wrapFramesAfter)
			if !diffWraps(d) {
				t.Fatalf("%s fit %v: the diff unwrapped on later frames", name, fit)
			}
		}
	}
}
