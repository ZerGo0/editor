//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZerGo0/editor/internal/testutil"
)

const (
	keysFileLines   = 20
	firstChange     = 1
	secondChange    = 14
	diffToolbar     = "Diff Toolbar"
	diffSideLabel   = "New text"
	stackListPrefix = "Diffs of "
)

func numberedLines(changed ...int) string {
	var b strings.Builder
	for i := range keysFileLines {
		word := "line"
		for _, c := range changed {
			if c == i {
				word = "changed"
			}
		}
		fmt.Fprintf(&b, "%s %d\n", word, i)
	}
	return b.String()
}

func focusPath(app *App, win int) []Node {
	app.t.Helper()
	var res struct {
		Focus string `json:"focus"`
		Nodes []Node `json:"nodes"`
	}
	if err := json.Unmarshal(app.must("tree", map[string]any{"window": win}), &res); err != nil {
		app.t.Fatal(err)
	}
	byID := map[string]Node{}
	for _, n := range res.Nodes {
		byID[n.ID] = n
	}
	var path []Node
	for n, ok := byID[res.Focus]; ok && len(path) < len(res.Nodes); n, ok = byID[n.Parent] {
		path = append(path, n)
	}
	return path
}

func waitFocusIn(app *App, win int, what string, match func(Node) bool) {
	app.t.Helper()
	var last []Node
	defer func() {
		if app.t.Failed() {
			for _, n := range last {
				app.t.Logf("focus path: %s %q", n.Role, n.Label)
			}
		}
	}()
	app.Eventually(what, func() bool {
		last = focusPath(app, win)
		for _, n := range last {
			if match(n) {
				return true
			}
		}
		return false
	})
}

func runPalette(app *App, win int, query, row string) {
	app.t.Helper()
	app.Menu(win, "View", "Command Palette...")
	waitQuickOpen(app, win, commandPaletteTitle)
	app.Type(win, query)
	app.WaitNode(win, row+" row", RoleLabel("listItem", row))
	app.Key(win, "Enter")
}

func waitText(app *App, win int, text string) {
	app.t.Helper()
	app.WaitNode(win, text, Label(text))
}

func TestSingleDiffKeysStepChangesAndJumpToSource(t *testing.T) {
	t.Parallel()
	root := gitProject(t, map[string]string{"a.txt": numberedLines()})
	testutil.WriteFile(t, filepath.Join(root, "a.txt"), numberedLines(firstChange, secondChange))
	app := Launch(t, root)
	win := app.WaitWindow(filepath.Base(root))
	app.Idle(win)
	openInEditor(t, app, win, "a.txt")
	runPalette(app, win, "show diff with head", "Git: Show Diff with HEAD")
	app.WaitNode(win, "diff toolbar", Label(diffToolbar))
	waitText(app, win, "2 changes")
	waitFocusIn(app, win, "focus in the diff", Label(diffSideLabel))
	app.Key(win, "F7")
	waitText(app, win, "1 of 2")
	app.Key(win, "F7")
	waitText(app, win, "2 of 2")
	app.Key(win, "Shift+Alt+F5")
	waitText(app, win, "1 of 2")
	app.Key(win, "Alt+F5")
	waitText(app, win, "2 of 2")
	app.Key(win, "F4")
	app.WaitNode(win, "caret on the second change", caretOnLine(secondChange+1))
	runPalette(app, win, "git show log", "Git: Show Log")
	waitFocusIn(app, win, "focus in the commit list", RoleLabel("list", "Commits"))
}

func caretOnLine(line int) func(Node) bool {
	return func(n Node) bool { return strings.HasPrefix(n.Label, fmt.Sprintf("Ln %d, Col 1", line)) }
}

func TestSourceControlRowKeysAndStackedDiffFocus(t *testing.T) {
	t.Parallel()
	root := gitProject(t, map[string]string{"a.txt": numberedLines(), "b.txt": numberedLines()})
	testutil.WriteFile(t, filepath.Join(root, "a.txt"), numberedLines(secondChange))
	testutil.WriteFile(t, filepath.Join(root, "b.txt"), numberedLines(firstChange))
	app := Launch(t, root)
	win := app.WaitWindow(filepath.Base(root))
	app.Idle(win)
	app.Menu(win, "View", "Source Control")
	row := app.WaitNode(win, "a.txt row", func(n Node) bool { return hasName(n.Label, "a.txt") && n.Role != "treeItem" })
	app.Click(win, row, 1)
	app.WaitNode(win, "stacked diff toolbar", Label(diffToolbar))
	waitFocusIn(app, win, "focus in the changes list", RoleLabel("list", "Changes"))
	app.Key(win, "Enter")
	waitFocusIn(app, win, "focus in the stacked diff", func(n Node) bool { return strings.HasPrefix(n.Label, stackListPrefix) })
	app.Key(win, "F7")
	app.Key(win, "F4")
	waitTab(app, win, "a.txt", true)
	app.WaitNode(win, "caret on the change", caretOnLine(secondChange+1))
	app.Menu(win, "View", "Source Control")
	b := app.WaitNode(win, "b.txt row", func(n Node) bool { return hasName(n.Label, "b.txt") && n.Role != "treeItem" })
	app.Click(win, b, 1)
	waitFocusIn(app, win, "focus in the changes list", RoleLabel("list", "Changes"))
	app.Key(win, "F4")
	waitTab(app, win, "b.txt", true)
}

func conflictProject(t *testing.T) string {
	t.Helper()
	root := gitProject(t, map[string]string{"m.txt": "a\nb\nc\n"})
	testutil.RunGit(t, root, "checkout", "-q", "-b", "other")
	testutil.WriteFile(t, filepath.Join(root, "m.txt"), "a\ntheirs\nc\n")
	testutil.RunGit(t, root, "commit", "-q", "-am", "theirs")
	testutil.RunGit(t, root, "checkout", "-q", "main")
	testutil.WriteFile(t, filepath.Join(root, "m.txt"), "a\nours\nc\n")
	testutil.RunGit(t, root, "commit", "-q", "-am", "ours")
	merge := exec.Command("git", "merge", "-q", "other")
	merge.Dir, merge.Env = root, os.Environ()
	if out, err := merge.CombinedOutput(); err == nil {
		t.Fatalf("merge produced no conflict:\n%s", out)
	}
	return root
}

func TestConflictsEnterOpensTheMergeEditorWithFocus(t *testing.T) {
	t.Parallel()
	root := conflictProject(t)
	app := Launch(t, root)
	win := app.WaitWindow(filepath.Base(root))
	app.Idle(win)
	runPalette(app, win, "resolve conflicts", "Git: Resolve Conflicts...")
	app.WaitNode(win, "conflicts dialog", RoleLabel("dialog", "Conflicts"))
	waitFocusIn(app, win, "focus in the conflict list", RoleLabel("list", "Conflicted Files"))
	app.Key(win, "Enter")
	waitText(app, win, "1 conflict left")
	waitFocusIn(app, win, "focus in the result", Label("Result"))
	app.Key(win, "F7")
	app.Key(win, "Ctrl+Shift+Right")
	waitText(app, win, "All conflicts are resolved")
}
