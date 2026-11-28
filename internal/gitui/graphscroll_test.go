package gitui

import (
	"fmt"
	"testing"
	"time"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/egoist/mygo/ui"
)

const (
	graphScrollCommits = 80
	graphScrollTarget  = 40
	graphNewCommit     = "Brand new commit"
	graphTopSubject    = "Commit 79"
)

func graphScrollRepo() *fakeRepo {
	now := time.Now()
	commits := make([]git.Commit, graphScrollCommits)
	for i := range commits {
		n := graphScrollCommits - 1 - i
		commits[i] = git.Commit{OID: fmt.Sprintf("c%d", n), Author: "Ada", AuthorTime: now, Subject: fmt.Sprintf("Commit %d", n)}
		if n > 0 {
			commits[i].Parents = []string{fmt.Sprintf("c%d", n-1)}
		}
	}
	return &fakeRepo{status: &git.Status{Branch: "main", OID: commits[0].OID}, commits: commits}
}

func addGraphCommit(r *fakeRepo, ahead int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	top := git.Commit{OID: "new", Parents: []string{r.commits[0].OID}, Author: "Ada", AuthorTime: time.Now(), Subject: graphNewCommit}
	r.commits = append([]git.Commit{top}, r.commits...)
	r.status = &git.Status{Branch: "main", OID: r.status.OID, Ahead: ahead}
}

func settleGraph(t *testing.T, g *Git, h *fakeHost, tt *ui.Tester, done func() bool) {
	t.Helper()
	g.refreshNow()
	h.drain(t, func() bool { tt.Frame(); return !g.refresh.running && !g.graph.loading && done() })
	tt.Frame()
	tt.Frame()
}

func openScrolledGraph(t *testing.T, scroll bool) (*Git, *fakeHost, *fakeRepo, *ui.Tester) {
	t.Helper()
	r := graphScrollRepo()
	g, h := newFakeGit(t, r)
	tt := ui.NewTester(g.SideView, sideWidth, sideHeightPx)
	h.drain(t, func() bool { tt.Frame(); return tt.HasText(graphTopSubject) })
	if scroll {
		g.graph.list.ScrollTo(graphScrollTarget, ui.Start)
		tt.Frame()
		tt.Frame()
	}
	return g, h, r, tt
}

func TestGraphAtTopShowsNewCommitsOnTop(t *testing.T) {
	t.Parallel()
	g, h, r, tt := openScrolledGraph(t, false)
	addGraphCommit(r, 1)
	settleGraph(t, g, h, tt, func() bool { return tt.HasText(graphNewCommit) })
	if g.graph.outdated {
		t.Fatal("graph at the top marked outdated")
	}
	if first, _ := g.graph.list.Visible(); first != 0 {
		t.Fatalf("first visible row = %d, want 0", first)
	}
}

func TestGraphScrolledKeepsPlaceUntilRefresh(t *testing.T) {
	t.Parallel()
	g, h, r, tt := openScrolledGraph(t, true)
	before, _ := g.graph.list.Visible()
	if before == 0 {
		t.Fatal("graph did not scroll")
	}
	subject := g.graph.commits[before].Subject
	addGraphCommit(r, 1)
	settleGraph(t, g, h, tt, func() bool { return g.graph.outdated })
	if after, _ := g.graph.list.Visible(); after != before || g.graph.commits[after].Subject != subject {
		t.Fatalf("first visible row moved from %d to %d", before, after)
	}
	if tt.HasText(graphNewCommit) {
		t.Fatal("new commit loaded while scrolled")
	}
	if err := tt.Click(graphOutdatedLabel); err != nil {
		t.Fatal(err)
	}
	h.drain(t, func() bool { tt.Frame(); return tt.HasText(graphNewCommit) })
	if g.graph.outdated {
		t.Fatal("graph still outdated after refresh")
	}
	if first, _ := g.graph.list.Visible(); first != 0 {
		t.Fatalf("first visible row after refresh = %d, want 0", first)
	}
}
