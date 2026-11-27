//go:build e2e

package e2e

import (
	"strings"
	"testing"
)

const (
	peekPreviewLabel = "Peek preview"
	defsFileName     = "defs.go"
	paletteLabel     = "Command Palette"
)

func TestPeekAndGoToDefinitionInTheApp(t *testing.T) {
	t.Parallel()
	app, win, _ := openProject(t, map[string]string{"use.go": "foo()\n", defsFileName: "package main\n\nfunc foo() {}\n"})
	openInEditor(t, app, win, "use.go")
	app.Eventually("the peek of the definition", func() bool {
		app.Key(win, "Alt+F12")
		n, ok := app.Find(win, RoleLabel("textField", peekPreviewLabel))
		return ok && strings.Contains(n.Value, "func foo")
	})
	app.Key(win, "Escape")
	app.Eventually("the peek closed", func() bool {
		_, open := app.Find(win, RoleLabel("textField", peekPreviewLabel))
		return !open
	})
	app.Menu(win, "View", "Command Palette...")
	app.WaitNode(win, "palette", RoleLabel("dialog", paletteLabel))
	app.Type(win, "Go to Definition")
	app.WaitNode(win, "go to definition command", func(n Node) bool { return strings.HasPrefix(n.Label, "Go to Definition") })
	app.Key(win, "Enter")
	app.WaitNode(win, "the definition tab", RoleLabel("tab", defsFileName))
}
