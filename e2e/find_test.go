//go:build e2e

package e2e

import "testing"

const (
	findFieldLabel    = "Find"
	replaceFieldLabel = "Replace"
	replaceAllLabel   = "Replace All"
	iconButtonWidth   = 22
)

func TestFindReplaceMultilineQuery(t *testing.T) {
	t.Parallel()
	app, win, _ := openProject(t, map[string]string{"a.txt": "one\ntwo\nthree\none\ntwo\n"})
	editor := openInEditor(t, app, win, "a.txt")
	app.Key(win, "Cmd+Alt+F")
	query := app.WaitNode(win, "find field", RoleLabel("textField", findFieldLabel))
	app.Type(win, "one")
	app.Key(win, "Ctrl+Enter")
	app.Type(win, "two")
	app.Eventually("two-line query", func() bool {
		n, ok := app.Find(win, func(n Node) bool { return n.ID == query.ID })
		return ok && n.Value == "one\ntwo"
	})
	app.WaitNode(win, "match count", Label("1 of 2"))
	all := app.WaitNode(win, "Replace All button", RoleLabel("button", replaceAllLabel))
	if all.Bounds.Width != iconButtonWidth {
		t.Fatalf("Replace All is %v wide, want an icon-only button %d wide", all.Bounds.Width, iconButtonWidth)
	}
	replace := app.WaitNode(win, "replace field", RoleLabel("textField", replaceFieldLabel))
	app.Act(win, replace.ID, "focus", "")
	app.Type(win, "joined")
	app.Key(win, "Cmd+Enter")
	app.Eventually("both matches replaced", func() bool {
		n, ok := app.Find(win, func(n Node) bool { return n.ID == editor.ID })
		return ok && n.Value == "joined\nthree\njoined\n"
	})
}
