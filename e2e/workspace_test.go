//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func openProject(t *testing.T, files map[string]string) (*App, int, string) {
	t.Helper()
	root := Project(t, files)
	app := Launch(t, root)
	win := app.WaitWindow(filepath.Base(root))
	app.Idle(win)
	return app, win, root
}

func countRole(nodes []Node, role string) int {
	n := 0
	for _, node := range nodes {
		if node.Role == role {
			n++
		}
	}
	return n
}

func TestProjectFromArgsShowsFileTree(t *testing.T) {
	app, win, _ := openProject(t, map[string]string{"main.go": "package main\n", "pkg/util.go": "package pkg\n"})
	app.WaitNode(win, "main.go row", RoleLabel("treeItem", "main.go"))
	dir := app.WaitNode(win, "pkg row", RoleLabel("treeItem", "pkg"))
	if !dir.Has("expandable") {
		t.Fatalf("pkg is not expandable: %v", dir.States)
	}
	app.WaitNode(win, "placeholder", Label("No open files"))
}

func TestSingleClickReplacesPreviewTab(t *testing.T) {
	app, win, _ := openProject(t, map[string]string{"a.txt": "alpha\n", "b.txt": "beta\n"})
	app.Click(win, app.WaitNode(win, "a.txt row", RoleLabel("treeItem", "a.txt")), 1)
	app.WaitNode(win, "a.txt tab", RoleLabel("tab", "a.txt"))
	app.Click(win, app.WaitNode(win, "b.txt row", RoleLabel("treeItem", "b.txt")), 1)
	app.WaitNode(win, "b.txt tab", RoleLabel("tab", "b.txt"))
	if got := countRole(app.Tree(win), "tab"); got != 1 {
		t.Fatalf("tabs = %d, want 1 preview tab", got)
	}
}

func TestDoubleClickPinsTab(t *testing.T) {
	app, win, _ := openProject(t, map[string]string{"a.txt": "alpha\n", "b.txt": "beta\n"})
	app.Click(win, app.WaitNode(win, "a.txt row", RoleLabel("treeItem", "a.txt")), 2)
	app.WaitNode(win, "a.txt tab", RoleLabel("tab", "a.txt"))
	app.Click(win, app.WaitNode(win, "b.txt row", RoleLabel("treeItem", "b.txt")), 1)
	app.WaitNode(win, "b.txt tab", RoleLabel("tab", "b.txt"))
	if got := countRole(app.Tree(win), "tab"); got != 2 {
		t.Fatalf("tabs = %d, want 2", got)
	}
}

func TestEditAndSaveWritesFile(t *testing.T) {
	app, win, root := openProject(t, map[string]string{"note.txt": "hello\n"})
	app.Click(win, app.WaitNode(win, "note.txt row", RoleLabel("treeItem", "note.txt")), 2)
	field := app.WaitNode(win, "editor", RoleLabel("textField", "Code editor"))
	app.Act(win, field.ID, "focus", "")
	app.Type(win, "edited ")
	app.Menu(win, "File", "Save")
	path := filepath.Join(root, "note.txt")
	app.Eventually("note.txt saved", func() bool {
		data, err := os.ReadFile(path)
		return err == nil && strings.Contains(string(data), "edited ")
	})
}

func TestSplitRightShowsTwoEditors(t *testing.T) {
	app, win, _ := openProject(t, map[string]string{"a.txt": "alpha\n"})
	app.Click(win, app.WaitNode(win, "a.txt row", RoleLabel("treeItem", "a.txt")), 2)
	app.WaitNode(win, "editor", RoleLabel("textField", "Code editor"))
	app.Act(win, app.WaitNode(win, "split button", RoleLabel("button", "Split Right")).ID, "press", "")
	app.Eventually("two editors", func() bool { return countRole(app.Tree(win), "textField") == 2 })
}

func TestCloseTabShowsPlaceholder(t *testing.T) {
	app, win, _ := openProject(t, map[string]string{"a.txt": "alpha\n"})
	app.Click(win, app.WaitNode(win, "a.txt row", RoleLabel("treeItem", "a.txt")), 2)
	app.WaitNode(win, "a.txt tab", RoleLabel("tab", "a.txt"))
	app.Menu(win, "File", "Close Editor")
	app.WaitNode(win, "placeholder", Label("No open files"))
}
