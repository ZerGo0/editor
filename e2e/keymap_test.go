//go:build e2e

package e2e

import (
	"path/filepath"
	"strings"
	"testing"
)

func openInEditor(t *testing.T, app *App, win int, name string) Node {
	t.Helper()
	app.Click(win, app.WaitNode(win, name+" row", RoleLabel("treeItem", name)), 2)
	field := app.WaitNode(win, "editor", RoleLabel("textField", "Code editor"))
	app.Act(win, field.ID, "focus", "")
	return field
}

func TestCtrlGOpensGoToLine(t *testing.T) {
	app, win, _ := openProject(t, map[string]string{"a.txt": "alpha\nbeta\n"})
	openInEditor(t, app, win, "a.txt")
	app.Key(win, "Ctrl+G")
	app.WaitNode(win, "go to line", RoleLabel("dialog", "Go to line"))
}

func TestCmdAltFShowsReplace(t *testing.T) {
	app, win, _ := openProject(t, map[string]string{"a.txt": "alpha\nbeta\n"})
	openInEditor(t, app, win, "a.txt")
	app.Key(win, "Cmd+Alt+F")
	app.WaitNode(win, "replace field", func(n Node) bool { return n.Role == "textField" && strings.HasPrefix(n.Label, "Replace") })
}

func TestCmdShiftHOpensSearchView(t *testing.T) {
	app, win, _ := openProject(t, map[string]string{"a.txt": "alpha\n"})
	openInEditor(t, app, win, "a.txt")
	app.Key(win, "Cmd+Shift+H")
	app.WaitNode(win, "search field", RoleLabel("textField", "Search"))
}

func TestCmdSlashCommentsLine(t *testing.T) {
	app, win, _ := openProject(t, map[string]string{"main.go": "package main\n\nfunc main() {}\n"})
	field := openInEditor(t, app, win, "main.go")
	app.Key(win, "Cmd+/")
	app.Eventually("line commented", func() bool {
		n, ok := app.Find(win, func(n Node) bool { return n.ID == field.ID })
		return ok && strings.HasPrefix(n.Value, "// package main")
	})
}

func launchWindowsKeymap(t *testing.T, files map[string]string) (*App, int) {
	t.Helper()
	userData := t.TempDir()
	writeFile(t, filepath.Join(userData, "settings.json"), "{\"keyboard.keymap\": \"vscode-windows\"}\n")
	root := Project(t, files)
	app := LaunchWith(t, userData, root)
	win := app.WaitWindow(filepath.Base(root))
	app.Idle(win)
	return app, win
}

func TestWindowsKeymapBindsCtrlPToGoToFile(t *testing.T) {
	app, win := launchWindowsKeymap(t, map[string]string{"a.txt": "alpha\n"})
	openInEditor(t, app, win, "a.txt")
	app.Menu(win, "View", "Command Palette...")
	app.WaitNode(win, "palette", RoleLabel("dialog", "Command Palette"))
	app.Type(win, "go to file")
	app.WaitNode(win, "Go to File with Ctrl+P", func(n Node) bool {
		return n.Label == "Go: Go to File..." && n.Description == "⌃P"
	})
	app.Key(win, "Enter")
	waitQuickOpen(app, win, "Go to File")
}

func TestWindowsKeymapCtrlHShowsReplace(t *testing.T) {
	app, win := launchWindowsKeymap(t, map[string]string{"a.txt": "alpha\n"})
	openInEditor(t, app, win, "a.txt")
	app.Key(win, "Ctrl+H")
	app.WaitNode(win, "replace field", func(n Node) bool { return n.Role == "textField" && strings.HasPrefix(n.Label, "Replace") })
}
