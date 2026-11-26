package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func conflictRepo(t *testing.T) (*Repo, string) {
	t.Helper()
	repo := newRepo(t)
	commitFile(t, repo, "c.txt", "base 1\nbase 2\nbase 3\n", "base")
	g(t, repo, "switch", "--quiet", "--create", "topic")
	topic := commitFile(t, repo, "c.txt", "base 1\ntheirs 2\nbase 3\n", "topic change")
	g(t, repo, "switch", "--quiet", "main")
	commitFile(t, repo, "c.txt", "base 1\nours 2\nbase 3\n", "main change")
	return repo, topic
}

func TestConflictingMerge(t *testing.T) {
	repo, topic := conflictRepo(t)
	ctx := context.Background()
	if st, err := repo.State(ctx); err != nil || st.Kind != StateNone {
		t.Fatalf("clean state: %+v %v", st, err)
	}
	res, err := repo.Merge(ctx, "topic", MergeOptions{})
	if err != nil || !res.Stopped {
		t.Fatalf("merge: %+v %v", res, err)
	}
	st, err := repo.State(ctx)
	if err != nil || st.Kind != StateMerging || st.Head != topic {
		t.Fatalf("state: %+v %v", st, err)
	}
	status, _ := repo.Status(ctx)
	if got := status.Conflicted(); len(got) != 1 || got[0].Path != "c.txt" || got[0].Index != 'U' || got[0].WorkTree != 'U' {
		t.Fatalf("conflicted: %+v", got)
	}
	conflicts, err := repo.Conflicts(ctx)
	if err != nil || !reflect.DeepEqual(conflicts, []ConflictFile{{Path: "c.txt", Base: true, Ours: true, Theirs: true}}) {
		t.Fatalf("conflicts: %+v %v", conflicts, err)
	}
	base, ours, theirs, err := repo.MergeVersions(ctx, "c.txt")
	if err != nil || string(base) != "base 1\nbase 2\nbase 3\n" || string(ours) != "base 1\nours 2\nbase 3\n" || string(theirs) != "base 1\ntheirs 2\nbase 3\n" {
		t.Fatalf("versions: %q %q %q %v", base, ours, theirs, err)
	}
	chunks := Merge3(SplitLines(string(base)), SplitLines(string(ours)), SplitLines(string(theirs)))
	if render(chunks) != "S(base 1) C(ours 2|theirs 2) S(base 3)" {
		t.Fatalf("chunks: %s", render(chunks))
	}
	if _, err := repo.Continue(ctx); err != nil {
		t.Logf("continue with unresolved paths: %v", err)
	}
	if st, _ = repo.State(ctx); st.Kind != StateMerging {
		t.Fatal("continue must not finish an unresolved merge")
	}
	if err := repo.AcceptSide(ctx, "c.txt", false); err != nil {
		t.Fatal(err)
	}
	if slurp(t, repo, "c.txt") != "base 1\ntheirs 2\nbase 3\n" {
		t.Fatalf("accept theirs: %q", slurp(t, repo, "c.txt"))
	}
	if conflicts, _ = repo.Conflicts(ctx); len(conflicts) != 0 {
		t.Fatalf("still conflicted: %+v", conflicts)
	}
	res, err = repo.Continue(ctx)
	if err != nil || res.Stopped {
		t.Fatalf("continue: %+v %v", res, err)
	}
	if st, _ = repo.State(ctx); st.Kind != StateNone {
		t.Fatalf("state after continue: %+v", st)
	}
	if got := g(t, repo, "rev-list", "--parents", "-n", "1", "HEAD"); len(splitFields(got)) != 3 {
		t.Fatalf("expected merge commit: %s", got)
	}
}

func splitFields(s string) []string {
	var out []string
	start := -1
	for i, r := range s + " " {
		if r == ' ' || r == '\n' {
			if start >= 0 {
				out = append(out, s[start:i])
				start = -1
			}
		} else if start < 0 {
			start = i
		}
	}
	return out
}

