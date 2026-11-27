//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCommandPaletteRunsCommand(t *testing.T) {
	t.Parallel()
	app, win, _ := openProject(t, map[string]string{"a.txt": "alpha\n"})
	app.Click(win, app.WaitNode(win, "a.txt row", RoleLabel("treeItem", "a.txt")), 2)
	app.WaitNode(win, "editor", RoleLabel("textField", "Code editor"))
	app.Menu(win, "View", "Command Palette...")
	app.WaitNode(win, "palette", RoleLabel("dialog", "Command Palette"))
	app.Type(win, "split right")
	app.WaitNode(win, "split command", func(n Node) bool { return strings.HasPrefix(n.Label, "View: Split Editor Right") })
	app.Key(win, "Enter")
	app.Eventually("two editors", func() bool { return countRole(app.Tree(win), "textField") == 2 })
}

func TestCommandPaletteListsRecentlyUsedFirst(t *testing.T) {
	t.Parallel()
	app, win, _ := openProject(t, map[string]string{"a.txt": "alpha\n"})
	app.Click(win, app.WaitNode(win, "a.txt row", RoleLabel("treeItem", "a.txt")), 2)
	app.WaitNode(win, "editor", RoleLabel("textField", "Code editor"))
	app.Menu(win, "View", "Command Palette...")
	app.WaitNode(win, "palette", RoleLabel("dialog", "Command Palette"))
	app.Type(win, "split right")
	app.WaitNode(win, "split command", func(n Node) bool { return strings.HasPrefix(n.Label, "View: Split Editor Right") })
	app.Key(win, "Enter")
	app.Eventually("two editors", func() bool { return countRole(app.Tree(win), "textField") == 2 })
	app.Menu(win, "View", "Command Palette...")
	app.WaitNode(win, "palette", RoleLabel("dialog", "Command Palette"))
	app.WaitNode(win, "kept query", func(n Node) bool {
		return n.Role == "textField" && n.Label == "Command Palette" && n.Value == "split right"
	})
	app.Key(win, "Cmd+A")
	app.Key(win, "Backspace")
	group := app.WaitNode(win, "recently used label", Label("Recently Used"))
	split := app.WaitNode(win, "split command", func(n Node) bool { return strings.HasPrefix(n.Label, "View: Split Editor Right") })
	if split.Bounds.Y != group.Bounds.Y+group.Bounds.Height {
		t.Fatalf("recently used label %+v does not head the first row %+v", group.Bounds, split.Bounds)
	}
	app.WaitNode(win, "other commands label", Label("Other Commands"))
}

func TestQuickOpenRecentlyOpenedOpensToSide(t *testing.T) {
	t.Parallel()
	app, win, _ := openProject(t, map[string]string{"a.txt": "alpha\n", "b.txt": "beta\n"})
	app.Click(win, app.WaitNode(win, "a.txt row", RoleLabel("treeItem", "a.txt")), 2)
	app.WaitNode(win, "a.txt tab", RoleLabel("tab", "a.txt"))
	app.Click(win, app.WaitNode(win, "b.txt row", RoleLabel("treeItem", "b.txt")), 2)
	app.WaitNode(win, "b.txt tab", RoleLabel("tab", "b.txt"))
	app.Menu(win, "Go", "Go to File...")
	dialog := app.WaitNode(win, "quick open", RoleLabel("dialog", "Go to File"))
	app.WaitNode(win, "recently opened label", Label("Recently Opened"))
	app.WaitNode(win, "remove button", RoleLabel("button", "Remove from Recently Opened"))
	for _, n := range app.Tree(win) {
		if n.Label == "b.txt" && n.Role != "tab" && n.Role != "treeItem" && n.Bounds.Y > dialog.Bounds.Y {
			t.Fatalf("the active file is listed in Quick Open: %+v", n)
		}
	}
	app.Key(win, "Cmd+Enter")
	app.Eventually("a.txt opened to the side", func() bool { return countRole(app.Tree(win), "textField") == 2 })
}

