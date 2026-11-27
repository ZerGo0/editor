//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZerGo0/editor/internal/design/token"
)

const (
	stackFiles     = 30
	stackLines     = 40
	stackTarget    = 5
	stackListLabel = "Diffs of Changes"
	headerSlack    = 2
)

func stackLinesOf(word string) string {
	var b strings.Builder
	for i := range stackLines {
		fmt.Fprintf(&b, "%s line %d\n", word, i)
	}
	return b.String()
}

func TestSourceControlFileOpensStackedDiffAtItsHeader(t *testing.T) {
	t.Parallel()
	files := map[string]string{}
	for i := range stackFiles {
		files[fmt.Sprintf("f%02d.txt", i)] = stackLinesOf("old")
	}
	root := gitProject(t, files)
	for name := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(stackLinesOf("new")), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	app := Launch(t, root)
	win := app.WaitWindow(filepath.Base(root))
	app.Idle(win)
	app.Menu(win, "View", "Source Control")
	target := fmt.Sprintf("f%02d.txt", stackTarget)
	row := app.WaitNode(win, "changed file", func(n Node) bool { return n.Label == target && n.Role != "treeItem" })
	app.Click(win, row, 1)
	app.WaitNode(win, "stacked diff toolbar", Label("Diff Toolbar"))
	app.Eventually("the file header at the top of the stack", func() bool {
		header, ok := app.Find(win, RoleLabel("group", target))
		stack, found := app.Find(win, RoleLabel("list", stackListLabel))
		if !ok || !found {
			return false
		}
		top := header.Bounds.Y - stack.Bounds.Y
		return top >= -headerSlack && top <= float64(token.HeaderHeight.Px())+headerSlack
	})
	app.WaitNode(win, "file tree", RoleLabel("list", "Changed Files"))
}