func TestAcceptOursMarkResolvedAndAbort(t *testing.T) {
	repo, _ := conflictRepo(t)
	ctx := context.Background()
	if _, err := repo.Merge(ctx, "topic", MergeOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := repo.AcceptSide(ctx, "c.txt", true); err != nil {
		t.Fatal(err)
	}
	if slurp(t, repo, "c.txt") != "base 1\nours 2\nbase 3\n" {
		t.Fatalf("accept ours: %q", slurp(t, repo, "c.txt"))
	}
	if err := repo.AcceptSide(ctx, "c.txt", true); err == nil {
		t.Fatal("resolved path has no conflict")
	}
	if err := repo.Abort(ctx); err != nil {
		t.Fatal(err)
	}
	if st, _ := repo.State(ctx); st.Kind != StateNone {
		t.Fatalf("state after abort: %+v", st)
	}
	if err := repo.Abort(ctx); !errors.Is(err, ErrNoOperation) {
		t.Fatalf("abort without operation: %v", err)
	}
	if _, err := repo.Continue(ctx); !errors.Is(err, ErrNoOperation) {
		t.Fatalf("continue without operation: %v", err)
	}
	if _, err := repo.Merge(ctx, "topic", MergeOptions{}); err != nil {
		t.Fatal(err)
	}
	put(t, repo, "c.txt", "hand merged\n")
	if err := repo.MarkResolved(ctx, "c.txt"); err != nil {
		t.Fatal(err)
	}
	if res, err := repo.Continue(ctx); err != nil || res.Stopped {
		t.Fatalf("continue: %+v %v", res, err)
	}
	if got := g(t, repo, "show", "HEAD:c.txt"); got != "hand merged" {
		t.Fatalf("committed: %q", got)
	}
	if _, err := repo.Skip(ctx); err == nil {
		t.Fatal("skip without operation must fail")
	}
}

func TestModifyDeleteConflict(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	commitFile(t, repo, "d.txt", "1\n2\n", "base")
	g(t, repo, "switch", "--quiet", "--create", "topic")
	put(t, repo, "d.txt", "1\nchanged\n")
	commitAll(t, repo, "modify")
	g(t, repo, "switch", "--quiet", "main")
	g(t, repo, "rm", "--quiet", "d.txt")
	g(t, repo, "commit", "--quiet", "-m", "delete")
	if res, err := repo.Merge(ctx, "topic", MergeOptions{}); err != nil || !res.Stopped {
		t.Fatalf("merge: %+v %v", res, err)
	}
	conflicts, _ := repo.Conflicts(ctx)
	if !reflect.DeepEqual(conflicts, []ConflictFile{{Path: "d.txt", Base: true, Theirs: true}}) {
		t.Fatalf("conflicts: %+v", conflicts)
	}
	base, ours, theirs, err := repo.MergeVersions(ctx, "d.txt")
	if err != nil || base == nil || ours != nil || theirs == nil {
		t.Fatalf("versions: %q %q %q %v", base, ours, theirs, err)
	}
	if err := repo.AcceptSide(ctx, "d.txt", true); err != nil {
		t.Fatal(err)
	}
	if exists(repo, "d.txt") {
		t.Fatal("accepting the deleted side must remove the file")
	}
	if conflicts, _ = repo.Conflicts(ctx); len(conflicts) != 0 {
		t.Fatalf("still conflicted: %+v", conflicts)
	}
}

func TestCherryPickAndRevertStates(t *testing.T) {
	repo, topic := conflictRepo(t)
	ctx := context.Background()
	res, err := repo.CherryPick(ctx, topic)
	if err != nil || !res.Stopped {
		t.Fatalf("cherry-pick: %+v %v", res, err)
	}
	if st, _ := repo.State(ctx); st.Kind != StateCherryPicking || st.Head != topic {
		t.Fatalf("state: %+v", st)
	}
	if res, err = repo.Skip(ctx); err != nil || res.Stopped {
		t.Fatalf("skip: %+v %v", res, err)
	}
	if st, _ := repo.State(ctx); st.Kind != StateNone {
		t.Fatalf("state after skip: %+v", st)
	}
	first := g(t, repo, "rev-list", "--max-parents=0", "HEAD")
	res, err = repo.Revert(ctx, first)
	if err != nil || !res.Stopped {
		t.Fatalf("revert: %+v %v", res, err)
	}
	if st, _ := repo.State(ctx); st.Kind != StateReverting {
		t.Fatalf("state: %+v", st)
	}
	if err := repo.Abort(ctx); err != nil {
		t.Fatal(err)
	}
	if st, _ := repo.State(ctx); st.Kind != StateNone {
		t.Fatalf("state after abort: %+v", st)
	}
}

func TestRebaseConflictState(t *testing.T) {
	repo, _ := conflictRepo(t)
	ctx := context.Background()
	g(t, repo, "switch", "--quiet", "topic")
	commitFile(t, repo, "extra.txt", "x\n", "topic extra")
	res, err := repo.Rebase(ctx, "main")
	if err != nil || !res.Stopped {
		t.Fatalf("rebase: %+v %v", res, err)
	}
	st, _ := repo.State(ctx)
	if st.Kind != StateRebasing && st.Kind != StateRebaseInteractive {
		t.Fatalf("state: %+v", st)
	}
	if st.Head != "topic" || st.Onto == "" || st.Total != 2 || st.Step != 1 {
		t.Fatalf("rebase progress: %+v", st)
	}
	if err := repo.AcceptSide(ctx, "c.txt", true); err != nil {
		t.Fatal(err)
	}
	res, err = repo.Continue(ctx)
	if err != nil {
		t.Fatalf("continue: %+v %v", res, err)
	}
	if st, _ = repo.State(ctx); st.Kind != StateNone {
		t.Fatalf("state after rebase: %+v", st)
	}
}

func TestBisectState(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	commitFile(t, repo, "a.txt", "1\n", "one")
	commitFile(t, repo, "a.txt", "2\n", "two")
	g(t, repo, "bisect", "start", "HEAD", "HEAD~1")
	if st, _ := repo.State(ctx); st.Kind != StateBisecting {
		t.Fatalf("state: %+v", st)
	}
	if err := repo.Abort(ctx); err != nil {
		t.Fatal(err)
	}
	if st, _ := repo.State(ctx); st.Kind != StateNone {
		t.Fatalf("state after reset: %+v", st)
	}
}

func TestStateFromFiles(t *testing.T) {
	dir := t.TempDir()
	repo := &Repo{Root: dir, GitDir: dir}
	ctx := context.Background()
	write := func(rel, content string) {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("rebase-merge/interactive", "")
	write("rebase-merge/head-name", "refs/heads/feature\n")
	write("rebase-merge/onto", "abc123\n")
	write("rebase-merge/done", "pick 1 a\npick 2 b\n")
	write("rebase-merge/git-rebase-todo", "pick 3 c\n# comment\n\n")
	st, err := repo.State(ctx)
	want := RepoState{Kind: StateRebaseInteractive, Head: "feature", Onto: "abc123", Step: 2, Total: 3}
	if err != nil || st != want {
		t.Fatalf("got %+v %v want %+v", st, err, want)
	}
	write("rebase-merge/msgnum", "2\n")
	write("rebase-merge/end", "5\n")
	if st, _ = repo.State(ctx); st.Step != 2 || st.Total != 5 {
		t.Fatalf("msgnum/end: %+v", st)
	}
	write("rebase-merge/end", "oops\n")
	if _, err = repo.State(ctx); err == nil {
		t.Fatal("malformed counter must fail")
	}
	if err := os.RemoveAll(filepath.Join(dir, "rebase-merge")); err != nil {
		t.Fatal(err)
	}
	write("rebase-apply/rebasing", "")
	write("rebase-apply/next", "1\n")
	write("rebase-apply/last", "4\n")
	write("rebase-apply/head-name", "detached HEAD\n")
	if st, _ = repo.State(ctx); st.Kind != StateRebasing || st.Head != "" || st.Step != 1 || st.Total != 4 {
		t.Fatalf("rebase-apply: %+v", st)
	}
	if err := os.RemoveAll(filepath.Join(dir, "rebase-apply")); err != nil {
		t.Fatal(err)
	}
	write("sequencer/todo", "# c\nrevert abc subject\n")
	if st, _ = repo.State(ctx); st.Kind != StateReverting {
		t.Fatalf("sequencer revert: %+v", st)
	}
	write("sequencer/todo", "pick abc subject\n")
	if st, _ = repo.State(ctx); st.Kind != StateCherryPicking {
		t.Fatalf("sequencer pick: %+v", st)
	}
	if StateCherryPicking.String() != "cherry-picking" || StateNone.String() != "none" {
		t.Fatal("state names")
	}
}

func TestParseConflictsSample(t *testing.T) {
	data := "100644 aaaa 1\ta b.txt\x00100644 bbbb 2\ta b.txt\x00100644 cccc 3\ta b.txt\x00100644 dddd 2\tüni.txt\x00"
	got, err := parseConflicts([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	want := []ConflictFile{{Path: "a b.txt", Base: true, Ours: true, Theirs: true}, {Path: "üni.txt", Ours: true}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
	for _, bad := range []string{"100644 aaaa\tx\x00", "100644 aaaa 9\tx\x00", "no tab here\x00", "100644 aaaa x\tp\x00"} {
		if _, err := parseConflicts([]byte(bad)); err == nil {
			t.Errorf("expected error for %q", bad)
		}
	}
}

func TestParseWorktreesAndSubmodulesSamples(t *testing.T) {
	trees, err := parseWorktrees([]byte("worktree /a\x00HEAD aaaa\x00branch refs/heads/main\x00\x00worktree /b c\x00HEAD bbbb\x00detached\x00locked reason\x00\x00worktree /bare\x00bare\x00\x00worktree /p\x00HEAD cccc\x00branch refs/heads/x\x00prunable gone\x00\x00"))
	if err != nil {
		t.Fatal(err)
	}
	want := []Worktree{
		{Path: "/a", HEAD: "aaaa", Branch: "main"},
		{Path: "/b c", HEAD: "bbbb", Detached: true, Locked: true},
		{Path: "/bare", Bare: true},
		{Path: "/p", HEAD: "cccc", Branch: "x", Prunable: true},
	}
	if !reflect.DeepEqual(trees, want) {
		t.Fatalf("worktrees: %+v", trees)
	}
	if _, err := parseWorktrees([]byte("HEAD aaaa\x00")); err == nil {
		t.Error("record before worktree must fail")
	}
	subs, err := parseSubmodules(" aaaa libs/one (heads/main)\n-bbbb libs/two words\n+cccc libs/three (v1.0-2-gabc)\nUdddd libs/four (x)\n")
	if err != nil {
		t.Fatal(err)
	}
	wantSubs := []Submodule{
		{Path: "libs/one", OID: "aaaa", Status: ' '},
		{Path: "libs/two words", OID: "bbbb", Status: '-'},
		{Path: "libs/three", OID: "cccc", Status: '+'},
		{Path: "libs/four", OID: "dddd", Status: 'U'},
	}
	if !reflect.DeepEqual(subs, wantSubs) {
		t.Fatalf("submodules: %+v", subs)
	}
	for _, bad := range []string{"xaaaa path", "-", " aaaa"} {
		if _, err := parseSubmodules(bad); err == nil {
			t.Errorf("expected error for %q", bad)
		}
	}
}
