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

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/testutil"
)

const (
	testImageSize = 8
	commitSubject = "Add main func"
)

func gitProject(t *testing.T, files map[string]string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	root := Project(t, files)
	testutil.RunGit(t, root, "init", "-q", "-b", "main")
	testutil.RunGit(t, root, "config", "user.name", "E2E")
	testutil.RunGit(t, root, "config", "user.email", "e2e@example.invalid")
	testutil.RunGit(t, root, "config", "commit.gpgsign", "false")
	testutil.RunGit(t, root, "add", ".")
	testutil.RunGit(t, root, "commit", "-q", "-m", "init")
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
	t.Parallel()
	root := gitProject(t, map[string]string{"main.go": "package main\n"})
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	app := Launch(t, root)
	win := app.WaitWindow(filepath.Base(root))
	app.Idle(win)
	app.Menu(win, "View", "Source Control")
	app.WaitNode(win, "changed file", func(n Node) bool { return hasName(n.Label, "main.go") && n.Role != "treeItem" })
	field := app.WaitNode(win, "commit message", Label("Commit Message"))
	app.Act(win, field.ID, "focus", "")
	app.Type(win, commitSubject)
	commit := app.WaitNode(win, "commit button", commitButton(false))
	app.Click(win, commit, 1)
	app.Eventually("commit in history", func() bool {
		return testutil.RunGit(t, root, "log", "-1", "--format=%s") == commitSubject
	})
}

func TestImageOpensViewer(t *testing.T) {
	t.Parallel()
	app, win, _ := openProject(t, map[string]string{"shot.png": pngData(t)})
	row := app.WaitNode(win, "shot.png row", RoleLabel("treeItem", "shot.png"))
	app.Click(win, row, 1)
	app.WaitNode(win, "shot.png tab", RoleLabel("tab", "shot.png"))
	app.WaitNode(win, "viewer toolbar", Label("Viewer actions"))
	app.WaitNode(win, "zoom control", Label("Zoom In"))
}

func TestGitHubViewShowsOneEmptyStateWithoutRepository(t *testing.T) {
	t.Parallel()
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

func TestTitleBarContentCentresOnTrafficLights(t *testing.T) {
	t.Parallel()
	const tolerance = 0.01
	centre := float64(token.TitleBarHeight.Px()) / 2
	app, win, _ := openProject(t, map[string]string{"main.go": "package main\n"})
	for _, what := range []string{"Layout Controls", "Search Everywhere"} {
		n := app.WaitNode(win, what, func(n Node) bool { return strings.HasPrefix(n.Label, what) })
		if got := n.Bounds.Y + n.Bounds.Height/2; got < centre-tolerance || got > centre+tolerance {
			t.Errorf("%s centre y = %v, want %v, the traffic lights' centre", what, got, centre)
		}
	}
}

func TestViewActionsShareTheActivityBarRow(t *testing.T) {
	t.Parallel()
	root := gitProject(t, map[string]string{"main.go": "package main\n"})
	app := Launch(t, root)
	win := app.WaitWindow(filepath.Base(root))
	app.Idle(win)
	app.Menu(win, "View", "Source Control")
	bar := app.WaitNode(win, "activity bar", RoleLabel("tabList", "Activity Bar"))
	actions := app.WaitNode(win, "view actions", RoleLabel("toolbar", "Source Control Actions"))
	if top, bottom := bar.Bounds.Y, bar.Bounds.Y+bar.Bounds.Height; actions.Bounds.Y < top || actions.Bounds.Y+actions.Bounds.Height > bottom {
		t.Errorf("view actions at y %v..%v, want them in the activity bar row at y %v..%v", actions.Bounds.Y, actions.Bounds.Y+actions.Bounds.Height, top, bottom)
	}
	if bar.Bounds.X > actions.Bounds.X {
		t.Errorf("view actions at x %v, want them after the activity icons at x %v", actions.Bounds.X, bar.Bounds.X)
	}
	if n, ok := app.Find(win, RoleLabel("heading", "Source Control")); ok {
		t.Errorf("the side bar still shows a view title row: %+v", n)
	}
	app.WaitNode(win, "view region", RoleLabel("region", "Source Control"))
}
