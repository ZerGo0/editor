package workspace

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/ZerGo0/editor/internal/search/finder"
	"github.com/ZerGo0/editor/internal/session"
	"github.com/ZerGo0/editor/internal/testutil"
	"github.com/egoist/mygo/ui"
)

const (
	activityTestWidth  = 600
	activityTestHeight = 120
	searchTooltipKeys  = "⇧⌘F"
)

func activityLabels(entries []activityEntry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.item.Label
	}
	return out
}

func TestActivityBarShowsSearchAfterExplorerWithItsKey(t *testing.T) {
	w := newTestWorkspace(t, t.TempDir(), newTestEnv(t), session.Project{})
	entries, selected := w.activityEntries()
	labels := activityLabels(entries)
	if len(labels) < 2 || labels[0] != "Explorer" || labels[1] != searchActivityLabel {
		t.Fatalf("activity bar %v; want Search right after Explorer", labels)
	}
	if got := entries[1].item.Keys; got != searchTooltipKeys {
		t.Fatalf("Search keys %q, want %q", got, searchTooltipKeys)
	}
	if selected != 0 {
		t.Fatalf("selected %d, want Explorer", selected)
	}
}

func TestActivityBarSearchIsActiveWhileTheDialogShowsTheTextTab(t *testing.T) {
	w := newTestWorkspace(t, t.TempDir(), newTestEnv(t), session.Project{})
	for _, tc := range []struct {
		tab  finder.Tab
		open bool
		want string
	}{
		{finder.TabText, true, searchActivityLabel},
		{finder.TabFiles, true, "Explorer"},
		{finder.TabAll, true, "Explorer"},
		{finder.TabText, false, "Explorer"},
	} {
		w.search.dialog.Close()
		if tc.open {
			w.OpenSearch(tc.tab)
		}
		entries, selected := w.activityEntries()
		if got := entries[selected].item.Label; got != tc.want {
			t.Errorf("tab %v open %v: active %q, want %q", tc.tab, tc.open, got, tc.want)
		}
	}
}

func TestActivityBarSearchOpensTheTextTabOnClick(t *testing.T) {
	w := newTestWorkspace(t, t.TempDir(), newTestEnv(t), session.Project{})
	tt := ui.NewTester(func(c *ui.Context) { w.activityBar(c, nil, false) }, activityTestWidth, activityTestHeight)
	tt.Frame()
	tab, ok := tt.AccessByRole(ui.RoleTab, searchActivityLabel)
	if !ok {
		t.Fatalf("no Search tab among %v", activityLabels(func() []activityEntry { e, _ := w.activityEntries(); return e }()))
	}
	if explorer, _ := tt.AccessByRole(ui.RoleTab, "Explorer"); !(explorer.Bounds.X < tab.Bounds.X) {
		t.Fatal("Search is not placed after Explorer")
	}
	tt.ClickAt(tab.Bounds.X+tab.Bounds.W/2, tab.Bounds.Y+tab.Bounds.H/2)
	if !w.search.dialog.IsOpen() || w.search.dialog.Tab() != finder.TabText {
		t.Fatalf("click did not open Search Everywhere on the Text tab: open %v tab %v", w.search.dialog.IsOpen(), w.search.dialog.Tab())
	}
	tt.Frame()
	if tab, _ = tt.AccessByRole(ui.RoleTab, searchActivityLabel); !tab.Selected {
		t.Fatal("Search is not shown as active while the Text tab shows")
	}
}

func TestActivityBarKeepsSearchWhenExplorerIsHidden(t *testing.T) {
	w := newTestWorkspace(t, t.TempDir(), newTestEnv(t), session.Project{})
	w.setViewHidden(ExplorerView, true)
	entries, selected := w.activityEntries()
	if labels := activityLabels(entries); !slices.Contains(labels, searchActivityLabel) || labels[0] != searchActivityLabel {
		t.Fatalf("activity bar %v; want Search first without Explorer", labels)
	}
	if selected < 0 || selected >= len(entries) {
		t.Fatalf("selected %d out of range", selected)
	}
}

func explorerEntry(t *testing.T, w *Workspace) activityEntry {
	t.Helper()
	entries, _ := w.activityEntries()
	for _, e := range entries {
		if e.view == ExplorerView {
			return e
		}
	}
	t.Fatal("no Explorer entry")
	return activityEntry{}
}

func TestExplorerBadgeCountsUnsavedFilesAndUntitled(t *testing.T) {
	root := t.TempDir()
	testutil.WriteFiles(t, root, map[string]string{"a.txt": "a", "b.txt": "b"})
	w := newTestWorkspace(t, root, newTestEnv(t), session.Project{})
	if e := explorerEntry(t, w); e.item.Badge != 0 || e.item.BadgeTip != "" {
		t.Fatalf("clean workspace shows badge %d %q", e.item.Badge, e.item.BadgeTip)
	}
	_, a := openTab(t, w, filepath.Join(root, "a.txt"))
	_, b := openTab(t, w, filepath.Join(root, "b.txt"))
	a.doc.Buffer.Set("edited")
	if e := explorerEntry(t, w); e.item.Badge != 1 || e.item.BadgeTip != "1 unsaved file" {
		t.Fatalf("one dirty file shows badge %d %q", e.item.Badge, e.item.BadgeTip)
	}
	b.doc.Buffer.Set("edited")
	w.NewUntitled()
	w.entry(w.active.ActiveTab().Path).doc.Buffer.Set("draft")
	if e := explorerEntry(t, w); e.item.Badge != 3 || e.item.BadgeTip != "3 unsaved files" {
		t.Fatalf("two files and an untitled show badge %d %q", e.item.Badge, e.item.BadgeTip)
	}
	w.SaveAll()
	if e := explorerEntry(t, w); e.item.Badge != 1 {
		t.Fatalf("after Save All the untitled draft stays: badge %d", e.item.Badge)
	}
}
