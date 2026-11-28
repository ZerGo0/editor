//go:build e2e

package e2e

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZerGo0/editor/internal/testutil"
)

const goToLineTitle = "Go to Line/Column"

func openInEditor(t *testing.T, app *App, win int, name string) Node {
	t.Helper()
	app.Click(win, app.WaitNode(win, name+" row", RoleLabel("treeItem", name)), 2)
	field := app.WaitNode(win, "editor", codeEditor(name))
	app.Act(win, field.ID, "focus", "")
	return field
}

func isReplaceField(n Node) bool {
	return n.Role == "textField" && strings.HasPrefix(n.Label, "Replace")
}

func TestCtrlGOpensGoToLine(t *testing.T) {
	t.Parallel()
	app, win, _ := openProject(t, map[string]string{"a.txt": "alpha\nbeta\n"})
	openInEditor(t, app, win, "a.txt")
	app.Key(win, "Ctrl+G")
	app.WaitNode(win, "go to line", RoleLabel("dialog", goToLineTitle))
	app.Type(win, "2")
	app.Key(win, "Enter")
	app.WaitNode(win, "caret on line 2", func(n Node) bool { return strings.HasPrefix(n.Label, "Ln 2, Col 1") })
	waitQuickInputClosed(app, win, goToLineTitle)
}

func TestStatusBarPositionOpensGoToLineAndTypingStaysOutOfTheDocument(t *testing.T) {
	t.Parallel()
	app, win, _ := openProject(t, map[string]string{"a.txt": "alpha\nbeta\ngamma\n"})
	field := openInEditor(t, app, win, "a.txt")
	app.Click(win, app.WaitNode(win, "position item", func(n Node) bool { return n.Role == "button" && strings.HasPrefix(n.Label, "Ln 1, Col 1") }), 1)
	app.WaitNode(win, "go to line", RoleLabel("dialog", goToLineTitle))
	app.Type(win, "3")
	app.Key(win, "Escape")
	waitQuickInputClosed(app, win, goToLineTitle)
	app.Eventually("document untouched", func() bool {
		n, ok := app.Find(win, codeEditor("a.txt"))
		return ok && n.ID == field.ID && !strings.Contains(n.Value, "3")
	})
}

func TestCmdAltFShowsReplace(t *testing.T) {
	t.Parallel()
	app, win, _ := openProject(t, map[string]string{"a.txt": "alpha\nbeta\n"})
	openInEditor(t, app, win, "a.txt")
	app.Key(win, "Cmd+Alt+F")
	app.WaitNode(win, "replace field", isReplaceField)
}

func TestCmdShiftHAndCmdShiftROpenReplaceInFiles(t *testing.T) {
	t.Parallel()
	app, win, _ := openProject(t, map[string]string{"a.txt": "alpha\n"})
	openInEditor(t, app, win, "a.txt")
	for _, key := range []string{"Cmd+Shift+H", "Cmd+Shift+R"} {
		app.Key(win, key)
		app.WaitNode(win, key+" replace popup", RoleLabel("dialog", replaceInFilesTitle))
		app.Key(win, "Escape")
		app.Eventually("popup closed", func() bool {
			_, open := app.Find(win, RoleLabel("dialog", replaceInFilesTitle))
			return !open
		})
	}
}

func TestCmdPAndCmdShiftOOpenFilesAndCmdShiftAOpensActions(t *testing.T) {
	t.Parallel()
	app, win, _ := openProject(t, map[string]string{"a.txt": "alpha\n"})
	openInEditor(t, app, win, "a.txt")
	for _, key := range []string{"Cmd+P", "Cmd+Shift+O"} {
		app.Key(win, key)
		waitQuickOpen(app, win, "Go to File")
		waitTab(app, win, "Files", true)
		app.Key(win, "Escape")
	}
	app.Key(win, "Cmd+Shift+A")
	waitQuickOpen(app, win, commandPaletteTitle)
}

func TestCmdEOpensRecentFiles(t *testing.T) {
	t.Parallel()
	app, win, _ := openProject(t, map[string]string{"a.txt": "alpha\n"})
	openInEditor(t, app, win, "a.txt")
	app.Key(win, "Cmd+E")
	waitQuickOpen(app, win, "Recent Files")
	app.WaitNode(win, "tool windows", Label("Views"))
}

func TestCmdSlashCommentsLine(t *testing.T) {
	t.Parallel()
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
	testutil.WriteFile(t, filepath.Join(userData, "settings.json"), "{\"keyboard.keymap\": \"vscode-windows\"}\n")
	root := Project(t, files)
	app := LaunchWith(t, userData, root)
	win := app.WaitWindow(filepath.Base(root))
	app.Idle(win)
	return app, win
}

func TestWindowsKeymapBindsCtrlPToGoToFile(t *testing.T) {
	t.Parallel()
	app, win := launchWindowsKeymap(t, map[string]string{"a.txt": "alpha\n"})
	openInEditor(t, app, win, "a.txt")
	app.Menu(win, "View", "Command Palette...")
	app.WaitNode(win, "palette", RoleLabel("dialog", "Command Palette"))
	app.Type(win, "go to file")
	row := app.WaitNode(win, "Go to File row", RoleLabel("listItem", "Go: Go to File..."))
	app.WaitNode(win, "Go to File with Ctrl+P", func(n Node) bool {
		return n.Label == "⌃P" && n.Bounds.Y >= row.Bounds.Y && n.Bounds.Y < row.Bounds.Y+row.Bounds.Height
	})
	app.Key(win, "Enter")
	waitQuickOpen(app, win, "Go to File")
}

func TestWindowsKeymapCtrlHShowsReplace(t *testing.T) {
	t.Parallel()
	app, win := launchWindowsKeymap(t, map[string]string{"a.txt": "alpha\n"})
	openInEditor(t, app, win, "a.txt")
	app.Key(win, "Ctrl+H")
	app.WaitNode(win, "replace field", isReplaceField)
}

func TestBindingWithoutWhenRunsWithTheExplorerFocused(t *testing.T) {
	t.Parallel()
	userData := t.TempDir()
	testutil.WriteFile(t, filepath.Join(userData, "keybindings.json"), "[{\"key\": \"f9\", \"command\": \"workbench.action.findInFiles\"}]\n")
	root := Project(t, map[string]string{"a.txt": "alpha\n"})
	app := LaunchWith(t, userData, root)
	win := app.WaitWindow(filepath.Base(root))
	app.Idle(win)
	app.Menu(win, "View", "Explorer")
	app.WaitNode(win, "a.txt row", RoleLabel("treeItem", "a.txt"))
	app.Key(win, "F9")
	waitQuickOpen(app, win, findInFilesTitle)
	waitTab(app, win, "Text", true)
}
