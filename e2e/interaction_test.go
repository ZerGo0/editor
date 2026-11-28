//go:build e2e

package e2e

import (
	"slices"
	"strings"
	"testing"
)

const (
	keysSearchField     = "Search keyboard shortcuts"
	settingsSearchField = "Search settings"
	notificationsDialog = "Notifications"
)

func waitFieldValue(app *App, win int, label, value string) {
	app.t.Helper()
	app.WaitNode(win, label+" = "+value, FieldValue(label, value))
}

func waitRowKeys(app *App, win int, row, keys string) {
	app.t.Helper()
	app.Eventually(row+" shows "+keys, func() bool {
		tree := app.Tree(win)
		parents := map[string]string{}
		rows := map[string]bool{}
		for _, n := range tree {
			parents[n.ID] = n.Parent
			if n.Role == "listItem" && n.Label == row {
				rows[n.ID] = true
			}
		}
		return slices.ContainsFunc(tree, func(n Node) bool {
			for id := n.Parent; n.Label == keys && id != ""; id = parents[id] {
				if rows[id] {
					return true
				}
			}
			return false
		})
	})
}

func TestKeyboardShortcutsAndSettingsFocusTheirSearch(t *testing.T) {
	t.Parallel()
	app, win, _ := openProject(t, map[string]string{"a.txt": "alpha\n"})
	openInEditor(t, app, win, "a.txt")
	app.Key(win, "Cmd+K")
	app.Key(win, "Cmd+S")
	app.WaitNode(win, "keyboard shortcuts tab", RoleLabel("tab", "Keyboard Shortcuts"))
	app.Type(win, "split")
	waitFieldValue(app, win, keysSearchField, "split")
	app.Click(win, app.WaitNode(win, "a shortcut row", func(n Node) bool { return strings.HasPrefix(n.Label, "View: Split Editor Right") }), 1)
	app.Key(win, "Cmd+F")
	app.Key(win, "Cmd+A")
	app.Type(win, "zoom")
	waitFieldValue(app, win, keysSearchField, "zoom")
	app.Key(win, "Cmd+Alt+K")
	app.WaitNode(win, "record keys on", func(n Node) bool { return n.Label == "Record Keys" && n.Has("checked") })
	app.Key(win, "F8")
	waitFieldValue(app, win, keysSearchField, "f8")
	app.WaitNode(win, "problem command found", func(n Node) bool { return strings.HasPrefix(n.Label, "Go to Next Problem in Files") })
	app.Key(win, "Escape")
	app.WaitNode(win, "record keys off", func(n Node) bool { return n.Label == "Record Keys" && !n.Has("checked") })
	app.Key(win, "Cmd+,")
	app.WaitNode(win, "settings tab", RoleLabel("tab", "Settings"))
	app.Type(win, "folding")
	waitFieldValue(app, win, settingsSearchField, "folding")
}

func TestPaletteFindsVSCodeTitlesWithContextKeys(t *testing.T) {
	t.Parallel()
	app, win, _ := openProject(t, map[string]string{"main.go": "package main\n\nfunc main() {}\n"})
	openInEditor(t, app, win, "main.go")
	app.Key(win, "Cmd+Shift+P")
	waitQuickOpen(app, win, commandPaletteTitle)
	for query, row := range map[string]string{
		"expand selection":                 "Editor: Expand Selection",
		"copy line down":                   "Editor: Copy Line Down",
		"add selection to next find match": "Editor: Add Selection to Next Find Match",
		"next problem in files":            "Go to Next Problem in Files (Error, Warning, Info)",
	} {
		app.Key(win, "Cmd+A")
		app.Type(win, query)
		app.WaitNode(win, row, func(n Node) bool { return strings.HasPrefix(n.Label, row) })
	}
	app.Key(win, "Cmd+A")
	app.Type(win, "go to definition")
	waitRowKeys(app, win, "Go to Definition", "F12")
	app.Key(win, "Cmd+A")
	app.Type(win, "search everywhere")
	waitRowKeys(app, win, "Go: Search Everywhere", "\u21e7\u21e7")
	app.Key(win, "Escape")
	app.Click(win, app.WaitNode(win, "notifications button", func(n Node) bool { return n.Role == "button" && strings.HasPrefix(n.Label, notificationsDialog) }), 1)
	app.WaitNode(win, "notification center", RoleLabel("dialog", notificationsDialog))
	app.Key(win, "Escape")
	waitQuickInputClosed(app, win, notificationsDialog)
}

func TestEditorIndexMaximizeAndProblemKeys(t *testing.T) {
	t.Parallel()
	app, win, _ := openProject(t, map[string]string{"a.txt": "alpha\n", "b.json": "{\n  \"a\": 1\n  \"b\" 2\n}\n"})
	openInEditor(t, app, win, "a.txt")
	openInEditor(t, app, win, "b.json")
	app.Key(win, "Ctrl+1")
	app.WaitNode(win, "a.txt active", func(n Node) bool { return n.Role == "tab" && n.Label == "a.txt" && n.Has("checked") })
	app.Key(win, "Ctrl+0")
	app.WaitNode(win, "b.json active", func(n Node) bool { return n.Role == "tab" && n.Label == "b.json" && n.Has("checked") })
	field := app.WaitNode(win, "editor", codeEditor("b.json"))
	app.Act(win, field.ID, "focus", "")
	app.Key(win, "F8")
	app.WaitNode(win, "caret on the problem", func(n Node) bool { return strings.HasPrefix(n.Label, "Ln 3,") })
	app.Key(win, "Cmd+\\")
	app.Eventually("two editors", func() bool { return countRole(app.Tree(win), "textField") == 2 })
	app.Key(win, "Cmd+K")
	app.Key(win, "Cmd+M")
	app.Eventually("one group shown", func() bool { return countRole(app.Tree(win), "textField") == 1 })
	app.Key(win, "Cmd+K")
	app.Key(win, "Cmd+M")
	app.Eventually("both groups shown", func() bool { return countRole(app.Tree(win), "textField") == 2 })
}

func TestRevertAndCompareWithSaved(t *testing.T) {
	t.Parallel()
	app, win, _ := openProject(t, map[string]string{"a.txt": "alpha\n"})
	field := openInEditor(t, app, win, "a.txt")
	app.Type(win, "new ")
	app.Eventually("edited", func() bool {
		n, ok := app.Find(win, func(n Node) bool { return n.ID == field.ID })
		return ok && strings.HasPrefix(n.Value, "new alpha")
	})
	app.Key(win, "Cmd+K")
	app.Key(win, "D")
	app.WaitNode(win, "compare tab", RoleLabel("tab", "a.txt (in file) \u2194 a.txt"))
	app.WaitNode(win, "one change", func(n Node) bool { return n.Label == "1 change" || n.Label == "1 of 1" })
	app.Click(win, app.WaitNode(win, "a.txt tab", RoleNamed("tab", "a.txt")), 1)
	app.Key(win, "Cmd+Shift+P")
	waitQuickOpen(app, win, commandPaletteTitle)
	app.Type(win, "revert file")
	app.WaitNode(win, "revert command", func(n Node) bool { return strings.HasPrefix(n.Label, "File: Revert File") })
	app.Key(win, "Enter")
	app.Eventually("reverted", func() bool {
		n, ok := app.Find(win, codeEditor("a.txt"))
		return ok && n.Value == "alpha\n"
	})
}
