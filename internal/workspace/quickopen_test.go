package workspace

import (
	"context"
	"path/filepath"
	"slices"
	"testing"

	"github.com/ZerGo0/editor/internal/command"
	"github.com/ZerGo0/editor/internal/palette"
	"github.com/ZerGo0/editor/internal/session"
	"github.com/egoist/mygo"
)

func commandResults(w *Workspace, q string) []palette.Item {
	var got []palette.Item
	w.searchCommands(q, func(items []palette.Item) { got = items })
	return got
}

func TestCommandPaletteListsRecentlyUsedFirst(t *testing.T) {
	env := newTestEnv(t)
	noop := func(*mygo.Window) {}
	env.Commands.Add(
		&command.Command{ID: "c", Title: "Close Editor", Category: "View", Run: noop},
		&command.Command{ID: "a", Title: "Add Cursor", Category: "Edit", Run: noop},
		&command.Command{ID: "b", Title: "Build", Category: "Edit", Run: noop},
	)
	w := newTestWorkspace(t, t.TempDir(), env, session.Project{})

	labels, groups := itemLabels(commandResults(w, ""))
	if !slices.Equal(labels, []string{"Edit: Add Cursor", "Edit: Build", "View: Close Editor"}) || slices.ContainsFunc(groups, func(g string) bool { return g != "" }) {
		t.Fatalf("no history: labels %v groups %v", labels, groups)
	}

	w.runFromPalette(palette.Item{ID: "c"})
	w.runFromPalette(palette.Item{ID: "b"})
	labels, groups = itemLabels(commandResults(w, ""))
	if !slices.Equal(labels, []string{"Edit: Build", "View: Close Editor", "Edit: Add Cursor"}) {
		t.Fatalf("labels %v; want recent Build, Close Editor, then Add Cursor", labels)
	}
	if !slices.Equal(groups, []string{recentlyUsedGroup, recentlyUsedGroup, otherCommandsGroup}) {
		t.Fatalf("groups %v", groups)
	}

	labels, groups = itemLabels(commandResults(w, "close"))
	if !slices.Equal(labels, []string{"View: Close Editor"}) || !slices.Equal(groups, []string{recentlyUsedGroup}) {
		t.Fatalf("typed query: labels %v groups %v", labels, groups)
	}
}

func fileResults(w *Workspace, q string) []palette.Item {
	var got []palette.Item
	w.searchFiles(q, func(items []palette.Item) { got = items })
	return got
}

func TestQuickOpenListsRecentlyOpenedFirst(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"a.go": "a\n", "b.go": "b\n", "lib/c.go": "c\n"})
	w := newTestWorkspace(t, root, newTestEnv(t), session.Project{})
	if err := w.search.files.Refresh(context.Background(), func(string) bool { return false }); err != nil {
		t.Fatal(err)
	}
	openTab(t, w, filepath.Join(root, "a.go"))
	openTab(t, w, filepath.Join(root, "b.go"))

	got := fileResults(w, "")
	labels, groups := itemLabels(got)
	if !slices.Equal(labels, []string{"a.go"}) || !slices.Equal(groups, []string{recentlyOpenedGroup}) || !got[0].Removable {
		t.Fatalf("empty query: labels %v groups %v; want a.go only, the active b.go left out", labels, groups)
	}

	labels, groups = itemLabels(fileResults(w, "go"))
	if len(labels) != 3 || labels[2] != "c.go" || !slices.Equal(groups, []string{recentlyOpenedGroup, recentlyOpenedGroup, fileResultsGroup}) {
		t.Fatalf("typed query: labels %v groups %v", labels, groups)
	}

	w.forgetRecent(got[0])
	if labels, _ = itemLabels(fileResults(w, "")); len(labels) != 0 {
		t.Fatalf("removed file still listed: %v", labels)
	}
}

func itemLabels(items []palette.Item) (labels, groups []string) {
	for _, it := range items {
		labels = append(labels, it.Label)
		groups = append(groups, it.Group)
	}
	return labels, groups
}
