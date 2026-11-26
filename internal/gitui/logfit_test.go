package gitui

import (
	"strings"
	"testing"
	"time"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/git"
	"github.com/egoist/mygo/ui"
)

const (
	narrowPanelW  = 470
	widePanelW    = 1100
	longSubject   = "fix(settings): use a shorter timeout and a narrower sidebar"
	longBranch    = "origin/bugfix/JIRA-12345-fix-overflowing-labels-in-the-source-control-sidebar-view"
	longAuthor    = "Margaret Hamilton-Wallace-Montgomery"
	subjectShare  = 0.6
	panelHeightPx = 400
)

func logFixture(t *testing.T) (*Git, *fakeHost) {
	t.Helper()
	now := time.Now()
	r := &fakeRepo{
		status: &git.Status{Branch: "main", OID: "c2"},
		commits: []git.Commit{
			{OID: "c2", Parents: []string{"c1"}, Author: longAuthor, AuthorTime: now, Subject: longSubject, Refs: []string{"HEAD -> main", longBranch, "feature/second-long-branch-name"}},
			{OID: "c1", Author: "Ada", AuthorTime: now, Subject: "First change"},
		},
	}
	return newFakeGit(t, r)
}

func openLog(t *testing.T, width int) *ui.Tester {
	t.Helper()
	g, h := logFixture(t)
	tt := ui.NewTester(g.LogPanel, width, panelHeightPx)
	h.drain(t, func() bool { tt.Frame(); return tt.HasText(longSubject) || tt.HasText("First change") })
	tt.Frame()
	tt.Frame()
	return tt
}

func TestLogFilterBarNeverForcesThePanelWider(t *testing.T) {
	tt := openLog(t, narrowPanelW)
	bar, ok := tt.AccessByRole(ui.RoleToolbar, "Log Filters")
	if !ok {
		t.Fatal("no Log Filters toolbar")
	}
	if bar.Bounds.W > narrowPanelW {
		t.Errorf("the toolbar is %.1f px wide in a %d px panel", bar.Bounds.W, narrowPanelW)
	}
	for _, name := range []string{"Filter by Text", "Filter by User", "Filter by Path"} {
		in, ok := tt.AccessByRole(ui.RoleTextField, name)
		if !ok {
			t.Fatalf("no %s field", name)
		}
		if in.Bounds.X+in.Bounds.W > narrowPanelW {
			t.Errorf("%s ends at %.1f, outside the %d px panel", name, in.Bounds.X+in.Bounds.W, narrowPanelW)
		}
	}
}

func TestLogFilterBarSitsOnWholePixels(t *testing.T) {
	tt := openLog(t, widePanelW)
	bar, _ := tt.AccessByRole(ui.RoleToolbar, "Log Filters")
	branch, ok := tt.AccessByRole(ui.RolePopUpButton, "Filter by Branch")
	if !ok {
		t.Fatal("no branch filter")
	}
	for name, v := range map[string]float32{"toolbar y": bar.Bounds.Y, "toolbar h": bar.Bounds.H, "branch y": branch.Bounds.Y, "branch h": branch.Bounds.H} {
		if v != float32(int(v)) {
			t.Errorf("%s is %v, not a whole pixel", name, v)
		}
	}
	above, below := branch.Bounds.Y-bar.Bounds.Y, bar.Bounds.Y+bar.Bounds.H-(branch.Bounds.Y+branch.Bounds.H)
	if above != below {
		t.Errorf("the branch filter is not centred in the toolbar: %v above, %v below", above, below)
	}
}

func TestLogRowsKeepTheSubjectAndTruncateAuthor(t *testing.T) {
	tt := openLog(t, widePanelW)
	subject, ok := tt.Find(longSubject)
	if !ok {
		t.Fatalf("no subject row: %v", tt.Texts())
	}
	text, _ := ui.ShapeText(longSubject, ui.Font{Size: token.TextBody.Px()}, 0).Size()
	if subject.W < text*subjectShare {
		t.Errorf("the subject keeps %.1f px of %.1f", subject.W, text)
	}
}

func TestLogColumnsDropWhatDoesNotFit(t *testing.T) {
	s := &logState{}
	if got := s.columns(0, 3); got != (logColumns{}) {
		t.Errorf("an unmeasured list shows only the subject: %+v", got)
	}
	s.listWidth = narrowPanelW
	if got := s.columns(token.GraphLaneWidth.Px(), 3); got.author || got.date || got.hash {
		t.Errorf("a narrow list drops the author, date and hash columns: %+v", got)
	}
	s.listWidth = float32(widePanelW)
	got := s.columns(token.GraphLaneWidth.Px(), 3)
	if !got.author || !got.date || !got.hash || got.tags == 0 {
		t.Errorf("a wide list shows every column: %+v", got)
	}
}

func TestRefTagsAreWholePixelsAndShort(t *testing.T) {
	if got := refLabel(longBranch); len([]rune(got)) != refNameMax || !strings.HasSuffix(got, refNameEllipsis) {
		t.Errorf("refLabel = %q", got)
	}
	if got := refLabel("main"); got != "main" {
		t.Errorf("short names stay whole: %q", got)
	}
	tt := openLog(t, widePanelW)
	for _, a := range tt.AccessAll() {
		if a.Role != ui.RoleGroup || !strings.HasSuffix(a.Name, refNameEllipsis) && !strings.Contains(a.Name, "main") {
			continue
		}
		if a.Bounds.W != float32(int(a.Bounds.W)) {
			t.Errorf("tag %q is %v px wide", a.Name, a.Bounds.W)
		}
	}
}
