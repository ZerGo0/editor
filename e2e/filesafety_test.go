//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	readOnlyState = "Read-only"
	deletedState  = "Deleted from disk"
	readOnlyMode  = 0o444
)

func tabState(name, state string) func(Node) bool {
	return func(n Node) bool {
		return n.Role == "tab" && strings.HasPrefix(n.Label, name+", ") && strings.Contains(n.Label, state)
	}
}

func TestReadOnlyFileSaveFailsWithActions(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root can write read-only files")
	}
	app, win, root := openProject(t, map[string]string{"a.txt": "alpha\n"})
	p := filepath.Join(root, "a.txt")
	app.Click(win, app.WaitNode(win, "a.txt row", RoleLabel("treeItem", "a.txt")), 2)
	focusEditor(app, win, "a.txt")
	app.Type(win, "x")
	app.WaitNode(win, "dirty a.txt", tabModified("a.txt"))
	if err := os.Chmod(p, readOnlyMode); err != nil {
		t.Fatal(err)
	}
	app.Key(win, "Cmd+S")
	app.WaitNode(win, "save error", Label("Cannot save a.txt"))
	app.WaitNode(win, "Save As action", RoleLabel("button", "Save As..."))
	app.WaitNode(win, "Reveal action", RoleLabel("button", "Reveal in Finder"))
	app.WaitNode(win, "locked tab", tabState("a.txt", readOnlyState))
	if got, err := os.ReadFile(p); err != nil || string(got) != "alpha\n" {
		t.Fatalf("file = %q, %v", got, err)
	}
	app.Key(win, "Escape")
	app.Eventually("the toast hides", func() bool {
		_, ok := app.Find(win, RoleLabel("button", "Save As..."))
		return !ok
	})
}

func TestDeletedFileTabIsMarked(t *testing.T) {
	t.Parallel()
	app, win, root := openProject(t, map[string]string{"a.txt": "alpha\n"})
	app.Click(win, app.WaitNode(win, "a.txt row", RoleLabel("treeItem", "a.txt")), 2)
	app.WaitNode(win, "a.txt tab", RoleLabel("tab", "a.txt"))
	if err := os.Remove(filepath.Join(root, "a.txt")); err != nil {
		t.Fatal(err)
	}
	app.WaitNode(win, "deleted tab", tabState("a.txt", deletedState))
}
