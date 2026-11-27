package workspace

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/ZerGo0/editor/internal/command"
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/search/finder"
	"github.com/ZerGo0/editor/internal/session"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func itemLabels(w *Workspace) (recent, other []string) {
	r, o := w.commandItems()
	for _, it := range r {
		recent = append(recent, it.Label)
	}
	for _, it := range o {
		other = append(other, it.Label)
	}
	return recent, other
}

func TestCommandItemsListRecentlyUsedFirst(t *testing.T) {
	env := newTestEnv(t)
	noop := func(*mygo.Window) {}
	env.Commands.Add(
		&command.Command{ID: "c", Title: "Close Editor", Category: "View", Run: noop},
		&command.Command{ID: "a", Title: "Add Cursor", Category: "Edit", Run: noop},
		&command.Command{ID: "b", Title: "Build", Category: "Edit", Run: noop},
	)
	w := newTestWorkspace(t, t.TempDir(), env, session.Project{})

	recent, other := itemLabels(w)
	if len(recent) != 0 || !slices.Equal(other, []string{"Edit: Add Cursor", "Edit: Build", "View: Close Editor"}) {
		t.Fatalf("no history: recent %v other %v", recent, other)
	}
	w.runCommand("c")
	w.runCommand("b")
	recent, other = itemLabels(w)
	if !slices.Equal(recent, []string{"Edit: Build", "View: Close Editor"}) || !slices.Equal(other, []string{"Edit: Add Cursor"}) {
		t.Fatalf("history: recent %v other %v", recent, other)
	}
}

func TestForgetRecentRemovesFile(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"a.go": "a\n", "b.go": "b\n"})
	w := newTestWorkspace(t, root, newTestEnv(t), session.Project{})
	openTab(t, w, filepath.Join(root, "a.go"))
	openTab(t, w, filepath.Join(root, "b.go"))
	w.forgetRecent(filepath.Join(root, "a.go"))
	if !slices.Equal(w.search.recent, []string{filepath.Join(root, "b.go")}) {
		t.Fatalf("recent %v", w.search.recent)
	}
}

func TestSearchEntryPointsOpenTheirTabs(t *testing.T) {
	w := newTestWorkspace(t, t.TempDir(), newTestEnv(t), session.Project{})
	d := w.search.dialog
	for _, tt := range []struct {
		name string
		open func()
		tab  finder.Tab
		seed string
	}{
		{"command palette", w.ShowCommands, finder.TabActions, ""},
		{"go to file", w.QuickOpen, finder.TabFiles, ""},
		{"find in files", w.FindInFiles, finder.TabText, ""},
		{"show search", w.ShowSearch, finder.TabText, ""},
		{"symbols in editor", w.GoToSymbolInEditor, finder.TabFiles, "@"},
		{"symbols in workspace", w.GoToSymbolInWorkspace, finder.TabFiles, "#"},
		{"search everywhere", func() { w.OpenSearch(finder.TabAll) }, finder.TabAll, ""},
	} {
		tt.open()
		if !d.IsOpen() || d.Tab() != tt.tab || (tt.seed != "" && d.Query() != tt.seed) {
			t.Errorf("%s: open %v tab %v query %q; want tab %v", tt.name, d.IsOpen(), d.Tab(), d.Query(), tt.tab)
		}
	}
	w.ReplaceInFiles()
	if !d.IsOpen() || d.Tab() != finder.TabText || !d.Snapshot().ShowReplace {
		t.Fatal("Replace in Files did not open the Text tab with the replace field")
	}
	w.ShowRecentFiles()
	if d.IsOpen() || !w.search.recentFiles.IsOpen() {
		t.Fatal("Recent Files did not replace Search Everywhere")
	}
	w.OpenSearch(finder.TabAll)
	if w.search.recentFiles.IsOpen() || !d.IsOpen() {
		t.Fatal("Search Everywhere did not replace Recent Files")
	}
}

func TestDoubleShiftOpensSearchEverywhere(t *testing.T) {
	w := newTestWorkspace(t, t.TempDir(), newTestEnv(t), session.Project{})
	shift := ui.InputEvent{Kind: ui.InputKeyDown, Code: ui.CodeShiftLeft, Mods: ui.Shift}
	w.observeShift(shift)
	if w.search.dialog.IsOpen() {
		t.Fatal("one Shift opened the dialog")
	}
	w.observeShift(shift)
	if !w.search.dialog.IsOpen() || w.search.dialog.Tab() != finder.TabAll {
		t.Fatal("Double Shift did not open the All tab")
	}
}

func TestToolWindowsListViewsAndPanels(t *testing.T) {
	w := newTestWorkspace(t, t.TempDir(), newTestEnv(t), session.Project{})
	var titles []string
	for _, tw := range w.toolWindows() {
		titles = append(titles, tw.Title)
	}
	for _, want := range []string{"Explorer", "Problems", "TODO", "Find"} {
		if !slices.Contains(titles, want) {
			t.Errorf("tool windows %v lack %s", titles, want)
		}
	}
	if slices.Contains(titles, "Search") {
		t.Error("the Search view is still a tool window")
	}
}

func TestFindResultsOpenTheFindPanel(t *testing.T) {
	w := newTestWorkspace(t, t.TempDir(), newTestEnv(t), session.Project{})
	w.showFindResults(finder.FindResults{Title: "Text matches for \"x\"", Total: 0})
	if !w.tools.panelOpen || w.tools.panel != FindPanel {
		t.Fatalf("panel open %v showing %q", w.tools.panelOpen, w.tools.panel)
	}
}

func TestSearchDialogStateIsSavedWithTheSession(t *testing.T) {
	root := t.TempDir()
	w := newTestWorkspace(t, root, newTestEnv(t), session.Project{})
	w.OpenSearch(finder.TabText)
	w.search.dialog.Close()
	var p session.Project
	w.snapshotSearch(&p)
	again := newTestWorkspace(t, root, newTestEnv(t), p)
	if again.search.dialog.Tab() != finder.TabText {
		t.Fatalf("restored tab %v", again.search.dialog.Tab())
	}
}

type searchCommands struct{ run func() }

func (s searchCommands) Lookup(id string) (action.CommandInfo, bool) {
	return action.CommandInfo{Title: "Search Everywhere", Keys: "Double ⇧"}, id == searchID
}

func (s searchCommands) Run(id string) {
	if id == searchID {
		s.run()
	}
}

func TestTitleBarCommandCenterOpensSearchEverywhere(t *testing.T) {
	w := newTestWorkspace(t, t.TempDir(), newTestEnv(t), session.Project{})
	action.SetCommands(searchCommands{run: func() { w.OpenSearch(finder.TabAll) }})
	t.Cleanup(func() { action.SetCommands(nil) })
	tt := ui.NewTester(w.View, testWideW, testViewH)
	tt.Frame()
	if err := tt.Click("Search Everywhere"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !w.search.dialog.IsOpen() || w.search.dialog.Tab() != finder.TabAll {
		t.Fatal("the command center did not open the All tab")
	}
}
