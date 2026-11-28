//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const explorerFilterLabel = "Filter Files"

func selectedRow(label string) func(Node) bool {
	return func(n Node) bool { return n.Role == "treeItem" && n.Label == label && n.Has("checked") }
}

func onDisk(path string) func() bool {
	return func() bool {
		_, err := os.Stat(path)
		return err == nil
	}
}

func TestExplorerMultiSelectUndoAndFilter(t *testing.T) {
	t.Parallel()
	app, win, root := openProject(t, map[string]string{"a.txt": "a\n", "b.txt": "b\n"})
	app.Menu(win, "View", "Explorer")
	app.Click(win, app.WaitNode(win, "a.txt row", RoleLabel("treeItem", "a.txt")), 1)
	app.Key(win, "Shift+Down")
	app.WaitNode(win, "b.txt selected", selectedRow("b.txt"))
	app.WaitNode(win, "a.txt still selected", selectedRow("a.txt"))

	app.Key(win, "F2")
	app.WaitNode(win, "rename field", RoleLabel("textField", "Rename"))
	app.Type(win, "c")
	app.Key(win, "Enter")
	app.Eventually("c.txt on disk", onDisk(filepath.Join(root, "c.txt")))
	app.WaitNode(win, "c.txt row", RoleLabel("treeItem", "c.txt"))
	app.Key(win, "Cmd+Z")
	app.Eventually("b.txt back on disk", onDisk(filepath.Join(root, "b.txt")))
	app.Key(win, "Cmd+Shift+Z")
	app.Eventually("c.txt on disk again", onDisk(filepath.Join(root, "c.txt")))

	app.WaitNode(win, "c.txt row", RoleLabel("treeItem", "c.txt"))
	app.Key(win, "Cmd+F")
	app.WaitNode(win, "filter field", RoleLabel("textField", explorerFilterLabel))
	app.Type(win, "a.t")
	app.Eventually("c.txt filtered out", func() bool {
		_, shown := app.Find(win, RoleLabel("treeItem", "c.txt"))
		return !shown
	})
	app.WaitNode(win, "a.txt row", RoleLabel("treeItem", "a.txt"))
}

func focusExplorerRow(app *App, win int, name string) {
	app.t.Helper()
	app.Click(win, app.WaitNode(win, name+" row", RoleLabel("treeItem", name)), 1)
	app.WaitNode(win, name+" selected", selectedRow(name))
}

func runPaletteCommand(app *App, win int, query, label string) {
	app.t.Helper()
	app.Key(win, "Cmd+Shift+P")
	waitQuickOpen(app, win, commandPaletteTitle)
	app.Type(win, query)
	app.WaitNode(win, label+" command", func(n Node) bool { return strings.HasPrefix(n.Label, label) })
	app.Key(win, "Enter")
	waitQuickInputClosed(app, win, commandPaletteTitle)
}

func TestExplorerOpenToSideKey(t *testing.T) {
	t.Parallel()
	app, win, _ := openProject(t, map[string]string{"a.txt": "a\n"})
	app.Menu(win, "View", "Explorer")
	focusExplorerRow(app, win, "a.txt")
	app.Key(win, "Cmd+Enter")
	app.Eventually("a.txt open in two groups", func() bool {
		n := 0
		for _, node := range app.Tree(win) {
			if node.Role == "tab" && node.Label == "a.txt" {
				n++
			}
		}
		return n == 2
	})
}

func TestExplorerFindInFolderKey(t *testing.T) {
	t.Parallel()
	app, win, _ := openProject(t, map[string]string{"src/b.go": "package b\n", "a.txt": "a\n"})
	app.Menu(win, "View", "Explorer")
	focusExplorerRow(app, win, "src")
	app.Key(win, "Shift+Alt+F")
	app.WaitNode(win, "directory field", func(n Node) bool { return n.Role == "textField" && n.Label == "Directory" && n.Value == "src" })
}

func TestExplorerFindFilterCommand(t *testing.T) {
	t.Parallel()
	app, win, _ := openProject(t, map[string]string{"a.txt": "a\n", "b.txt": "b\n"})
	app.Menu(win, "View", "Explorer")
	focusExplorerRow(app, win, "a.txt")
	app.Key(win, "Cmd+Alt+F")
	app.WaitNode(win, "filter field", RoleLabel("textField", explorerFilterLabel))
}

func TestExplorerCompareWithSelected(t *testing.T) {
	t.Parallel()
	app, win, _ := openProject(t, map[string]string{"a.txt": "alpha\n", "b.txt": "beta\n"})
	app.Menu(win, "View", "Explorer")
	focusExplorerRow(app, win, "a.txt")
	runPaletteCommand(app, win, "select for compare", "Explorer: Select for Compare")
	focusExplorerRow(app, win, "b.txt")
	runPaletteCommand(app, win, "compare with selected", "Explorer: Compare with Selected")
	app.WaitNode(win, "compare tab", RoleLabel("tab", "a.txt \u2194 b.txt"))
	app.WaitNode(win, "one change", func(n Node) bool { return n.Label == "1 change" || n.Label == "1 of 1" })
}
