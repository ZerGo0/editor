package gitui

import (
	"slices"
	"testing"
	"time"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/egoist/mygo/ui/editor"
)

func TestHasChangeAt(t *testing.T) {
	st := &git.Status{Branch: "main", OID: "head1"}
	r := &fakeRepo{status: st, shows: map[string]string{"head1:a.go": "one\ntwo\nthree\n"}}
	g, h := newFakeGit(t, r)
	abs := g.paths.abs("a.go")
	h.drain(t, func() bool { return g.Base(abs) != nil })
	buf := editor.NewBuffer("one\nTWO\nthree\n")
	if !g.HasChangeAt(abs, buf, 1) {
		t.Error("changed line has no change")
	}
	if g.HasChangeAt(abs, buf, 0) || g.HasChangeAt(abs, buf, 2) {
		t.Error("unchanged line has a change")
	}
	if g.HasChangeAt(abs, nil, 1) || g.HasChangeAt(g.paths.abs("other.go"), buf, 1) {
		t.Error("change without a base")
	}
}

func TestStatusHookReceivesFiles(t *testing.T) {
	t.Parallel()
	files := []git.FileStatus{{Path: "a.go", Index: '.', WorkTree: 'M'}}
	g, h := newFakeGit(t, &fakeRepo{status: &git.Status{Branch: "main", OID: "head1", Files: files}})
	var got []git.FileStatus
	g.OnStatus(func(f []git.FileStatus) { got = f })
	g.refreshNow()
	h.drain(t, func() bool { return got != nil })
	if !slices.Equal(got, files) {
		t.Fatalf("files = %+v", got)
	}
}

func TestCommitTemplateFillsEmptyMessageOnce(t *testing.T) {
	g, _ := newFakeGit(t, &fakeRepo{status: testStatus()})
	tmpl := "Fixes #12"
	g.SetCommitTemplate(func() string { return tmpl })
	g.applyTemplate()
	if g.commit.message != tmpl {
		t.Fatalf("message = %q", g.commit.message)
	}
	g.commit.message = ""
	g.applyTemplate()
	if g.commit.message != "" {
		t.Fatalf("template applied twice: %q", g.commit.message)
	}
	tmpl = "Fixes #13"
	g.commit.message = "Own text"
	g.applyTemplate()
	if g.commit.message != "Own text" {
		t.Fatalf("template replaced a typed message: %q", g.commit.message)
	}
}

func TestSettingsPullModeAndPageSize(t *testing.T) {
	g, _ := newFakeGit(t, &fakeRepo{status: testStatus()})
	if o := g.pullOptions(); g.logPageSize() != defaultLogPageSize || o.Rebase || o.FFOnly {
		t.Fatalf("defaults: %d %+v", g.logPageSize(), g.pullOptions())
	}
	g.SetSettings(Settings{PullMode: PullRebase, LogPageSize: 50})
	if g.logPageSize() != 50 || !g.pullOptions().Rebase || g.pullOptions().FFOnly {
		t.Fatalf("rebase: %d %+v", g.logPageSize(), g.pullOptions())
	}
	g.SetSettings(Settings{PullMode: PullFFOnly})
	if g.logPageSize() != defaultLogPageSize || !g.pullOptions().FFOnly {
		t.Fatalf("ff-only: %d %+v", g.logPageSize(), g.pullOptions())
	}
}

func TestCommitCount(t *testing.T) {
	for _, tt := range []struct {
		n        int
		complete bool
		want     string
	}{{1, true, "1 commit"}, {2, true, "2 commits"}, {300, false, "300+ commits"}, {0, true, "0 commits"}} {
		if got := commitCount(tt.n, tt.complete); got != tt.want {
			t.Errorf("commitCount(%d, %v) = %q; want %q", tt.n, tt.complete, got, tt.want)
		}
	}
}

func TestAutoFetchTimer(t *testing.T) {
	g, _ := newFakeGit(t, &fakeRepo{status: testStatus()})
	g.SetSettings(Settings{AutoFetch: time.Hour})
	if g.autoFetch == nil {
		t.Fatal("auto fetch not scheduled")
	}
	g.SetSettings(Settings{})
	if g.autoFetch != nil {
		t.Fatal("auto fetch still scheduled")
	}
}
