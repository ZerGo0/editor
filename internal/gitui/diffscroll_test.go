package gitui

import (
	"strings"
	"testing"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/git/diffmode"
	"github.com/egoist/mygo/ui"
)

const (
	longFileLines = 200
	editEvery     = 5
	barTolerance  = 1.5
)

func longText(edit string) string {
	var b strings.Builder
	for i := range longFileLines {
		if i%editEvery == 0 {
			b.WriteString(edit)
		} else {
			b.WriteString("line of the long file")
		}
		b.WriteString("\n")
	}
	return b.String()
}

func openLongDiff(t *testing.T) (*diffTab, *ui.Tester) {
	t.Helper()
	if !git.Available() {
		t.Skip("git is not installed")
	}
	dir := newTempRepo(t)
	writeFile(t, dir, codePath, longText("before"))
	commitAll(t, dir, "init")
	writeFile(t, dir, codePath, longText("after"))
	useSession(t, diffmode.Options{})
	g, h := newRealGit(t, dir)
	g.openDiff(diffWorkTree, codePath)
	tab := g.tabs[diffSource{kind: diffWorkTree, path: codePath}.id()]
	h.drain(t, func() bool { return tab.engine != nil && tab.engine.Ready() })
	tt := newTester(g, tab.view)
	tt.Frame()
	tt.Frame()
	return tab, tt
}

func TestOldSideHasItsOwnScrollbar(t *testing.T) {
	tab, tt := openLongDiff(t)
	divider, ok := tt.AccessByRole(ui.RoleSplitter, "Divider")
	if !ok {
		t.Fatalf("no divider: %v", tt.Texts())
	}
	bar := tab.bar.bar
	if !bar.Scrolls() {
		t.Fatalf("the old side shows no scrollbar: %+v", bar)
	}
	if edge := bar.Box.X + bar.Box.W; edge < divider.Bounds.X-barTolerance || edge > divider.Bounds.X+barTolerance {
		t.Errorf("old side bar ends at %.1f, the divider is at %.1f", edge, divider.Bounds.X)
	}
	if bar.Box.H != float32(tab.state.ViewportHeight()) {
		t.Errorf("bar height %.1f, viewport %.1f", bar.Box.H, tab.state.ViewportHeight())
	}
}

func TestOldSideScrollbarScrollsBothSides(t *testing.T) {
	tab, tt := openLongDiff(t)
	bar, ok := tab.localBar()
	if !ok {
		t.Fatalf("no old side bar: %+v", tab.bar.bar)
	}
	_, length := bar.Thumb()
	room := tab.bar.bar.Box
	x := room.X + room.W - bar.Style.TrackWidth/2
	tt.Press(x, room.Y+length/2)
	tt.Move(x, room.Y+room.H-1)
	tt.Release(x, room.Y+room.H-1)
	tt.Frame()
	if tab.state.ScrollY() <= 0 {
		t.Fatalf("dragging the old side bar did not scroll: %.1f", tab.state.ScrollY())
	}
	if got := tab.state.ScrollY(); got < float64(bar.Content-bar.View)-1 {
		t.Errorf("dragging to the track end stopped at %.1f of %.1f", got, bar.Content-bar.View)
	}
}
