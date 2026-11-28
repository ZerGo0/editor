//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"testing"
)

func notModified(name string) func(Node) bool {
	return func(n Node) bool { return n.Role == "tab" && n.Label == name && !tabModified(name)(n) }
}

func TestDirtyIndicatorFollowsSavedState(t *testing.T) {
	t.Parallel()
	app, win, root := openProject(t, map[string]string{"a.txt": "alpha\n"})
	app.Click(win, app.WaitNode(win, "a.txt row", RoleLabel("treeItem", "a.txt")), 2)
	focusEditor(app, win, "a.txt")
	app.WaitNode(win, "clean a.txt tab", notModified("a.txt"))
	app.Type(win, "x")
	app.WaitNode(win, "dirty after typing", tabModified("a.txt"))
	app.Key(win, "Cmd+S")
	app.WaitNode(win, "clean after save", notModified("a.txt"))
	app.Type(win, "y")
	app.WaitNode(win, "dirty after save and typing", tabModified("a.txt"))
	app.Key(win, "Cmd+Z")
	app.WaitNode(win, "clean after undo to saved", notModified("a.txt"))
	app.Type(win, "z")
	app.WaitNode(win, "dirty again", tabModified("a.txt"))
	app.Key(win, "Cmd+S")
	app.WaitNode(win, "clean after second save", notModified("a.txt"))
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("external\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	app.WaitNode(win, "reloaded text", func(n Node) bool { return n.Role == "textField" && n.Value == "external\n" })
	app.WaitNode(win, "clean after reload", notModified("a.txt"))
	app.Type(win, "w")
	app.WaitNode(win, "dirty after reload and typing", tabModified("a.txt"))
}
