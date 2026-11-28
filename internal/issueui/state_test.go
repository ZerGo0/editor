package issueui

import (
	"testing"

	"github.com/ZerGo0/editor/internal/ghshared/ghtest"
	"github.com/ZerGo0/editor/internal/session"
	"github.com/egoist/mygo/ui"
)

const (
	restoredIssueID     = "issue/acme/widgets/5"
	otherRepoIssueID    = "issue/other/repo/5"
	restoredIssueNumber = 5
	restoredIssueTitle  = "Crash on start"
)

func TestSideOpenStateRoundTripsThroughProjectState(t *testing.T) {
	hs := newHarness(t)
	hs.ready(t)
	first, second := hs.i.side.sections[0], hs.i.side.sections[1]
	first.open, second.open = false, true
	snapshot := hs.h.ProjectState().Snapshot()

	next := newHarness(t)
	next.h.Project = session.NewMemento(snapshot)
	next.ready(t)
	got := next.i.side.sections
	if got[0].open || !got[1].open {
		t.Fatalf("open states %v %v, want false true", got[0].open, got[1].open)
	}
	next.i.SetSettings(DefaultSettings())
	if got := next.i.side.sections; got[0].open || !got[1].open {
		t.Fatalf("new settings lost the open states: %v %v", got[0].open, got[1].open)
	}
}

func TestRestoreIssueDetailRejectsOtherViews(t *testing.T) {
	hs := newHarness(t)
	hs.start(t)
	restore := hs.h.Restorers[detailIDPrefix+"/"]
	if restore == nil {
		t.Fatal("no restorer for issue tabs")
	}
	for _, id := range []string{"issue/acme/widgets", "issue/acme/widgets/0", restoredIssueID + "/x"} {
		if _, ok := restore(id, ""); ok {
			t.Errorf("restored %q", id)
		}
	}
}

func TestRestoredIssueDetailShowsSkeletonThenLoads(t *testing.T) {
	hs := newHarness(t)
	hs.be.issues[restoredIssueNumber] = testIssue(restoredIssueNumber, restoredIssueTitle)
	hs.i = NewWith(hs.h, hs.env)
	t.Cleanup(hs.i.Close)
	view, ok := hs.h.Restorers[detailIDPrefix+"/"](restoredIssueID, "")
	if !ok {
		t.Fatal("issue tab not restored")
	}
	tt := ui.NewTester(view, ghtest.DetailWidth, ghtest.DetailHeight)
	tt.Frame()
	ghtest.MustHaveText(t, tt, "Loading #5...")
	hs.h.Pump(t, hs.i.ready)
	tt.Frame()
	hs.h.Pump(t, func() bool { d := hs.i.details[restoredIssueNumber]; return d != nil && d.loaded })
	tt.Frame()
	ghtest.MustHaveText(t, tt, restoredIssueTitle)
	if hs.h.Views[restoredIssueID] == nil {
		t.Fatal("the loaded detail did not replace the restored view")
	}
}

func TestRestoredIssueDetailFromOtherRepoShowsError(t *testing.T) {
	hs := newHarness(t)
	hs.ready(t)
	view, ok := hs.h.Restorers[detailIDPrefix+"/"](otherRepoIssueID, "")
	if !ok {
		t.Fatal("issue tab not restored")
	}
	tt := ui.NewTester(view, ghtest.DetailWidth, ghtest.DetailHeight)
	tt.Frame()
	ghtest.MustHaveText(t, tt, errOtherRepo.Error())
	if hs.be.Count("GetIssue") != 0 {
		t.Fatal("loaded an issue from another repository")
	}
}
