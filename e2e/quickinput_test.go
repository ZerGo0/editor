//go:build e2e

package e2e

import (
	"math"
	"strings"
	"testing"
)

const (
	languageModeTitle = "Select Language Mode"
	edgeTolerance     = 1.0
)

func TestQuickPickIsAsWideAsTheCommandCenter(t *testing.T) {
	t.Parallel()
	app, win, _ := openProject(t, map[string]string{"a.txt": "alpha\nbeta\n"})
	openInEditor(t, app, win, "a.txt")
	bar := app.WaitNode(win, "command center", RoleLabel("button", commandCenterLabel))
	status := app.WaitNode(win, "language status item", func(n Node) bool {
		return n.Role == "button" && strings.Contains(n.Description, languageModeTitle)
	})
	app.Click(win, status, 1)
	panel := app.WaitNode(win, "language mode quick pick", RoleLabel("dialog", languageModeTitle))
	if math.Abs(panel.Bounds.X-bar.Bounds.X) > edgeTolerance || math.Abs(panel.Bounds.Width-bar.Bounds.Width) > edgeTolerance {
		t.Fatalf("quick pick x %v width %v, command center x %v width %v; want the same edges", panel.Bounds.X, panel.Bounds.Width, bar.Bounds.X, bar.Bounds.Width)
	}
	if panel.Bounds.Y < bar.Bounds.Y+bar.Bounds.Height {
		t.Fatalf("quick pick at y %v overlaps the command center above it (bottom %v)", panel.Bounds.Y, bar.Bounds.Y+bar.Bounds.Height)
	}
}
