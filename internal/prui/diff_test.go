package prui

import (
	"errors"
	"testing"

	"github.com/ZerGo0/editor/internal/github"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	diffPath   = "src/widget.go"
	diffViewID = "pr/acme/widgets/12/diff/src/widget.go"
	headText   = "package w\n\nfunc A() {}\n"
	baseText   = "package w\n\nfunc B() {}\n"
)

func (hs *harness) withDiffFixtures() {
	if _, ok := hs.be.contents[testHeadKey()]; !ok {
		hs.be.contents[testHeadKey()] = headText
	}
	if _, ok := hs.be.contents["basesha:"+diffPath]; !ok {
		hs.be.contents["basesha:"+diffPath] = baseText
	}
	hs.be.threads = []github.Thread{
		{ID: "t-open", Path: diffPath, Line: 3, Side: github.SideRight, Comments: []github.Comment{{ID: "c1", Author: "carol", Body: "Please rename this function", Created: testNow}}},
		{ID: "t-done", Path: diffPath, Line: 3, Side: github.SideLeft, Resolved: true, Comments: []github.Comment{{ID: "c2", Author: "dave", Body: "Old note", Created: testNow}}},
		{ID: "t-other", Path: "other.go", Line: 1, Side: github.SideRight, Comments: []github.Comment{{ID: "c3", Author: "erin", Body: "Elsewhere", Created: testNow}}},
	}
}

func testHeadKey() string { return "headsha:" + diffPath }

func (hs *harness) openDiffTab(t *testing.T) (*prData, *diffTab, *ui.Tester) {
	t.Helper()
	hs.withDiffFixtures()
	d, detail := hs.openDetail(t)
	mustClick(t, detail, diffPath)
	if n := len(hs.h.opened); hs.h.opened[n-1] != diffViewID {
		t.Fatalf("opened %v", hs.h.opened)
	}
	tab := hs.p.diffs[diffViewID]
	hs.h.pump(t, func() bool { return tab.ready || tab.err != nil })
	return d, tab, viewTester(hs, diffViewID)
}

func TestDiffTabLoadsBothVersionsAndThreads(t *testing.T) {
	hs := newHarness(t)
	d, tab, tt := hs.openDiffTab(t)
	if tab.err != nil {
		t.Fatal(tab.err)
	}
	if tab.state.Old.String() != baseText || tab.state.New.String() != headText {
		t.Errorf("old %q new %q", tab.state.Old.String(), tab.state.New.String())
	}
	if d.baseSHA != "basesha" {
		t.Errorf("base sha %q", d.baseSHA)
	}
	if title := hs.h.titles[diffViewID]; title != "widget.go (#12)" {
		t.Errorf("title %q", title)
	}
	mustHaveText(t, tt, diffPath, "+5", "-2", "Please rename this function", "carol", "Reply", "Resolve", "Resolved", "Show thread")
	if tt.HasText("Elsewhere") {
		t.Error("thread of another file shown")
	}
	if tt.HasText("Old note") {
		t.Error("resolved thread is not collapsed")
	}
	if len(tab.state.Annotations) != 2 {
		t.Errorf("annotations %+v", tab.state.Annotations)
	}
}

func TestDiffTabPrefersLocalContents(t *testing.T) {
	hs := newHarness(t)
	hs.local.files[testHeadKey()] = "local head\n"
	hs.local.files["basesha:"+diffPath] = "local base\n"
	_, tab, _ := hs.openDiffTab(t)
	if tab.state.New.String() != "local head\n" || tab.state.Old.String() != "local base\n" {
		t.Errorf("old %q new %q", tab.state.Old.String(), tab.state.New.String())
	}
	if hs.be.called("FileAt") != 0 {
		t.Error("the API was used although git had the files")
	}
}

func TestDiffTabShowsResolvedThreadOnDemand(t *testing.T) {
	hs := newHarness(t)
	_, tab, tt := hs.openDiffTab(t)
	mustClick(t, tt, "Show thread")
	mustHaveText(t, tt, "Old note", "Unresolve")
	if !tab.expanded["t-done"] {
		t.Error("thread not expanded")
	}
}

func TestDiffTabResolvesThread(t *testing.T) {
	hs := newHarness(t)
	d, _, tt := hs.openDiffTab(t)
	mustClick(t, tt, "Resolve")
	hs.h.pump(t, func() bool { return hs.be.called("Resolve") == 1 && d.busy == "" })
	if !d.threads[0].Resolved {
		t.Error("thread not marked resolved")
	}
}

func TestDiffTabReplies(t *testing.T) {
	hs := newHarness(t)
	d, _, tt := hs.openDiffTab(t)
	mustClick(t, tt, "Reply")
	mustHaveText(t, tt, "Reply to carol")
	hs.p.submitDialog()
	if hs.p.dialog.err != "Write a reply first." {
		t.Errorf("dialog error %q", hs.p.dialog.err)
	}
	hs.p.dialog.body = "Done, thanks"
	hs.p.submitDialog()
	hs.h.pump(t, func() bool { return hs.be.called("Reply") == 1 && !hs.p.dialog.open })
	if hs.be.replied != "t-open:Done, thanks" || len(d.threads[0].Comments) != 2 {
		t.Errorf("replied %q comments %d", hs.be.replied, len(d.threads[0].Comments))
	}
}

