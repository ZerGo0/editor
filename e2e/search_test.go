//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var searchProject = map[string]string{
	"main.go":          "package main\n\nfunc main() {\n\tstartServer()\n}\n",
	"server/server.go": "package server\n\ntype Handler struct{}\n\nfunc startServer() {\n\t// needle here\n}\n",
	"docs/readme.md":   "# Title\n\nNothing to see.\n",
}

const (
	searchEverywhereTitle = "Search Everywhere"
	commandPaletteTitle   = "Command Palette"
	findInFilesTitle      = "Find in Files"
	replaceInFilesTitle   = "Replace in Files"
	commandCenterLabel    = "Search Everywhere"
	matchRowLabel         = "line 6: // needle here"
	replaceRowLabel       = "// needle here line 6"
)

func waitQuickOpen(app *App, win int, title string) {
	app.t.Helper()
	app.WaitNode(win, title+" quick open", RoleLabel("dialog", title))
}

func waitTab(app *App, win int, name string, selected bool) {
	app.t.Helper()
	app.WaitNode(win, name+" tab", func(n Node) bool { return n.Role == "tab" && n.Label == name && n.Has("checked") == selected })
}

func TestGoToFileOpensFile(t *testing.T) {
	t.Parallel()
	app, win, _ := openProject(t, searchProject)
	app.Menu(win, "Go", "Go to File...")
	waitQuickOpen(app, win, "Go to File")
	waitTab(app, win, "Files", true)
	app.Type(win, "srvgo")
	app.WaitNode(win, "server.go result", func(n Node) bool { return strings.HasPrefix(n.Label, "server.go") })
	app.Key(win, "Enter")
	app.WaitNode(win, "server.go tab", RoleLabel("tab", "server.go"))
}

func TestFindInFilesOpensMatch(t *testing.T) {
	t.Parallel()
	app, win, _ := openProject(t, searchProject)
	app.Menu(win, "Go", "Find in Files")
	waitQuickOpen(app, win, findInFilesTitle)
	waitTab(app, win, "Text", true)
	app.Type(win, "needle")
	row := app.WaitNode(win, "match row", Label(matchRowLabel))
	app.Click(win, row, 1)
	app.WaitNode(win, "server.go tab", RoleLabel("tab", "server.go"))
}

func TestCmdShiftFOpensTextTabWithMatch(t *testing.T) {
	t.Parallel()
	app, win, _ := openProject(t, searchProject)
	app.Key(win, "Cmd+Shift+F")
	waitQuickOpen(app, win, findInFilesTitle)
	waitTab(app, win, "Text", true)
	app.Type(win, "needle")
	app.WaitNode(win, "match row", Label(matchRowLabel))
}

func TestCmdShiftPOpensActionsTab(t *testing.T) {
	t.Parallel()
	app, win, _ := openProject(t, searchProject)
	app.Key(win, "Cmd+Shift+P")
	waitQuickOpen(app, win, commandPaletteTitle)
	waitTab(app, win, "Actions", true)
}

func TestTitleBarClickOpensSearchEverywhere(t *testing.T) {
	t.Parallel()
	app, win, _ := openProject(t, searchProject)
	app.Click(win, app.WaitNode(win, "command center", RoleLabel("button", commandCenterLabel)), 1)
	waitQuickOpen(app, win, searchEverywhereTitle)
	waitTab(app, win, "All", true)
}

func TestSearchEverywhereMenuOpensAllTab(t *testing.T) {
	t.Parallel()
	app, win, _ := openProject(t, searchProject)
	app.Menu(win, "Go", "Search Everywhere")
	waitQuickOpen(app, win, searchEverywhereTitle)
	waitTab(app, win, "All", true)
}

func TestTabKeySwitchesTabsAndKeepsQuery(t *testing.T) {
	t.Parallel()
	app, win, _ := openProject(t, searchProject)
	app.Menu(win, "Go", "Search Everywhere")
	waitTab(app, win, "All", true)
	app.Type(win, "needle")
	app.Key(win, "Tab")
	waitTab(app, win, "Actions", true)
	app.Key(win, "Shift+Tab")
	waitTab(app, win, "All", true)
	app.WaitNode(win, "kept query", func(n Node) bool { return n.Role == "textField" && n.Value == "needle" })
}

func TestSymbolSearchFindsType(t *testing.T) {
	t.Parallel()
	app, win, _ := openProject(t, searchProject)
	app.Menu(win, "Go", "Go to Symbol in Workspace...")
	waitQuickOpen(app, win, "Go to Symbol in Workspace")
	app.Type(win, "Handler")
	app.WaitNode(win, "symbol row", func(n Node) bool { return n.Role != "textField" && strings.HasPrefix(n.Label, "Handler") })
	app.Key(win, "Enter")
	app.WaitNode(win, "server.go tab", RoleLabel("tab", "server.go"))
}

func TestReplaceAllEditsMatchesAfterConfirmation(t *testing.T) {
	t.Parallel()
	app, win, root := openProject(t, searchProject)
	app.Menu(win, "Go", "Replace in Files")
	waitQuickOpen(app, win, replaceInFilesTitle)
	app.Type(win, "needle")
	app.WaitNode(win, "match row", Label(replaceRowLabel))
	replace := app.WaitNode(win, "replace field", RoleLabel("textField", "Replace"))
	app.Act(win, replace.ID, "focus", "")
	app.Type(win, "pin")
	buttons := func() []Node {
		var out []Node
		for _, n := range app.Tree(win) {
			if n.Role == "button" && n.Label == "Replace All" {
				out = append(out, n)
			}
		}
		return out
	}
	app.Click(win, buttons()[0], 1)
	app.WaitNode(win, "confirmation", func(n Node) bool { return n.Role == "dialog" && n.Label == "Replace All" })
	all := buttons()
	app.Click(win, all[len(all)-1], 1)
	app.Eventually("needle replaced on disk", func() bool {
		data, _ := os.ReadFile(filepath.Join(root, "server", "server.go"))
		return strings.Contains(string(data), "// pin here")
	})
}

func TestSearchViewIsGone(t *testing.T) {
	t.Parallel()
	app, win, _ := openProject(t, searchProject)
	if _, ok := app.Find(win, func(n Node) bool { return n.Label == "Search" && (n.Role == "tab" || n.Role == "button") }); ok {
		t.Fatal("the activity bar still shows a Search view")
	}
	app.Menu(win, "View", "Command Palette...")
	waitQuickOpen(app, win, commandPaletteTitle)
	app.Type(win, "Show Search")
	app.WaitNode(win, "Show Search command", func(n Node) bool { return strings.Contains(n.Label, "Show Search") })
	app.Key(win, "Enter")
	waitQuickOpen(app, win, findInFilesTitle)
	waitTab(app, win, "Text", true)
}
