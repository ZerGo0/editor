package prui

import (
	"testing"

	"github.com/ZerGo0/editor/internal/ghshared/ghtest"
	"github.com/ZerGo0/editor/internal/session"
	"github.com/egoist/mygo/ui"
)

const (
	savedSearch    = "is:draft"
	restoredPRID   = "pr/acme/widgets/12"
	otherRepoPRID  = "pr/other/repo/5"
	restoredPRName = "#12 Fix the widget"
)

func TestSideStateRoundTripsThroughProjectState(t *testing.T) {
	hs := newHarness(t)
	hs.ready(t)
	hs.p.side.all.open = true
	hs.p.side.review.open = false
	hs.p.runSearch(savedSearch)
	snapshot := hs.h.ProjectState().Snapshot()

	next := newHarness(t)
	next.h.Project = session.NewMemento(snapshot)
	next.ready(t)
	s := next.p.side
	if !s.all.open || s.review.open {
		t.Fatalf("all open %v, review open %v", s.all.open, s.review.open)
	}
	if s.field != savedSearch || s.search == nil || s.search.query != savedSearch {
		t.Fatalf("search field %q, section %+v", s.field, s.search)
	}
}

func TestRestoreDetailRejectsOtherViews(t *testing.T) {
	hs := newHarness(t)
	hs.start(t)
	restore := hs.h.Restorers[detailIDPrefix+"/"]
	if restore == nil {
		t.Fatal("no restorer for pull request tabs")
	}
	for _, id := range []string{createViewID, restoredPRID + "/" + diffIDSegment, "pr/acme/widgets/x"} {
		if _, ok := restore(id, ""); ok {
			t.Errorf("restored %q", id)
		}
	}
}

func TestRestoredDetailShowsSkeletonThenLoads(t *testing.T) {
	hs := newHarness(t)
	hs.p = NewWith(hs.h, hs.env)
	t.Cleanup(hs.p.Close)
	view, ok := hs.h.Restorers[detailIDPrefix+"/"](restoredPRID, restoredPRName)
	if !ok {
		t.Fatal("pull request tab not restored")
	}
	tt := ui.NewTester(view, ghtest.DetailWidth, ghtest.DetailHeight)
	tt.Frame()
	ghtest.MustHaveText(t, tt, "Loading #12...")
	hs.h.Pump(t, func() bool { return hs.p.Ready() })
	tt.Frame()
	hs.h.Pump(t, func() bool { return hs.p.details[12] != nil && hs.p.details[12].loaded })
	tt.Frame()
	ghtest.MustHaveText(t, tt, "Fix the widget")
	if hs.h.Views[restoredPRID] == nil {
		t.Fatal("the loaded detail did not replace the restored view")
	}
}

func TestRestoredDetailFromOtherRepoShowsError(t *testing.T) {
	hs := newHarness(t)
	hs.ready(t)
	view, ok := hs.h.Restorers[detailIDPrefix+"/"](otherRepoPRID, "")
	if !ok {
		t.Fatal("pull request tab not restored")
	}
	tt := ui.NewTester(view, ghtest.DetailWidth, ghtest.DetailHeight)
	tt.Frame()
	ghtest.MustHaveText(t, tt, errOtherRepo.Error())
	if hs.be.Count("Get") != 0 {
		t.Fatal("loaded a pull request from another repository")
	}
}
