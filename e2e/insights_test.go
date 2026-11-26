//go:build e2e

package e2e

import "testing"

func TestTODOPanelListsComments(t *testing.T) {
	t.Parallel()
	app, win, _ := openProject(t, map[string]string{"main.go": "package main\n\n// TODO: wire the server\nfunc main() {}\n"})
	app.Menu(win, "View", "TODO")
	app.WaitNode(win, "todo row", Label("TODO"))
	row := app.WaitNode(win, "main.go group", Label("main.go"))
	app.Click(win, row, 1)
	app.WaitNode(win, "main.go tab", RoleLabel("tab", "main.go"))
}

func TestProblemsPanelShowsBracketError(t *testing.T) {
	t.Parallel()
	app, win, _ := openProject(t, map[string]string{"bad.go": "package main\n\nfunc main() {\n"})
	app.Click(win, app.WaitNode(win, "bad.go row", RoleLabel("treeItem", "bad.go")), 2)
	app.WaitNode(win, "editor", RoleLabel("textField", "Code editor"))
	app.Menu(win, "View", "Problems")
	app.WaitNode(win, "problem row", Label("'{' is not closed"))
}
