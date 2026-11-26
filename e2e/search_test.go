//go:build e2e

package e2e

import (
	"strings"
	"testing"
)

var searchProject = map[string]string{
	"main.go":          "package main\n\nfunc main() {\n\tstartServer()\n}\n",
	"server/server.go": "package server\n\ntype Handler struct{}\n\nfunc startServer() {\n\t// needle here\n}\n",
	"docs/readme.md":   "# Title\n\nNothing to see.\n",
}

func waitScope(app *App, win int, scope string) {
	app.t.Helper()
	app.WaitNode(win, scope+" scope", func(n Node) bool { return n.Role == "tab" && n.Label == scope && n.Has("checked") })
}

func waitQuickOpen(app *App, win int, title string) {
	app.t.Helper()
	app.WaitNode(win, title+" quick open", RoleLabel("dialog", title))
}

func TestGoToFileOpensFile(t *testing.T) {
	app, win, _ := openProject(t, searchProject)
	app.Menu(win, "Go", "Go to File...")
	waitQuickOpen(app, win, "Go to File")
	app.Type(win, "srvgo")
	app.WaitNode(win, "server.go result", func(n Node) bool { return strings.HasPrefix(n.Label, "server.go") })
	app.Key(win, "Enter")
	app.WaitNode(win, "server.go tab", RoleLabel("tab", "server.go"))
}

func TestFindInFilesOpensMatch(t *testing.T) {
	app, win, _ := openProject(t, searchProject)
	app.Menu(win, "Go", "Find in Files")
	waitScope(app, win, "Text")
	app.Type(win, "needle")
	app.Key(win, "Enter")
	row := app.WaitNode(win, "match row", Label("line 6: // needle here"))
	app.Click(win, row, 1)
	app.WaitNode(win, "server.go tab", RoleLabel("tab", "server.go"))
}

func TestSymbolSearchFindsType(t *testing.T) {
	app, win, _ := openProject(t, searchProject)
	app.Menu(win, "Go", "Go to Symbol in Workspace...")
	waitQuickOpen(app, win, "Go to Symbol in Workspace")
	app.Type(win, "Handler")
	app.WaitNode(win, "symbol row", func(n Node) bool { return n.Role != "textField" && strings.HasPrefix(n.Label, "Handler") })
	app.Key(win, "Enter")
	app.WaitNode(win, "server.go tab", RoleLabel("tab", "server.go"))
}

func TestTabSwitchesSearchScope(t *testing.T) {
	app, win, _ := openProject(t, searchProject)
	app.Menu(win, "Go", "Search Everywhere")
	waitScope(app, win, "All")
	app.Type(win, "split right")
	app.WaitNode(win, "action in all scope", func(n Node) bool { return strings.Contains(n.Label, "Split Editor Right") })
	app.Key(win, "Tab")
	waitScope(app, win, "Files")
	app.Eventually("files scope hides actions", func() bool {
		_, ok := app.Find(win, func(n Node) bool { return strings.Contains(n.Label, "Split Editor Right") })
		return !ok
	})
}
