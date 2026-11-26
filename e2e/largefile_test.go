//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const (
	largeLimit = 1 << 20
	largeLines = 60_000
)

func TestLargeFileOpensEditsAndSaves(t *testing.T) {
	t.Setenv("EDITOR_E2E_EDIT_LIMIT", strconv.Itoa(largeLimit))
	var b strings.Builder
	for i := range largeLines {
		b.WriteString("line " + strconv.Itoa(i) + " of the large file\n")
	}
	app, win, root := openProject(t, map[string]string{"big.log": b.String()})
	app.Click(win, app.WaitNode(win, "big.log row", RoleLabel("treeItem", "big.log")), 2)
	app.WaitNode(win, "large file mode", Label("Large file mode"))
	app.Type(win, "HEAD ")
	app.Menu(win, "File", "Save")
	app.Eventually("big.log saved", func() bool {
		data, err := os.ReadFile(filepath.Join(root, "big.log"))
		return err == nil && strings.HasPrefix(string(data), "HEAD line 0")
	})
}