func TestChordOpensKeyboardShortcuts(t *testing.T) {
	t.Parallel()
	app, win, _ := openProject(t, map[string]string{"a.txt": "alpha\n"})
	app.Click(win, app.WaitNode(win, "a.txt row", RoleLabel("treeItem", "a.txt")), 2)
	field := app.WaitNode(win, "editor", RoleLabel("textField", "Code editor"))
	app.Act(win, field.ID, "focus", "")
	app.Key(win, "Cmd+K")
	app.Key(win, "Cmd+S")
	app.WaitNode(win, "keyboard shortcuts tab", RoleLabel("tab", "Keyboard Shortcuts"))
}

func TestUserKeybindingRunsCommand(t *testing.T) {
	t.Parallel()
	userData := t.TempDir()
	writeFile(t, filepath.Join(userData, "keybindings.json"), "// mine\n[{\"key\": \"cmd+e\", \"command\": \"workbench.action.closeAllEditors\"}]\n")
	root := Project(t, map[string]string{"a.txt": "alpha\n"})
	app := LaunchWith(t, userData, root)
	win := app.WaitWindow(filepath.Base(root))
	app.Click(win, app.WaitNode(win, "a.txt row", RoleLabel("treeItem", "a.txt")), 2)
	app.WaitNode(win, "a.txt tab", RoleLabel("tab", "a.txt"))
	app.Key(win, "Cmd+E")
	app.WaitNode(win, "placeholder", Label("No open files"))
}

func TestSettingsFileReloadsLive(t *testing.T) {
	t.Parallel()
	userData := t.TempDir()
	root := Project(t, map[string]string{"a.txt": "alpha\n", "skip.log": "x\n"})
	app := LaunchWith(t, userData, root)
	win := app.WaitWindow(filepath.Base(root))
	app.WaitNode(win, "skip.log row", RoleLabel("treeItem", "skip.log"))
	writeFile(t, filepath.Join(userData, "settings.json"), "{\n  // hide logs\n  \"files.exclude\": [\"*.log\"],\n}\n")
	app.Eventually("skip.log hidden", func() bool {
		_, ok := app.Find(win, RoleLabel("treeItem", "skip.log"))
		return !ok
	})
}

func TestSettingsSwitchWritesFile(t *testing.T) {
	t.Parallel()
	userData := t.TempDir()
	root := Project(t, map[string]string{"a.txt": "alpha\n"})
	app := LaunchWith(t, userData, root)
	win := app.WaitWindow(filepath.Base(root))
	app.Menu(win, "Editor", "Settings")
	search := app.WaitNode(win, "settings search", RoleLabel("textField", "Search settings"))
	app.Act(win, search.ID, "focus", "")
	app.Type(win, "folding")
	toggle := app.WaitNode(win, "folding switch", RoleLabel("switch", "Folding"))
	app.Act(win, toggle.ID, "press", "")
	app.Eventually("settings.json written", func() bool {
		data, err := os.ReadFile(filepath.Join(userData, "settings.json"))
		return err == nil && strings.Contains(string(data), "\"editor.folding\": false")
	})
	app.WaitNode(win, "reset button", RoleLabel("button", "Reset Folding"))
}

func TestNewFileFromMenu(t *testing.T) {
	t.Parallel()
	app, win, root := openProject(t, map[string]string{"a.txt": "alpha\n"})
	app.Menu(win, "File", "New File...")
	field := app.WaitNode(win, "name field", RoleLabel("textField", "New File"))
	app.Act(win, field.ID, "focus", "")
	app.Type(win, "notes/todo.md")
	app.Key(win, "Enter")
	app.WaitNode(win, "todo.md tab", RoleLabel("tab", "todo.md"))
	if _, err := os.Stat(filepath.Join(root, "notes", "todo.md")); err != nil {
		t.Fatal(err)
	}
}
