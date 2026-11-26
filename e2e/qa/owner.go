package qa

import "strings"

const (
	OwnerWorkspace = "internal/workspace"
	OwnerSearch    = "internal/search"
	OwnerGit       = "internal/gitui"
	OwnerGitHub    = "internal/prui, internal/issueui"
	OwnerPrefs     = "internal/prefs"
	OwnerPalette   = "internal/palette"
	OwnerViewer    = "internal/viewer"
	OwnerLargeFile = "internal/largefile"
	OwnerHistory   = "internal/history"
	OwnerApp       = "internal/app"
)

var viewOwners = map[string]string{
	"explorer":    OwnerWorkspace,
	"tabs":        OwnerWorkspace,
	"statusbar":   OwnerWorkspace,
	"panel":       OwnerWorkspace,
	"titlebar":    OwnerWorkspace,
	"editor":      OwnerWorkspace,
	"search":      OwnerSearch,
	"scm":         OwnerGit,
	"graph":       OwnerGit,
	"github":      OwnerGitHub,
	"settings":    OwnerPrefs,
	"keybindings": OwnerPrefs,
	"quickinput":  OwnerPalette,
	"viewer":      OwnerViewer,
	"largefile":   OwnerLargeFile,
	"history":     OwnerHistory,
	"welcome":     OwnerApp,
}

const (
	titleBarBand  = 40.0
	statusBarBand = 24.0
)

func ChromeZone(b Rect, window Rect) bool {
	return b.Bottom() <= titleBarBand || (window.Height > 0 && b.Y >= window.Height-statusBarBand)
}

func Owner(view string, n Node, window Rect) string {
	if ChromeZone(n.Bounds, window) && view != "welcome" && !n.Bounds.Empty() {
		return OwnerWorkspace
	}
	if owner, ok := viewOwners[view]; ok {
		return owner
	}
	return strings.TrimSpace(view)
}
