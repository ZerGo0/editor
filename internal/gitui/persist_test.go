package gitui

import (
	"context"
	"testing"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/git/diffmode"
	"github.com/ZerGo0/editor/internal/multidiff"
	"github.com/ZerGo0/editor/internal/session"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const savedDraft = "feat: keep the draft"

func restartedHost(t *testing.T, h *fakeHost) *fakeHost {
	t.Helper()
	next := newFakeHost(h.root)
	next.Project = session.NewMemento(h.ProjectState().Snapshot())
	next.Global = session.NewMemento(h.GlobalState().Snapshot())
	return next
}

func TestProjectStateRoundTripsThroughTheSession(t *testing.T) {
	r := &fakeRepo{status: &git.Status{Branch: "main", OID: "c1"}}
	g, h := newFakeGit(t, r)
	g.commit.message = savedDraft
	g.rememberMessage("fix: first")
	g.log.filter = logFilter{text: "fix", branch: "main", author: "Ada", path: "src"}
	g.viewed.SetViewed("git-changes:changes", &multidiff.Entry{Path: "a.txt", Fingerprint: "f1"}, true)
	g.changes.stack.Weights = map[string]float32{paneChanges: 120, paneGraph: 300}
	g.changes.stack.Collapsed = map[string]bool{paneGraph: true}
	g.log.split, g.log.details.split = 40, 70
	g.layout.DiffSplit, g.layout.TreeWidth, g.layout.TreeHidden = 30, 260, true
	g.layout.MergeSplit, g.layout.ResultSplit = 25, 60
	g.layout.Columns[colDate] = 150

	g2, _ := newFakeGitOn(t, r, restartedHost(t, h))
	if g2.commit.message != savedDraft || len(g2.commit.history) != 1 || g2.commit.history[0] != "fix: first" {
		t.Errorf("commit = %q %v", g2.commit.message, g2.commit.history)
	}
	if g2.log.filter != g.log.filter || g2.log.applied != g.log.filter {
		t.Errorf("log filter = %+v, applied %+v", g2.log.filter, g2.log.applied)
	}
	if !g2.viewed.IsViewed("git-changes:changes", &multidiff.Entry{Path: "a.txt", Fingerprint: "f1"}) {
		t.Error("the viewed mark is lost")
	}
	if g2.changes.stack.Weights[paneGraph] != 300 || !g2.changes.stack.Collapsed[paneGraph] {
		t.Errorf("sections = %+v", g2.changes.stack)
	}
	if g2.log.split != 40 || g2.log.details.split != 70 {
		t.Errorf("log splits = %v %v", g2.log.split, g2.log.details.split)
	}
	l := g2.layout
	if l.DiffSplit != 30 || l.TreeWidth != 260 || !l.TreeHidden || l.MergeSplit != 25 || l.ResultSplit != 60 || l.Columns.of(colDate) != 150 {
		t.Errorf("layout = %+v", l)
	}
}

func TestCommitClearsTheSavedDraft(t *testing.T) {
	r := &fakeRepo{status: &git.Status{Branch: "main", OID: "c1", Files: []git.FileStatus{{Path: "a.txt", Index: 'M', WorkTree: '.'}}}}
	g, h := newFakeGit(t, r)
	g.commit.message = savedDraft
	g.doCommit(commitSmart, afterNone)
	h.drain(t, func() bool { return r.called("commit") && g.commit.message == "" })
	var cm savedCommit
	if !session.NewMemento(h.ProjectState().Snapshot()).Load(commitStateKey, &cm) || cm.Message != "" || len(cm.History) != 1 {
		t.Errorf("saved commit = %+v", cm)
	}
}

func TestDefaultsWhenNothingWasSaved(t *testing.T) {
	g, _ := newFakeGit(t, &fakeRepo{status: &git.Status{Branch: "main"}})
	if g.layout.MergeSplit != mergeSplitDefault || g.layout.ResultSplit != resultSplitDefault || g.log.split != defaultLogSplit {
		t.Errorf("layout = %+v, log split %v", g.layout, g.log.split)
	}
	if g.diffOpts.layout != 0 || g.log.filter.branch != allBranches || g.changes.stack.Weights[paneChanges] != changesPaneWeight {
		t.Errorf("diff options %+v, filter %+v", g.diffOpts, g.log.filter)
	}
}

func TestDiffLayoutChoiceIsRememberedGlobally(t *testing.T) {
	useSession(t, diffmode.Options{ShowMoves: true})
	r := &fakeRepo{status: &git.Status{Branch: "main"}, shows: map[string]string{"HEAD:a.txt": "a\n", ":a.txt": "b\n"}}
	g, h := newFakeGit(t, r)
	tab := g.diffTabFor(diffSource{kind: diffStaged, path: "a.txt"})
	tt := newTester(g, tab.view)
	h.drain(t, func() bool { tt.Frame(); return tab.state != nil })
	if err := tt.Click(layoutNames[1]); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if g.diffOpts.layout != 1 {
		t.Fatalf("layout choice = %d", g.diffOpts.layout)
	}
	before := currentDiffChoice(tab.layout)
	diffmode.Session.Whitespace = git.WhitespaceIgnore
	g.noteDiffChoice(before, tab.layout)
	g.diffOpts.hideNumbers = true
	g.saveDiffPrefs()
	if other := g.diffTabFor(diffSource{kind: diffStaged, path: "b.txt"}); other.layout != 1 {
		t.Errorf("a new diff tab opens with layout %d", other.layout)
	}
	if v := g.newDiffView(); v.Layout() != 1 {
		t.Errorf("a new stacked diff opens with layout %d", v.Layout())
	}

	diffmode.Session.Whitespace = git.WhitespaceNone
	g2, _ := newFakeGitOn(t, r, restartedHost(t, h))
	if g2.diffOpts.layout != 1 || !g2.diffOpts.hideNumbers || diffmode.Session.Whitespace != git.WhitespaceIgnore || !diffmode.Session.ShowMoves {
		t.Errorf("after restart: options %+v, session %+v", g2.diffOpts, diffmode.Session)
	}
	if g2.diffOpts.movesSet {
		t.Errorf("unchosen options came back as chosen: %+v", g2.diffOpts)
	}
}

func TestStackedDiffRemembersLayoutAndTree(t *testing.T) {
	g, h, _ := stackRepo(t)
	g.openDiffWith(diffSource{kind: diffWorkTree, path: "a.txt"}, false, true)
	tab := loadedTab(t, g, h, changesTabID)
	tt := newTester(g, tab.render)
	tt.Frame()
	if err := tt.Click(layoutNames[1]); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if g.diffOpts.layout != 1 {
		t.Errorf("stacked diff layout choice = %d", g.diffOpts.layout)
	}
	if err := tt.Click("Toggle File Tree"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !g.layout.TreeHidden {
		t.Error("hiding the file tree is not remembered")
	}
	if _, shown := g.newDiffView().Tree(); shown {
		t.Error("a new stacked diff shows the hidden tree")
	}
}

func TestRestoredTabsWaitForTheRepository(t *testing.T) {
	r := &fakeRepo{status: &git.Status{Branch: "main", OID: "c1", Files: []git.FileStatus{{Path: "a.txt", Index: '.', WorkTree: 'M'}}}}
	r.dir = tempDir(t)
	h := newFakeHost(r.dir)
	release := make(chan struct{})
	g := newGit(h, deps{
		available: func() bool { return true },
		open: func(context.Context, string, string) (repo, error) {
			<-release
			return r, nil
		},
		initRepo: func(context.Context, string) error { return nil },
		watch:    func(context.Context, string, func()) {},
		ask:      func(*mygo.Window, question) bool { return true },
	})
	t.Cleanup(g.Close)
	for _, id := range []string{"git-compare:main...dev", "git-compare-local:main", "git-merge:a.txt"} {
		for prefix := range h.Restorers {
			if len(id) >= len(prefix) && id[:len(prefix)] == prefix {
				t.Errorf("%s would be restored by %s", id, prefix)
			}
		}
	}
	view, ok := h.Restorers[changesViewPrefix](changesTabID, "Changes")
	if !ok {
		t.Fatal("the Changes tab is not restored")
	}
	tt := newTester(g, view)
	if !tt.HasText("Loading changes") && len(tt.Texts()) == 0 {
		t.Errorf("no skeleton before the repository is ready: %v", tt.Texts())
	}
	if g.patchTabs[changesTabID].loading {
		t.Error("the tab loads before the repository is ready")
	}
	close(release)
	tab := g.patchTabs[changesTabID]
	h.drain(t, func() bool { tt.Frame(); return tab.loaded && !tab.loading })
	if got := tab.view.Entries(); len(got) != 1 || got[0].Path != "a.txt" {
		t.Errorf("entries = %+v", got)
	}
}

func TestRestoreParsesEveryRebuildableTab(t *testing.T) {
	g, h := newFakeGit(t, &fakeRepo{status: &git.Status{Branch: "main"}})
	for _, src := range []diffSource{
		{kind: diffWorkTree, path: "dir/a.txt"},
		{kind: diffStaged, path: "b.txt", oldPath: "old.txt"},
		{kind: diffCommit, path: "c.txt", rev: "abc123"},
		{kind: diffCompare, path: "d.txt", rev: "main"},
	} {
		got, ok := parseDiffID(src.id())
		if !ok || got != src {
			t.Errorf("parseDiffID(%q) = %+v, %v", src.id(), got, ok)
		}
	}
	for _, bad := range []string{"git-diff:x:::a", "git-diff:1::", "git-diff:99:::a"} {
		if _, ok := h.Restorers[diffViewPrefix](bad, ""); ok {
			t.Errorf("%q was restored", bad)
		}
	}
	cases := map[string]string{
		commitViewPrefix + "abc123":               commitViewPrefix,
		historyViewPrefix + "src/a.txt":           historyViewPrefix,
		linesViewPrefix + "src/a.txt:3-9":         linesViewPrefix,
		changesViewPrefix + groupIDs[groupStaged]: changesViewPrefix,
	}
	for id, prefix := range cases {
		if _, ok := h.Restorers[prefix](id, "Title"); !ok {
			t.Errorf("%s was dropped", id)
		}
	}
	if _, ok := h.Restorers[linesViewPrefix](linesViewPrefix+"a.txt:x", ""); ok {
		t.Error("a bad line range was restored")
	}
	if _, ok := g.histories[historyViewPrefix+"src/a.txt"]; !ok {
		t.Error("no history state for the restored history tab")
	}
}

func TestMergePanesResizeAndReset(t *testing.T) {
	r := &fakeRepo{
		status:   &git.Status{Branch: "main", OID: "a", Files: []git.FileStatus{{Path: "m.txt", Index: 'U', WorkTree: 'U'}}},
		versions: [3]string{"a\nb\nc\n", "a\nours\nc\n", "a\ntheirs\nc\n"},
	}
	g, h := newFakeGit(t, r)
	g.openMergeTool("m.txt")
	tool := g.mergeTools["git-merge:m.txt"]
	h.drain(t, func() bool { return tool.model != nil })
	tt := newTester(g, tool.view)
	tt.Frame()
	var dividers []ui.AccessInfo
	for _, a := range tt.AccessAll() {
		if a.Role == ui.RoleSplitter {
			dividers = append(dividers, a)
		}
	}
	if len(dividers) != 2 {
		t.Fatalf("merge dividers = %d", len(dividers))
	}
	b := dividers[0].Bounds
	x, y := b.X+b.W/2, b.Y+b.H/2
	tt.Press(x, y)
	tt.Move(x-60, y)
	tt.Move(x-120, y)
	tt.Release(x-120, y)
	tt.Frame()
	if g.layout.MergeSplit >= mergeSplitDefault {
		t.Fatalf("dragging the left divider left kept %v%%", g.layout.MergeSplit)
	}
	for _, a := range tt.AccessAll() {
		if a.Role == ui.RoleSplitter {
			b = a.Bounds
			break
		}
	}
	tt.ClickAt(b.X+b.W/2, b.Y+b.H/2)
	tt.ClickAt(b.X+b.W/2, b.Y+b.H/2)
	tt.Frame()
	if g.layout.MergeSplit != mergeSplitDefault {
		t.Errorf("a double click left %v%%, want %v", g.layout.MergeSplit, mergeSplitDefault)
	}
}

func TestLogColumnsResizeFromTheHeader(t *testing.T) {
	g, h := logFixture(t)
	tt := ui.NewTester(g.LogPanel, widePanelW, panelHeightPx)
	h.drain(t, func() bool { tt.Frame(); return tt.HasText(longSubject) })
	tt.Frame()
	sash, ok := tt.AccessByRole(ui.RoleSplitter, "Resize Author Column")
	if !ok {
		t.Fatalf("no author column sash")
	}
	head, _ := tt.Find("Author")
	cell, _ := tt.Find(longAuthor)
	if head.X != cell.X {
		t.Errorf("the Author header starts at %v, its cells at %v", head.X, cell.X)
	}
	b := sash.Bounds
	x, y := b.X+b.W/2, b.Y+b.H/2
	tt.Press(x, y)
	tt.Move(x-20, y)
	tt.Release(x-20, y)
	tt.Frame()
	want := authorWidth() + 20
	if got := g.layout.Columns.of(colAuthor); got != want {
		t.Fatalf("author width = %v, want %v", got, want)
	}
	if cell, _ := tt.Find(longAuthor); cell.W > want || cell.W < want-1 {
		t.Errorf("the author cell is %v wide", cell.W)
	}
	sash, _ = tt.AccessByRole(ui.RoleSplitter, "Resize Author Column")
	b = sash.Bounds
	tt.ClickAt(b.X+b.W/2, b.Y+b.H/2)
	tt.ClickAt(b.X+b.W/2, b.Y+b.H/2)
	tt.Frame()
	if g.layout.Columns[colAuthor] != 0 {
		t.Errorf("a double click left the author width at %v", g.layout.Columns[colAuthor])
	}
}

func TestLogColumnWidthsAreClamped(t *testing.T) {
	w := logWidths{1, 0, 1e6}.clamped()
	if w[colAuthor] != minColumnWidth() || w[colDate] != 0 || w[colHash] != maxColumnWidth() {
		t.Errorf("clamped = %v", w)
	}
}