func TestDiffTabCommentStartsPendingReview(t *testing.T) {
	hs := newHarness(t)
	d, tab, _ := hs.openDiffTab(t)
	tab.state.OnGutterButton(editor.DiffLineRange{Side: editor.DiffNew, Start: 1, End: 2})
	if hs.p.dialog.title != "Comment on src/widget.go:3" {
		t.Fatalf("title %q", hs.p.dialog.title)
	}
	hs.p.submitDialog()
	if hs.p.dialog.err == "" {
		t.Error("an empty comment was accepted")
	}
	hs.p.dialog.body = "Why?"
	hs.p.submitDialog()
	hs.h.pump(t, func() bool { return !hs.p.dialog.open && d.pendingReview != "" })
	want := github.ReviewCommentOptions{Body: "Why?", Path: diffPath, Line: 3, StartLine: 2, Side: github.SideRight}
	if hs.be.lineOpts != want || hs.be.called("StartReview") != 1 || hs.be.called("AddReviewComment:review-1") != 1 {
		t.Errorf("opts %+v calls %v", hs.be.lineOpts, hs.be.calls)
	}
	if len(d.threads) != 4 || d.threads[3].ID != "t-new" {
		t.Errorf("threads %d", len(d.threads))
	}
	tab.state.OnGutterButton(editor.DiffLineRange{Side: editor.DiffOld, Start: 0, End: 0})
	hs.p.dialog.body = "Second"
	hs.p.submitDialog()
	hs.h.pump(t, func() bool { return !hs.p.dialog.open && len(d.threads) == 5 })
	if hs.be.called("StartReview") != 1 || hs.be.lineOpts.Side != github.SideLeft {
		t.Errorf("review restarted: calls %v opts %+v", hs.be.calls, hs.be.lineOpts)
	}
}

func TestDiffTabCommentRefusedOnClosedPR(t *testing.T) {
	hs := newHarness(t)
	hs.be.pr.State = github.StateClosed
	_, tab, _ := hs.openDiffTab(t)
	tab.state.OnGutterButton(editor.DiffLineRange{Side: editor.DiffNew, End: 0})
	if hs.p.dialog.open || len(hs.h.notices) == 0 {
		t.Error("comment dialog opened on a closed pull request")
	}
}

func TestDiffTabBinaryFile(t *testing.T) {
	hs := newHarness(t)
	hs.be.contents[testHeadKey()] = "a\x00b"
	_, tab, tt := hs.openDiffTab(t)
	if !errors.Is(tab.err, ErrBinaryFile) {
		t.Fatalf("err %v", tab.err)
	}
	mustHaveText(t, tt, "Cannot show "+diffPath)
	if _, ok := tt.AccessByRole(ui.RoleButton, "Retry"); ok {
		t.Error("retry offered for a binary file")
	}
}

func TestDiffTabErrorOffersRetry(t *testing.T) {
	hs := newHarness(t)
	hs.be.errs["FileAt"] = errors.New("github: down")
	hs.withDiffFixtures()
	hs.be.contents = map[string]string{}
	_, tab, tt := hs.openDiffTab(t)
	if tab.err == nil {
		t.Fatal("expected an error")
	}
	mustHaveText(t, tt, "Cannot show "+diffPath, "down", "Retry")
	hs.be.clearErr("FileAt")
	hs.be.contents[testHeadKey()] = headText
	hs.be.contents["basesha:"+diffPath] = baseText
	mustClick(t, tt, "Retry")
	hs.h.pump(t, func() bool { return tab.ready })
}

func TestDiffTabAddedAndRemovedFiles(t *testing.T) {
	hs := newHarness(t)
	hs.be.files = []github.PRFile{
		{Path: "new.txt", Status: statusAdded, Additions: 1},
		{Path: "gone.txt", Status: statusRemoved, Deletions: 1},
		{Path: "dst.txt", PrevPath: "src.txt", Status: statusRenamed},
	}
	hs.be.contents["headsha:new.txt"] = "fresh\n"
	hs.be.contents["basesha:gone.txt"] = "old\n"
	hs.be.contents["headsha:dst.txt"] = "same\n"
	hs.be.contents["basesha:src.txt"] = "same\n"
	d, _ := hs.openDetail(t)
	for _, f := range d.files {
		hs.p.openDiff(d, f, diffSpec{mode: modePR})
		tab := hs.p.diffs[diffID(hs.p.repo, 12, f.Path)]
		hs.h.pump(t, func() bool { return tab.ready || tab.err != nil })
		if tab.err != nil {
			t.Errorf("%s: %v", f.Path, tab.err)
		}
	}
	if hs.be.called("MergeBase") != 1 {
		t.Errorf("merge base resolved %d times", hs.be.called("MergeBase"))
	}
}

func TestDiffLayoutAndChangeNavigation(t *testing.T) {
	hs := newHarness(t)
	_, tab, tt := hs.openDiffTab(t)
	mustClick(t, tt, "Unified")
	if tab.layout != layoutInline || tab.state.Layout != editor.DiffUnified {
		t.Errorf("layout %d/%v", tab.layout, tab.state.Layout)
	}
	if _, ok := tt.AccessByRole(ui.RoleButton, "Next Change"); !ok {
		t.Error("no change navigation")
	}
}

func TestDiffTabShowsWhitespaceAndMoveControls(t *testing.T) {
	hs := newHarness(t)
	_, tab, tt := hs.openDiffTab(t)
	if tab.engine == nil || !tab.engine.Ready() {
		t.Fatal("the diff engine is not ready")
	}
	mustHaveText(t, tt, "Do not ignore", "Show Moved Code")
}
