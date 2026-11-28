package gitui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/testutil"
	"github.com/egoist/mygo"
)

const fetchLine = "Receiving objects:  50% (1/2)"

type slowRepo struct {
	*fakeRepo
	release chan struct{}
}

func (r *slowRepo) Fetch(ctx context.Context, _ string, _ bool, progress func(string)) error {
	progress(fetchLine)
	<-ctx.Done()
	return fmt.Errorf("git fetch: %w", ctx.Err())
}

func (r *slowRepo) Commit(ctx context.Context, o git.CommitOptions) (string, error) {
	<-r.release
	return r.fakeRepo.Commit(ctx, o)
}

func newSlowGit(t *testing.T) (*Git, *fakeHost, *slowRepo) {
	t.Helper()
	st := testStatus()
	st.Files = st.Files[:4]
	r := &slowRepo{fakeRepo: &fakeRepo{dir: tempDir(t), status: st, branches: []git.Branch{{Name: "main", Current: true}}}, release: make(chan struct{})}
	h := newFakeHost(r.dir)
	g := newGit(h, deps{
		available: func() bool { return true },
		open:      func(context.Context, string, string) (repo, error) { return r, nil },
		watch:     func(context.Context, string, func()) {},
		ask:       func(*mygo.Window, question) bool { return true },
	})
	t.Cleanup(g.Close)
	h.drain(t, func() bool { return g.ready() && g.snap.status != nil })
	return g, h, r
}

func TestFetchRunsInBackgroundAndCancelsQuietly(t *testing.T) {
	g, h, _ := newSlowGit(t)
	g.Run("git.fetch")
	h.drain(t, func() bool { return len(g.tasks) == 1 && g.tasks[0].phase != "" })
	tk := g.tasks[0]
	if got := tk.label(); got != "Fetch: Receiving objects 50%" {
		t.Errorf("label = %q", got)
	}
	if got := tk.statusText(); got != "Fetch 45%" {
		t.Errorf("status text = %q", got)
	}
	g.commit.message = "Fix"
	if reason := g.commitBlocker(g.snap.status, commitSmart); reason != "" {
		t.Errorf("commit is blocked during a fetch: %q", reason)
	}
	g.Run("git.pull")
	if !slices.Contains(h.notes, "Fetch is still running.") {
		t.Errorf("pull during a fetch: notes %v", h.notes)
	}
	tt := newTester(g, g.SideView)
	if err := tt.Click("Cancel Fetch"); err != nil {
		t.Fatal(err)
	}
	h.drain(t, func() bool { return len(g.tasks) == 0 })
	h.settle()
	if len(h.errs) > 0 {
		t.Errorf("a cancelled fetch shows errors: %v", h.errs)
	}
}

func TestCancelCommandStopsTheRunningFetch(t *testing.T) {
	g, h, _ := newSlowGit(t)
	g.Run(cancelCommand)
	if !slices.Contains(h.notes, "No Git operation that can be cancelled is running.") {
		t.Errorf("notes %v", h.notes)
	}
	g.Run("git.fetch")
	h.drain(t, func() bool { return len(g.tasks) == 1 })
	g.Run(cancelCommand)
	h.drain(t, func() bool { return len(g.tasks) == 0 })
}

func TestCommitShowsBusyWhileHooksRun(t *testing.T) {
	g, h, r := newSlowGit(t)
	g.commit.message = "Fix"
	g.doCommit(commitSmart, afterNone)
	if g.busy != "Commit" {
		t.Fatalf("busy = %q, want Commit", g.busy)
	}
	tt := newTester(g, g.SideView)
	if _, ok := tt.Access("Commit..."); !ok {
		t.Error("no progress bar while the commit runs")
	}
	if reason := g.commitBlocker(g.snap.status, commitSmart); reason != "Commit is running." {
		t.Errorf("commit blocker = %q", reason)
	}
	close(r.release)
	h.drain(t, func() bool { return g.busy == "" && g.commit.message == "" })
}

func TestFailedOpShowsDescribedError(t *testing.T) {
	g, h, _ := newSlowGit(t)
	g.do(op{title: "Commit", banner: true, run: func(context.Context, repo, func(string)) (git.OpResult, error) {
		return git.OpResult{}, &git.Error{Args: []string{"commit"}, ExitCode: 1, Stderr: "lint: checking\nlint: 3 problems found"}
	}})
	h.drain(t, func() bool { return len(h.errs) > 0 })
	if got := h.errs[0]; got != "Commit failed: lint: checking\nlint: 3 problems found" {
		t.Errorf("error = %q", got)
	}
}

type cloneHost struct {
	*fakeHost
	parent string
	acted  []string
	opened []string
}

func (h *cloneHost) PickFolder(string) (string, error) { return h.parent, nil }
func (h *cloneHost) OpenFolder(root string)            { h.opened = append(h.opened, root) }
func (h *cloneHost) NotifyAction(title, _, _ string, onAction func()) {
	h.acted = append(h.acted, title)
	onAction()
}

func TestCloneRepositoryOpensTheClone(t *testing.T) {
	src := newTempRepo(t)
	testutil.WriteFile(t, filepath.Join(src, "a.txt"), "a")
	commitAll(t, src, "first")
	h := &cloneHost{fakeHost: newFakeHost(tempDir(t)), parent: tempDir(t)}
	h.answer = "file://" + src
	g := newGit(h, deps{available: func() bool { return false }})
	t.Cleanup(g.Close)
	if !g.Run(cloneCommand) {
		t.Fatal("clone not handled without a repository")
	}
	h.drain(t, func() bool { return len(h.acted) > 0 || len(h.errs) > 0 })
	dest := filepath.Join(h.parent, filepath.Base(src))
	if len(h.opened) != 1 || h.opened[0] != dest {
		t.Fatalf("opened %v, errors %v", h.opened, h.errs)
	}
	if _, err := os.Stat(filepath.Join(dest, "a.txt")); err != nil {
		t.Fatal(err)
	}
	g.startClone(h.answer, h.parent)
	if len(h.errs) != 1 || !strings.Contains(h.errs[0], "already has a file or folder named") {
		t.Fatalf("second clone into the same folder: %v", h.errs)
	}
}

func TestNoGitChecksAgain(t *testing.T) {
	r := &fakeRepo{dir: tempDir(t), status: testStatus()}
	h := newFakeHost(r.dir)
	installed := false
	g := newGit(h, deps{
		available: func() bool { return installed },
		open:      func(context.Context, string, string) (repo, error) { return r, nil },
		watch:     func(context.Context, string, func()) {},
	})
	t.Cleanup(g.Close)
	h.drain(t, func() bool { return g.phase == phaseNoGit })
	installed = true
	tt := newTester(g, g.SideView)
	if err := tt.Click("Check Again"); err != nil {
		t.Fatal(err)
	}
	h.drain(t, func() bool { return g.ready() })
}
