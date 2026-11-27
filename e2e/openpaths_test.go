//go:build e2e

package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const (
	openFilePalette  = "File: Open File..."
	openFileQuery    = "File: Open File"
	openPaletteKey   = "Cmd+Shift+P"
	pickedPathsEnv   = "EDITOR_E2E_PICKED_PATHS"
	sessionFilePerm  = 0o600
	recentSessionKey = "recent"
)

func pickedEnv(paths ...string) []string {
	return []string{pickedPathsEnv + "=" + strings.Join(paths, string(os.PathListSeparator))}
}

func hasWindow(app *App, id int) bool {
	return slices.ContainsFunc(app.Windows(), func(w Window) bool { return w.ID == id })
}

func runOpenFile(app *App, win int) {
	app.Key(win, openPaletteKey)
	app.WaitNode(win, "palette", RoleLabel("dialog", "Command Palette"))
	app.Type(win, openFileQuery)
	app.Click(win, app.WaitNode(win, "open file command", func(n Node) bool {
		return n.Role == "listItem" && strings.HasPrefix(n.Label, openFilePalette)
	}), 1)
	app.Eventually("palette closed", func() bool {
		_, open := app.Find(win, RoleLabel("dialog", "Command Palette"))
		return !open
	})
}

func TestOpenFolderReplacingTheWindowKeepsUnsavedText(t *testing.T) {
	t.Parallel()
	first := Project(t, map[string]string{"a.txt": "alpha\n"})
	second := Project(t, map[string]string{"b.txt": "beta\n"})
	userData := t.TempDir()
	recent, err := json.Marshal(map[string][]string{recentSessionKey: {second}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userData, "session.json"), recent, sessionFilePerm); err != nil {
		t.Fatal(err)
	}
	writeUserSettings(t, userData, `{"files.hotExit": "onExitAndWindowClose"}`)
	app := LaunchWith(t, userData, first)
	win := app.WaitWindow(filepath.Base(first))
	app.Idle(win)
	app.Click(win, app.WaitNode(win, "a.txt row", RoleLabel("treeItem", "a.txt")), 2)
	focusEditor(app, win)
	app.Type(win, "unsaved ")
	app.WaitNode(win, "modified a.txt tab", tabModified("a.txt"))

	app.must("menu", map[string]any{"window": win, "path": []string{"File", "Open Recent", filepath.Base(second)}})
	other := app.WaitWindow(filepath.Base(second))
	app.Eventually("the first window is replaced", func() bool { return !hasWindow(app, win) })
	if data, err := os.ReadFile(filepath.Join(first, "a.txt")); err != nil || string(data) != "alpha\n" {
		t.Fatalf("a.txt on disk = %q, %v", data, err)
	}

	app.Idle(other)
	app.must("menu", map[string]any{"window": other, "path": []string{"File", "Open Recent", filepath.Base(first)}})
	win = app.WaitWindow("a.txt - " + filepath.Base(first))
	app.Idle(win)
	app.WaitNode(win, "restored a.txt tab", tabModified("a.txt"))
	app.WaitNode(win, "restored text", func(n Node) bool {
		return n.Role == "textField" && strings.Contains(n.Value, "unsaved alpha")
	})
}

func TestOpenFolderDialogReplacesTheCurrentWindow(t *testing.T) {
	t.Parallel()
	first := Project(t, map[string]string{"a.txt": "alpha\n"})
	second := Project(t, map[string]string{"b.txt": "beta\n"})
	app := LaunchEnv(t, t.TempDir(), pickedEnv(second), first)
	win := app.WaitWindow(filepath.Base(first))
	app.Idle(win)
	app.must("menu", map[string]any{"window": win, "path": []string{"File", "Open Folder..."}})
	app.WaitWindow(filepath.Base(second))
	app.Eventually("the first window is replaced", func() bool { return !hasWindow(app, win) })
}

func TestOpenFileOutsideEveryFolderOpensInTheCurrentWindow(t *testing.T) {
	t.Parallel()
	root := Project(t, map[string]string{"a.txt": "alpha\n"})
	outside := filepath.Join(Project(t, map[string]string{"loose.txt": "loose\n"}), "loose.txt")
	userData := t.TempDir()
	app := LaunchEnv(t, userData, pickedEnv(outside), root)
	win := app.WaitWindow(filepath.Base(root))
	app.Idle(win)
	windows := len(app.Windows())

	app.Menu(win, "File", "Open File...")
	app.WaitWindow("loose.txt - " + filepath.Base(root))
	if got := len(app.Windows()); got != windows {
		t.Fatalf("windows = %d, want %d", got, windows)
	}
	app.WaitNode(win, "loose text", func(n Node) bool {
		return n.Role == "textField" && strings.Contains(n.Value, "loose")
	})
	app.Idle(win)
	focusEditor(app, win)
	app.Type(win, "edited ")
	app.WaitNode(win, "modified loose.txt tab", tabModified("loose.txt"))
	app.Menu(win, "File", "Save")
	app.Eventually("loose.txt saved", func() bool {
		data, err := os.ReadFile(outside)
		return err == nil && string(data) == "edited loose\n"
	})
	app.Quit()

	again := LaunchWith(t, userData, root)
	win = again.WaitWindow("loose.txt - " + filepath.Base(root))
	again.Idle(win)
	again.WaitNode(win, "restored loose.txt tab", RoleLabel("tab", "loose.txt"))
}

func TestOpenFileOfAnotherOpenFolderOpensInTheCurrentWindow(t *testing.T) {
	t.Parallel()
	first := Project(t, map[string]string{"a.txt": "alpha\n"})
	second := Project(t, map[string]string{"b.txt": "beta\n"})
	app := LaunchEnv(t, t.TempDir(), pickedEnv(filepath.Join(second, "b.txt")), first, second)
	win := app.WaitWindow(filepath.Base(first))
	other := app.WaitWindow(filepath.Base(second))
	app.Idle(win)
	app.Idle(other)

	runOpenFile(app, win)
	app.WaitNode(win, "b.txt tab in the current window", RoleLabel("tab", "b.txt"))
	if hasTab(app, other, "b.txt") {
		t.Fatal("the window of the file's folder got the tab")
	}
}
