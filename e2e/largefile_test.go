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
	editLimitEnv = "EDITOR_E2E_EDIT_LIMIT"
	largeLimit   = 1 << 20
	largeLines   = 60_000
)

func TestLargeFileOpensEditsAndSaves(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	for i := range largeLines {
		b.WriteString("line " + strconv.Itoa(i) + " of the large file\n")
	}
	app, win, root := openProjectEnv(t, map[string]string{"big.log": b.String()}, []string{editLimitEnv + "=" + strconv.Itoa(largeLimit)})
	app.Click(win, app.WaitNode(win, "big.log row", RoleLabel("treeItem", "big.log")), 2)
	app.WaitNode(win, "large file mode", Label("Large file mode"))
	app.Type(win, "HEAD ")
	app.Menu(win, "File", "Save")
	app.Eventually("big.log saved", func() bool {
		data, err := os.ReadFile(filepath.Join(root, "big.log"))
		return err == nil && strings.HasPrefix(string(data), "HEAD line 0")
	})
}
