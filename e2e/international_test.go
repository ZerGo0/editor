//go:build e2e

package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/text/unicode/norm"
)

const trojanMark = "The character U+202E is a control character that can change how the text around it shows"

func TestInternationalTextAndTheHelpMenu(t *testing.T) {
	t.Parallel()
	nfd := norm.NFD.String("café.txt")
	app, win, _ := openProject(t, map[string]string{
		"trojan.go": "package p\n\nvar s = \"abc\u202edef\"\n",
		nfd:         "x\n",
	})

	app.Key(win, "Cmd+P")
	waitQuickOpen(app, win, "Go to File")
	app.Type(win, "café")
	app.WaitNode(win, "the NFD file for an NFC query", func(n Node) bool { return strings.HasPrefix(norm.NFC.String(n.Label), "café.txt") })
	app.Key(win, "Escape")

	app.Key(win, "Cmd+P")
	waitQuickOpen(app, win, "Go to File")
	app.Type(win, "trojan")
	app.WaitNode(win, "trojan.go result", func(n Node) bool { return strings.HasPrefix(n.Label, "trojan.go") })
	app.Key(win, "Enter")
	app.WaitNode(win, "trojan.go tab", RoleLabel("tab", "trojan.go"))
	app.WaitNode(win, "the bidi control mark", Label(trojanMark))

	focusEditor(app, win, "trojan.go")
	app.must("input", map[string]any{"window": win, "events": []map[string]any{KeyEvent("Alt+Z"), TextEvent("ż")}})
	app.Idle(win)
	app.WaitNode(win, "ż typed with Option", func(n Node) bool { return n.Role == "textField" && strings.Contains(n.Value, "ż") })

	app.Menu(win, "Help", "Keyboard Shortcuts")
	app.WaitNode(win, "keyboard shortcuts tab", RoleLabel("tab", "Keyboard Shortcuts"))
}

func TestWelcomeOffersToForgetAMissingProject(t *testing.T) {
	t.Parallel()
	gone := filepath.Join(t.TempDir(), "gone-project")
	name := filepath.Base(gone)
	userData := t.TempDir()
	recent, err := json.Marshal(map[string][]string{recentSessionKey: {gone}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userData, "session.json"), recent, sessionFilePerm); err != nil {
		t.Fatal(err)
	}
	app := LaunchWith(t, userData)
	welcome := app.WaitWindow("Welcome")
	app.Idle(welcome)

	row := app.WaitNode(welcome, "the missing project row", func(n Node) bool { return strings.HasPrefix(n.Label, name) })
	app.Click(welcome, row, 1)
	app.WaitNode(welcome, "the missing folder notice", func(n Node) bool { return n.Label == "The folder "+name+" no longer exists" })
	remove := app.WaitNode(welcome, "the remove button", RoleLabel("button", "Remove from Recent Projects"))
	app.Click(welcome, remove, 1)
	app.Eventually("the project leaves Recent Projects", func() bool {
		_, found := app.Find(welcome, func(n Node) bool { return strings.HasPrefix(n.Label, name) })
		return !found
	})

	missing := filepath.Join(t.TempDir(), "nothing-here")
	app.SecondInstance(filepath.Dir(missing), missing)
	app.WaitNode(welcome, "the notice for a missing argument", Label("nothing-here does not exist"))
}
