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
	otherWrapFile    = "other.txt"
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
	writeFiles(t, root, map[string]string{wrapFile: wrapText, otherWrapFile: wrapText})
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
	return newWrapDiffOf(w, wrapFile, layout, fit)
}

func newWrapDiffOf(w *Workspace, file string, layout editor.DiffLayout, fit bool) (*editor.DiffState, *ui.Tester) {
	d := &editor.DiffState{Old: editor.NewBuffer(wrapText), New: editor.NewBuffer(strings.Replace(wrapText, "end", "END", 1)), Layout: layout, FitContent: fit}
	tt := ui.NewTester(func(c *ui.Context) {
		w.shortcuts(c, scopeMain)
		w.ConfigureDiff(d, filepath.Join(w.Root, file))
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

func toggleWrap(w *Workspace, tt *ui.Tester) {
	tt.Key(ui.Alt, ui.KeyZ)
	w.runPendingUpdates()
	frames(tt, wrapFramesAfter)
}

func TestAltZWrapsOnlyTheFocusedFile(t *testing.T) {
	t.Parallel()
	w, tt, first := openWrapEditor(t, nil)
	toggleWrap(w, tt)
	if !editorWraps(first) {
		t.Fatal("Alt+Z did not wrap the focused file")
	}
	second, _ := openTab(t, w, filepath.Join(w.Root, otherWrapFile))
	w.focusTab = second
	frames(tt, wrapFramesAfter)
	if editorWraps(second.Editor) {
		t.Fatal("Alt+Z in one file wrapped another file")
	}
	toggleWrap(w, tt)
	if !editorWraps(second.Editor) {
		t.Fatal("Alt+Z did not wrap the second file")
	}
	w.OpenFile(filepath.Join(w.Root, wrapFile), false, true)
	frames(tt, wrapFramesAfter)
	if !editorWraps(first) {
		t.Fatal("the first file lost its wrap override")
	}
}

func TestWrapOverrideFollowsTheFileIntoADiffAndEndsWhenItCloses(t *testing.T) {
	t.Parallel()
	w, tt, st := openWrapEditor(t, nil)
	toggleWrap(w, tt)
	if !editorWraps(st) {
		t.Fatal("Alt+Z did not wrap the file")
	}
	same, _ := newWrapDiffOf(w, wrapFile, editor.DiffSplit, false)
	other, _ := newWrapDiffOf(w, otherWrapFile, editor.DiffSplit, false)
	if !diffWraps(same) {
		t.Error("a diff of the toggled file does not wrap on both sides")
	}
	if diffWraps(other) {
		t.Error("a diff of another file wraps")
	}
	w.CloseActiveTab()
	w.dropUnused()
	if _, ok := w.wrapOverrides[filepath.Join(w.Root, wrapFile)]; ok {
		t.Fatal("the override outlived the closed file")
	}
}

func TestAltZInADiffWrapsOnlyThatFile(t *testing.T) {
	t.Parallel()
	w := newWrapWorkspace(t, nil)
	d, tt := newWrapDiffOf(w, wrapFile, editor.DiffSplit, false)
	other, _ := newWrapDiffOf(w, otherWrapFile, editor.DiffSplit, false)
	d.Focus()
	frames(tt, wrapFramesAfter)
	toggleWrap(w, tt)
	if !diffWraps(d) {
		t.Fatal("Alt+Z did not wrap both sides of the diff")
	}
	if diffWraps(other) {
		t.Fatal("Alt+Z wrapped a diff of another file")
	}
}

func TestViewStateFollowsTheFocusedFileAndIsPublishedOnChange(t *testing.T) {
	t.Parallel()
	w, tt, _ := openWrapEditor(t, nil)
	var published []ViewState
	w.OnViewState = func(s ViewState) { published = append(published, s) }
	toggleWrap(w, tt)
	w.publishViewState()
	w.publishViewState()
	if !w.ViewState().WordWrap || len(published) != 1 || !published[0].WordWrap {
		t.Fatalf("after Alt+Z state %v, published %v", w.ViewState(), published)
	}
	openTab(t, w, filepath.Join(w.Root, otherWrapFile))
	w.publishViewState()
	if w.ViewState().WordWrap || len(published) != 2 || published[1].WordWrap {
		t.Fatalf("on the other file state %v, published %v", w.ViewState(), published)
	}
}
