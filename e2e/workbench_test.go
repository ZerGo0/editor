//go:build e2e

package e2e

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	testImageSize = 8
	commitSubject = "Add main func"
)

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func gitProject(t *testing.T, files map[string]string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	root := Project(t, files)
	runGit(t, root, "init", "-q", "-b", "main")
	runGit(t, root, "config", "user.name", "E2E")
	runGit(t, root, "config", "user.email", "e2e@example.invalid")
	runGit(t, root, "config", "commit.gpgsign", "false")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-q", "-m", "init")
	return root
}

func pngData(t *testing.T) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, testImageSize, testImageSize))
	for x := range testImageSize {
		for y := range testImageSize {
			img.Set(x, y, color.RGBA{R: 200, G: uint8(x * 30), B: uint8(y * 30), A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestSourceControlCommitsChange(t *testing.T) {
	root := gitProject(t, map[string]string{"main.go": "package main\n"})
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	app := Launch(t, root)
	win := app.WaitWindow(filepath.Base(root))
	app.Idle(win)
	app.Menu(win, "View", "Source Control")
	app.WaitNode(win, "changed file", func(n Node) bool { return n.Label == "main.go" && n.Role != "treeItem" })
	field := app.WaitNode(win, "commit message", Label("Commit Message"))
	app.Act(win, field.ID, "focus", "")
	app.Type(win, commitSubject)
	commit := app.WaitNode(win, "commit button", func(n Node) bool { return n.Role == "button" && n.Label == "Commit" && !n.Has("disabled") })
	app.Click(win, commit, 1)
	app.Eventually("commit in history", func() bool {
		return runGit(t, root, "log", "-1", "--format=%s") == commitSubject
	})
}

func TestImageOpensViewer(t *testing.T) {
	app, win, _ := openProject(t, map[string]string{"shot.png": pngData(t)})
	row := app.WaitNode(win, "shot.png row", RoleLabel("treeItem", "shot.png"))
	app.Click(win, row, 1)
	app.WaitNode(win, "shot.png tab", RoleLabel("tab", "shot.png"))
	app.WaitNode(win, "viewer toolbar", Label("Viewer actions"))
	app.WaitNode(win, "zoom control", Label("Zoom In"))
}

func TestGitHubViewShowsOneEmptyStateWithoutRepository(t *testing.T) {
	app, win, _ := openProject(t, map[string]string{"main.go": "package main\n"})
	app.Menu(win, "View", "GitHub")
	app.WaitNode(win, "empty state", Label("No Git repository"))
	app.Idle(win)
	for _, n := range app.Tree(win) {
		switch n.Label {
		case "Pull Requests", "Issues":
			t.Errorf("section %q drawn behind the view-level empty state", n.Label)
		}
	}
}
