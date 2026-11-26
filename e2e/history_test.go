//go:build e2e

package e2e

import (
	"strings"
	"testing"
)

func TestLocalHistoryShowsSavedVersions(t *testing.T) {
	app, win, _ := openProject(t, map[string]string{"a.txt": "alpha\n"})
	app.Click(win, app.WaitNode(win, "a.txt row", RoleLabel("treeItem", "a.txt")), 2)
	field := app.WaitNode(win, "editor", RoleLabel("textField", "Code editor"))
	app.Act(win, field.ID, "focus", "")
	app.Type(win, "one ")
	app.Menu(win, "File", "Save")
	app.Type(win, "two ")
	app.Menu(win, "File", "Save")
	app.Menu(win, "View", "Command Palette...")
	app.WaitNode(win, "palette", RoleLabel("dialog", "Command Palette"))
	app.Type(win, "show history")
	app.WaitNode(win, "history command", func(n Node) bool { return strings.HasSuffix(n.Label, "Show History") })
	app.Key(win, "Enter")
	app.WaitNode(win, "history tab", RoleLabel("tab", "History: a.txt"))
}
