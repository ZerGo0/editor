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
	modifiedState    = "Unsaved changes"
	closeButtonInset = 14
)

func clickAt(app *App, win int, x, y float64) {
	app.t.Helper()
	app.must("input", map[string]any{"window": win, "events": []map[string]any{{"type": "click", "x": x, "y": y, "clicks": 1}}})
	app.Idle(win)
}

func tabModified(name string) func(Node) bool {
	return func(n Node) bool {
		return n.Role == "tab" && n.Label == name && strings.Contains(n.Description, modifiedState)
	}
}

func hasTab(app *App, win int, name string) bool {
	_, ok := app.Find(win, RoleLabel("tab", name))
	return ok
}

func focusEditor(app *App, win int) {
	field := app.WaitNode(win, "editor", RoleLabel("textField", "Code editor"))
	app.Act(win, field.ID, "focus", "")
}

func TestNewUntitledFile(t *testing.T) {
	t.Parallel()
	app, win, _ := openProject(t, map[string]string{"a.txt": "alpha\n"})
	app.Menu(win, "File", "New Text File")
	app.WaitNode(win, "Untitled-1 tab", RoleLabel("tab", "Untitled-1"))
	focusEditor(app, win)
	app.Type(win, "draft")
	app.WaitNode(win, "modified Untitled-1 tab", tabModified("Untitled-1"))
	app.Menu(win, "File", "New Text File")
	app.WaitNode(win, "Untitled-2 tab", RoleLabel("tab", "Untitled-2"))
	if _, ok := app.Find(win, RoleLabel("tab", "Untitled-1")); !ok {
		t.Fatal("Untitled-1 tab closed")
	}
}

func TestHotExitRestoresUnsavedText(t *testing.T) {
	t.Parallel()
	userData := t.TempDir()
	root := Project(t, map[string]string{"a.txt": "alpha\n"})
	app := LaunchWith(t, userData, root)
	win := app.WaitWindow(filepath.Base(root))
	app.Idle(win)
	app.Click(win, app.WaitNode(win, "a.txt row", RoleLabel("treeItem", "a.txt")), 2)
	focusEditor(app, win)
	app.Type(win, "unsaved ")
	app.WaitNode(win, "modified a.txt tab", tabModified("a.txt"))
	app.Menu(win, "File", "Close Window")
	app.Eventually("project window closed", func() bool {
		return !slices.ContainsFunc(app.Windows(), func(w Window) bool { return strings.HasSuffix(w.Title, filepath.Base(root)) && w.Visible })
	})
	app.Quit()
	if data, err := os.ReadFile(filepath.Join(root, "a.txt")); err != nil || string(data) != "alpha\n" {
		t.Fatalf("a.txt on disk = %q, %v", data, err)
	}

	again := LaunchWith(t, userData, root)
	win = again.WaitWindow("a.txt - " + filepath.Base(root))
	again.Idle(win)
	again.WaitNode(win, "restored a.txt tab", tabModified("a.txt"))
	again.WaitNode(win, "restored text", func(n Node) bool {
		return n.Role == "textField" && strings.Contains(n.Value, "unsaved alpha")
	})
}

func TestFolderSettingsOverrideUserSetting(t *testing.T) {
	t.Parallel()
	userData := t.TempDir()
	if err := os.WriteFile(filepath.Join(userData, "settings.json"), []byte(`{"files.exclude": ["*.tmp"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	root := Project(t, map[string]string{"keep.tmp": "", "skip.log": ""})
	app := LaunchWith(t, userData, root)
	win := app.WaitWindow(filepath.Base(root))
	app.Idle(win)
	app.WaitNode(win, "skip.log row", RoleLabel("treeItem", "skip.log"))
	if _, ok := app.Find(win, RoleLabel("treeItem", "keep.tmp")); ok {
		t.Fatal("the user setting does not hide keep.tmp")
	}
	if err := os.MkdirAll(filepath.Join(root, ".editor"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".editor", "settings.json"), []byte(`{"files.exclude": ["*.log"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	app.WaitNode(win, "keep.tmp row", RoleLabel("treeItem", "keep.tmp"))
	app.Eventually("skip.log hidden", func() bool {
		_, ok := app.Find(win, RoleLabel("treeItem", "skip.log"))
		return !ok
	})
}

func TestClosingFirstTabKeepsOthers(t *testing.T) {
	t.Parallel()
	names := []string{"a.txt", "b.txt", "c.txt"}
	app, win, _ := openProject(t, map[string]string{"a.txt": "a\n", "b.txt": "b\n", "c.txt": "c\n"})
	for _, name := range names {
		app.Click(win, app.WaitNode(win, name+" row", RoleLabel("treeItem", name)), 2)
		app.WaitNode(win, name+" tab", RoleLabel("tab", name))
	}
	tab := app.WaitNode(win, "a.txt tab", RoleLabel("tab", "a.txt"))
	app.Click(win, tab, 1)
	clickAt(app, win, tab.Bounds.X+tab.Bounds.Width-closeButtonInset, tab.Bounds.Y+tab.Bounds.Height/2)
	app.Eventually("a.txt tab closed", func() bool { return !hasTab(app, win, "a.txt") })
	for _, name := range names[1:] {
		if !hasTab(app, win, name) {
			t.Fatalf("%s tab closed too", name)
		}
	}
	app.Menu(win, "File", "New Text File")
	app.WaitNode(win, "Untitled-1 tab", RoleLabel("tab", "Untitled-1"))
}

func TestOpenRecentFocusesAFolderOpenInAnotherWindow(t *testing.T) {
	t.Parallel()
	for name, choose := range map[string]func(app *App, win int, label string){
		"list": func(app *App, win int, label string) {
			app.Key(win, "Ctrl+R")
			app.WaitNode(win, "Open Recent entry", RoleLabel("listItem", label))
			app.Type(win, label)
			app.Key(win, "Enter")
		},
		"menu": func(app *App, win int, label string) { app.Menu(win, "File", "Open Recent", label) },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			first := Project(t, map[string]string{"a.txt": "a\n"})
			second := Project(t, map[string]string{"b.txt": "b\n"})
			app := LaunchWith(t, t.TempDir(), first, second)
			win := app.WaitWindow(filepath.Base(first))
			other := app.WaitWindow(filepath.Base(second))
			app.Idle(win)
			choose(app, win, filepath.Base(second))
			app.Eventually("the other window comes to the front", func() bool {
				return slices.ContainsFunc(app.Windows(), func(w Window) bool { return w.ID == other && w.Visible })
			})
			if !slices.ContainsFunc(app.Windows(), func(w Window) bool { return w.ID == win }) {
				t.Fatalf("the window that opened the folder closed; windows %v", app.Windows())
			}
		})
	}
}

func TestOpenRecentMenuReplacesTheCurrentWindow(t *testing.T) {
	t.Parallel()
	first := Project(t, map[string]string{"a.txt": "a\n"})
	second := Project(t, map[string]string{"b.txt": "b\n"})
	userData := t.TempDir()
	recent, err := json.Marshal(map[string][]string{"recent": {second}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userData, "session.json"), recent, 0o600); err != nil {
		t.Fatal(err)
	}
	app := LaunchWith(t, userData, first)
	win := app.WaitWindow(filepath.Base(first))
	app.Idle(win)
	app.must("menu", map[string]any{"window": win, "path": []string{"File", "Open Recent", filepath.Base(second)}})
	app.WaitWindow(filepath.Base(second))
	app.Eventually("the current window is replaced", func() bool {
		return !slices.ContainsFunc(app.Windows(), func(w Window) bool { return w.ID == win })
	})
}
