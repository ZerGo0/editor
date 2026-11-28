//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZerGo0/editor/internal/testutil"
)

const (
	hangingUploadPack = "while true; do sleep 1; done; git-upload-pack"
	hookPerm          = 0o755
	progressMessage   = "Fix the bug"
	cancelFetchLabel  = "Cancel Fetch"
)

func changedGitProject(t *testing.T) string {
	t.Helper()
	root := gitProject(t, map[string]string{"main.go": "package main\n"})
	testutil.WriteFile(t, filepath.Join(root, "main.go"), "package main\n\nfunc main() {}\n")
	return root
}

func typeCommitMessage(app *App, win int) {
	app.t.Helper()
	app.Menu(win, "View", "Source Control")
	field := app.WaitNode(win, "commit message", Label("Commit Message"))
	app.Act(win, field.ID, "focus", "")
	app.Type(win, progressMessage)
}

func TestFetchCancelKeepsCommitAvailable(t *testing.T) {
	t.Parallel()
	root := changedGitProject(t)
	remote := t.TempDir()
	testutil.RunGit(t, remote, "init", "-q", "--bare")
	testutil.RunGit(t, root, "remote", "add", "origin", remote)
	testutil.RunGit(t, root, "config", "remote.origin.uploadpack", hangingUploadPack)
	app := Launch(t, root)
	win := app.WaitWindow(filepath.Base(root))
	app.Idle(win)
	typeCommitMessage(app, win)
	app.Menu(win, "Git", "Fetch")
	cancel := app.WaitNode(win, "cancel fetch", RoleLabel("button", cancelFetchLabel))
	app.WaitNode(win, "commit enabled during the fetch", commitButton(false))
	app.Click(win, cancel, 1)
	app.Eventually("fetch stops", func() bool {
		_, running := app.Find(win, Label(cancelFetchLabel))
		return !running
	})
	if n, failed := app.Find(win, func(n Node) bool { return strings.Contains(n.Label, "Fetch failed") }); failed {
		t.Fatalf("a cancelled fetch shows a failure: %q", n.Label)
	}
}

func TestCommitShowsProgressWhileHookRuns(t *testing.T) {
	t.Parallel()
	root := changedGitProject(t)
	release := filepath.Join(t.TempDir(), "release")
	hook := "#!/bin/sh\nwhile [ ! -f '" + release + "' ]; do sleep 0.05; done\n"
	if err := os.WriteFile(filepath.Join(root, ".git", "hooks", "pre-commit"), []byte(hook), hookPerm); err != nil {
		t.Fatal(err)
	}
	app := Launch(t, root)
	win := app.WaitWindow(filepath.Base(root))
	app.Idle(win)
	typeCommitMessage(app, win)
	app.Click(win, app.WaitNode(win, "commit button", commitButton(false)), 1)
	app.WaitNode(win, "commit progress", Label("Commit..."))
	app.WaitNode(win, "commit button disabled", commitButton(true))
	testutil.WriteFile(t, release, "")
	app.Eventually("commit in history", func() bool {
		return testutil.RunGit(t, root, "log", "-1", "--format=%s") == progressMessage
	})
}

func TestCloneRepositoryOpensInNewWindow(t *testing.T) {
	t.Parallel()
	src := gitProject(t, map[string]string{"README.md": "# widgets\n"})
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := gitProject(t, map[string]string{"main.go": "package main\n"})
	app := LaunchEnv(t, t.TempDir(), pickedEnv(parent), root)
	win := app.WaitWindow(filepath.Base(root))
	app.Idle(win)
	runPalette(app, win, "git clone", "Git: Clone")
	app.WaitNode(win, "repository URL field", Label("Repository URL"))
	app.Type(win, "file://"+src)
	app.Key(win, "Enter")
	name := filepath.Base(src)
	app.WaitNode(win, "clone notice", func(n Node) bool { return strings.Contains(n.Label, "Cloned "+name) })
	app.Click(win, app.WaitNode(win, "open action", RoleLabel("button", "Open in New Window")), 1)
	app.WaitWindow(name)
	if _, err := os.Stat(filepath.Join(parent, name, "README.md")); err != nil {
		t.Fatal(err)
	}
}

func commitButton(disabled bool) func(Node) bool {
	return func(n Node) bool { return RoleLabel("button", "Commit")(n) && n.Has("disabled") == disabled }
}
