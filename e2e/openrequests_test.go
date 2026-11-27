//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSecondInstanceOpensAFileInTheWindowOfItsFolder(t *testing.T) {
	t.Parallel()
	first := Project(t, map[string]string{"a.txt": "alpha\n"})
	second := Project(t, map[string]string{"b.txt": "beta\n"})
	app := LaunchWith(t, t.TempDir(), first, second)
	win := app.WaitWindow(filepath.Base(first))
	other := app.WaitWindow(filepath.Base(second))
	app.Idle(win)
	app.Idle(other)
	windows := len(app.Windows())

	app.SecondInstance(first, filepath.Join(first, "a.txt"))
	app.WaitNode(win, "a.txt tab in the window of its folder", RoleLabel("tab", "a.txt"))
	app.SecondInstance(second, "b.txt")
	app.WaitNode(other, "b.txt tab in the window of its folder", RoleLabel("tab", "b.txt"))
	if hasTab(app, other, "a.txt") || hasTab(app, win, "b.txt") {
		t.Fatal("a window got the file of another folder")
	}
	if got := len(app.Windows()); got != windows {
		t.Fatalf("windows = %d, want %d", got, windows)
	}
}

func TestSecondInstanceOpensAFileOutsideEveryFolderInTheLastActiveWindow(t *testing.T) {
	t.Parallel()
	first := Project(t, map[string]string{"a.txt": "alpha\n"})
	second := Project(t, map[string]string{"b.txt": "beta\n"})
	outside := filepath.Join(Project(t, map[string]string{"loose.txt": "loose\n"}), "loose.txt")
	app := LaunchWith(t, t.TempDir(), first, second)
	win := app.WaitWindow(filepath.Base(first))
	other := app.WaitWindow(filepath.Base(second))
	app.Idle(win)
	app.Idle(other)
	windows := len(app.Windows())

	app.SecondInstance(t.TempDir(), outside)
	app.WaitNode(other, "loose.txt tab in the last active window", RoleLabel("tab", "loose.txt"))
	if hasTab(app, win, "loose.txt") {
		t.Fatal("the first window got the file, not the last active one")
	}
	if got := len(app.Windows()); got != windows {
		t.Fatalf("windows = %d, want %d", got, windows)
	}
}

func TestSecondInstanceWithoutAnOpenFolderOpensTheFilesFolder(t *testing.T) {
	t.Parallel()
	dir := Project(t, map[string]string{"c.txt": "gamma\n"})
	app := Launch(t)
	app.WaitWindow("Welcome")

	app.SecondInstance(dir, "c.txt")
	win := app.WaitWindow("c.txt - " + filepath.Base(dir))
	app.WaitNode(win, "c.txt tab", RoleLabel("tab", "c.txt"))
}

func TestFileOutsideTheFolderReloadsWhenItChangesOnDisk(t *testing.T) {
	t.Parallel()
	root := Project(t, map[string]string{"a.txt": "alpha\n"})
	outside := filepath.Join(Project(t, map[string]string{"loose.txt": "before\n"}), "loose.txt")
	app := LaunchWith(t, t.TempDir(), root, outside)
	var win int
	app.Eventually("the folder window", func() bool {
		windows := app.Windows()
		if len(windows) == 1 {
			win = windows[0].ID
		}
		return win != 0
	})
	app.Idle(win)
	app.WaitWindow("loose.txt - " + filepath.Base(root))
	app.WaitNode(win, "text before the change", func(n Node) bool {
		return n.Role == "textField" && strings.Contains(n.Value, "before")
	})

	if err := os.WriteFile(outside, []byte("after\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	app.WaitNode(win, "text after the change", func(n Node) bool {
		return n.Role == "textField" && strings.Contains(n.Value, "after")
	})
}
