package workspace

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/chrome"
	"github.com/ZerGo0/editor/internal/search/finder"
	"github.com/ZerGo0/editor/internal/session"
	"github.com/egoist/mygo/ui"
)

func TestTrafficLightsCentreInTitleBar(t *testing.T) {
	p := TrafficLightPosition()
	if p == nil || 2*p.Y+trafficLightSize != TitleBarHeight || p.X != p.Y {
		t.Fatalf("traffic lights at %+v in a %d px title bar, want them centred with an equal left inset", p, TitleBarHeight)
	}
}

func TestCommandCenterIsCenteredFromTheFirstFrame(t *testing.T) {
	w, _ := newGuardBed(t)
	var laidOut []ui.Rect
	tt := ui.NewTester(func(c *ui.Context) {
		w.View(c)
		laidOut = append(laidOut, chrome.CommandCenterBounds(c))
	}, testWideW, testViewH)
	tt.Frame()
	first, settled := laidOut[1], laidOut[len(laidOut)-1]
	if first != settled {
		t.Fatalf("command center after the first frame at %+v, settles at %+v", first, settled)
	}
}

func TestCmdClickOnTheCommandCenterShowsThePathOfTheFile(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "a.txt")
	if err := os.WriteFile(file, []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	w := newTestWorkspace(t, root, newTestEnv(t), session.Project{})
	action.SetCommands(searchCommands{run: func() { w.OpenSearch(finder.TabAll) }})
	t.Cleanup(func() { action.SetCommands(nil) })
	w.OpenFile(file, false, true)
	tt := ui.NewTester(w.View, testWideW, testViewH)
	tt.Frame()
	if err := tt.ClickWith(ui.Cmd, "Search Everywhere"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	menu := tt.Menu()
	if w.search.dialog.IsOpen() || len(menu) < 3 || !slices.Equal(menu[:2], []string{"a.txt", filepath.Base(root)}) || menu[len(menu)-1] != "/" {
		t.Fatalf("Cmd-click menu = %v, search open %v", menu, w.search.dialog.IsOpen())
	}
}
