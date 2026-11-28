package workspace

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ZerGo0/editor/internal/layout"
	"github.com/ZerGo0/editor/internal/session"
	"github.com/ZerGo0/editor/internal/testutil"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	stateLines    = 80
	stateLine     = 51
	stateFoldLine = 2
	toolViewID    = "tool:demo"
)

func stateText() string {
	return "func a() {\n\tif true {\n\t\tx()\n\t}\n}\n" + strings.Repeat("line\n", stateLines)
}

func stateWorkspace(t *testing.T, saved session.Project) (*Workspace, string) {
	t.Helper()
	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{"a.go": stateText(), "b.go": stateText(), "c.go": "c\n"})
	return newTestWorkspace(t, root, newTestEnv(t), saved), root
}

func TestSnapshotSavesTheActiveFileTabWhenToolTabsAreLeftOut(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		setup func(w *Workspace, root string)
		want  string
	}{
		{"a tool tab opened first", func(w *Workspace, root string) {
			w.OpenView(toolViewID, "Tool", func(c *ui.Context) {})
			w.OpenFile(filepath.Join(root, "a.go"), false, true)
			w.OpenFile(filepath.Join(root, "b.go"), false, true)
			w.active.Active = w.active.Find(filepath.Join(root, "a.go"))
		}, "a.go"},
		{"an active tool tab", func(w *Workspace, root string) {
			w.OpenFile(filepath.Join(root, "a.go"), false, true)
			w.OpenFile(filepath.Join(root, "b.go"), false, true)
			w.OpenView(toolViewID, "Tool", func(c *ui.Context) {})
		}, "b.go"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			w, root := stateWorkspace(t, session.Project{})
			tt.setup(w, root)
			g := w.Snapshot().Layout.Group
			if len(g.Tabs) != 2 || g.Tabs[g.Active].Path != tt.want {
				t.Fatalf("saved tabs %+v with active %d, want %s active", g.Tabs, g.Active, tt.want)
			}
		})
	}
}

func TestReopenedFileKeepsItsCursorFoldsAndSelections(t *testing.T) {
	t.Parallel()
	w, root := stateWorkspace(t, session.Project{})
	p := filepath.Join(root, "a.go")
	tab, e := openTab(t, w, p)
	tab.Editor.Fold(stateFoldLine - 1)
	buf := e.doc.Buffer
	first := buf.Offset(editor.Pos{Line: stateLine, Col: 0})
	second := buf.Offset(editor.Pos{Line: stateLine + 2, Col: 1})
	tab.Editor.SetSelections(editor.Selection{Anchor: first, Head: first + 2}, editor.Selection{Anchor: second, Head: second})
	w.CloseActiveTab()
	if len(w.active.Tabs) != 0 {
		t.Fatal("the tab did not close")
	}
	again, _ := openTab(t, w, p)
	if got := again.Editor.CursorPos().Line; got != stateLine+2 {
		t.Fatalf("reopened at line %d, want %d", got, stateLine+2)
	}
	if sels := again.Editor.Selections(); len(sels) != 2 || sels[0].Start() != first {
		t.Fatalf("selections = %+v", sels)
	}
	if !slices.Contains(again.Editor.Folded(), stateFoldLine-1) {
		t.Fatalf("folds = %v, want line %d folded", again.Editor.Folded(), stateFoldLine-1)
	}
}

func TestClosedViewStateSurvivesARestart(t *testing.T) {
	t.Parallel()
	w, root := stateWorkspace(t, session.Project{})
	p := filepath.Join(root, "b.go")
	tab, _ := openTab(t, w, p)
	tab.Editor.GoTo(editor.Pos{Line: stateLine})
	w.CloseActiveTab()
	saved := w.Snapshot()
	back := newTestWorkspace(t, root, w.env, saved)
	again, _ := openTab(t, back, p)
	if got := again.Editor.CursorPos().Line; got != stateLine {
		t.Fatalf("after the restart b.go opened at line %d, want %d", got, stateLine)
	}
}

func TestOpenTabsKeepFoldsAndSelectionsAcrossARestart(t *testing.T) {
	t.Parallel()
	w, root := stateWorkspace(t, session.Project{})
	tab, e := openTab(t, w, filepath.Join(root, "a.go"))
	tab.Editor.Fold(stateFoldLine - 1)
	at := e.doc.Buffer.Offset(editor.Pos{Line: stateLine})
	tab.Editor.SetSelections(editor.Selection{Anchor: at, Head: at + 3})
	back := newTestWorkspace(t, root, w.env, w.Snapshot())
	got := back.ensureEditorFor(t, back.active.ActiveTab())
	if sels := got.Selections(); len(sels) != 1 || sels[0].Start() != at || sels[0].End() != at+3 {
		t.Fatalf("selections = %+v", sels)
	}
	if !slices.Contains(got.Folded(), stateFoldLine-1) {
		t.Fatalf("folds = %v", got.Folded())
	}
}

func TestRestoredToolTabsComeBackThroughTheirRestorer(t *testing.T) {
	t.Parallel()
	saved := session.Project{Layout: &session.Node{Group: &session.Group{Active: 1, Tabs: []session.Tab{
		{Path: "a.go"}, {Path: virtualPrefix + toolViewID, Title: "Tool"}, {Path: virtualPrefix + "gone:1", Title: "Gone"},
	}}}}
	w, _ := stateWorkspace(t, saved)
	var titles []string
	w.RestoreViews("tool:", func(id, title string) (func(c *ui.Context), bool) {
		titles = append(titles, id+"="+title)
		return func(c *ui.Context) {}, true
	})
	w.dropUnrestoredViews()
	if !slices.Equal(titles, []string{toolViewID + "=Tool"}) {
		t.Fatalf("restorer calls = %v", titles)
	}
	g := w.active
	if len(g.Tabs) != 2 || g.ActiveTab().Path != virtualPrefix+toolViewID {
		t.Fatalf("tabs after restore: %d, active %v", len(g.Tabs), g.ActiveTab())
	}
	if tabs := w.Snapshot().Layout.Group.Tabs; len(tabs) != 2 || tabs[1].Title != "Tool" {
		t.Fatalf("a restorable tool tab was not saved: %+v", tabs)
	}
}

func (w *Workspace) ensureEditorFor(t *testing.T, tab *layout.Tab) *editor.State {
	t.Helper()
	if e := w.ensureEditor(tab); e.err != nil || tab.Editor == nil {
		t.Fatalf("no editor for %s: %v", tab.Path, e.err)
	}
	return tab.Editor
}
