//go:build e2e

package e2e

import (
	"testing"
)

func dumpProbe(t *testing.T, app *App, win int, when string) {
	for _, n := range app.Tree(win) {
		if n.Role == "dialog" || n.Role == "textField" || n.Role == "toolbar" {
			t.Logf("%s: %s %q %v", when, n.Role, n.Label, n.States)
		}
	}
}

func TestProbeCtrlG(t *testing.T) {
	app, win, _ := openProject(t, map[string]string{"a.txt": "alpha\nbeta\n"})
	app.Click(win, app.WaitNode(win, "a.txt row", RoleLabel("treeItem", "a.txt")), 2)
	field := app.WaitNode(win, "editor", RoleLabel("textField", "Code editor"))
	dumpProbe(t, app, win, "start")
	app.Key(win, "Ctrl+G")
	dumpProbe(t, app, win, "ctrl+g no explicit focus")
	app.Key(win, "Escape")
	dumpProbe(t, app, win, "after escape")
	app.Act(win, field.ID, "focus", "")
	app.Menu(win, "Go", "Go to Line/Column...")
	dumpProbe(t, app, win, "after menu")
	app.Key(win, "Escape")
	app.Click(win, app.WaitNode(win, "a.txt row", RoleLabel("treeItem", "a.txt")), 1)
	app.Key(win, "Ctrl+G")
	dumpProbe(t, app, win, "ctrl+g with tree focus")
	app.Key(win, "Cmd+Alt+F")
	dumpProbe(t, app, win, "cmd+alt+f")
}
