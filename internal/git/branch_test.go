package git

import (
	"context"
	"strings"
	"testing"
)

func TestParseBranchesSample(t *testing.T) {
	t.Parallel()
	sample := strings.Join([]string{
		"refs/heads/main\x00aaaa\x00*\x00origin/main\x00ahead 2, behind 1\x001700000000\x00Subject one",
		"refs/heads/gone\x00bbbb\x00 \x00origin/gone\x00gone\x001700000001\x00Subject two",
		"refs/heads/feature/x\x00cccc\x00 \x00\x00\x001700000002\x00",
		"refs/remotes/origin/HEAD\x00aaaa\x00 \x00\x00\x001700000000\x00x",
		"refs/remotes/origin/main\x00aaaa\x00 \x00\x00\x001700000000\x00Subject one",
		"",
	}, "\n")
	branches, err := parseBranches(sample)
	if err != nil {
		t.Fatal(err)
	}
	if len(branches) != 4 {
		t.Fatalf("got %d branches: %+v", len(branches), branches)
	}
	if b := branches[0]; b.Name != "main" || !b.Current || b.Upstream != "origin/main" || b.Ahead != 2 || b.Behind != 1 || b.Remote {
		t.Fatalf("main: %+v", b)
	}
	if !branches[1].Gone || branches[2].Name != "feature/x" || branches[2].Subject != "" {
		t.Fatalf("others: %+v", branches[1:3])
	}
	if b := branches[3]; b.Name != "origin/main" || !b.Remote {
		t.Fatalf("remote: %+v", b)
	}
	for _, bad := range []string{"refs/heads/x\x00a", "refs/other/x\x00a\x00 \x00\x00\x001\x00s\x00", "refs/heads/x\x00a\x00 \x00\x00sideways\x001\x00s", "refs/heads/x\x00a\x00 \x00\x00\x00nan\x00s"} {
		if _, err := parseBranches(bad); err == nil {
			t.Errorf("expected error for %q", bad)
		}
	}
}

func TestParseTagsSample(t *testing.T) {
	t.Parallel()
	tags, err := parseTags("refs/tags/v1\x00aaaa\x00\x001700000000\x00light\nrefs/tags/v2\x00tagobj\x00commit2\x001700000001\x00annotated\n")
	if err != nil {
		t.Fatal(err)
	}
	if tags[0].Annotated || tags[0].OID != "aaaa" || !tags[1].Annotated || tags[1].OID != "commit2" || tags[1].Subject != "annotated" {
		t.Fatalf("tags: %+v", tags)
	}
	if _, err := parseTags("bad\x00line"); err == nil {
		t.Error("expected error")
	}
}

func TestBranchLifecycle(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	ctx := context.Background()
	base := commitFile(t, repo, "a.txt", "a\n", "base")
	if err := repo.CreateBranch(ctx, "topic", "", false); err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateBranch(ctx, "from-base", base, true); err != nil {
		t.Fatal(err)
	}
	g(t, repo, "tag", "-a", "-m", "release", "v1")
	g(t, repo, "tag", "light")
	branches, err := repo.Branches(ctx)
	if err != nil || len(branches) != 3 {
		t.Fatalf("branches: %+v %v", branches, err)
	}
	current := 0
	for _, b := range branches {
		if b.Current {
			current++
			if b.Name != "from-base" {
				t.Errorf("current: %s", b.Name)
			}
		}
		if b.OID != base || b.Subject != "base" || b.Time.IsZero() {
			t.Errorf("branch %+v", b)
		}
	}
	if current != 1 {
		t.Fatalf("current count %d", current)
	}
	tags, err := repo.Tags(ctx)
	if err != nil || len(tags) != 2 {
		t.Fatalf("tags: %+v %v", tags, err)
	}
	for _, tag := range tags {
		if tag.OID != base || (tag.Name == "v1") != tag.Annotated {
			t.Errorf("tag %+v", tag)
		}
	}
	if err := repo.RenameBranch(ctx, "topic", "topic2"); err != nil {
		t.Fatal(err)
	}
	if err := repo.Checkout(ctx, "topic2", false); err != nil {
		t.Fatal(err)
	}
	if got := g(t, repo, "branch", "--show-current"); got != "topic2" {
		t.Fatalf("current branch %q", got)
	}
	put(t, repo, "a.txt", "dirty\n")
	if err := repo.Checkout(ctx, "from-base", false); err != nil {
		t.Fatalf("clean-file checkout: %v", err)
	}
	commitFile(t, repo, "c.txt", "c\n", "on from-base")
	put(t, repo, "a.txt", "dirty again\n")
	if err := repo.Checkout(ctx, "topic2", false); err == nil {
		t.Fatal("checkout with conflicting local change must fail without force")
	}
	if err := repo.Checkout(ctx, "topic2", true); err != nil {
		t.Fatalf("forced checkout: %v", err)
	}
	if err := repo.Checkout(ctx, "v1", false); err != nil {
		t.Fatalf("detached checkout of a tag: %v", err)
	}
	if got := g(t, repo, "rev-parse", "--abbrev-ref", "HEAD"); got != "HEAD" {
		t.Fatalf("expected detached HEAD, got %q", got)
	}
	if err := repo.Checkout(ctx, "topic2", false); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteBranch(ctx, "from-base", false); err == nil {
		t.Fatal("unmerged branch must need force")
	}
	if err := repo.DeleteBranch(ctx, "from-base", true); err != nil {
		t.Fatal(err)
	}
	if err := repo.Checkout(ctx, "-b", false); err == nil {
		t.Fatal("option-like ref must be rejected")
	}
}

func TestCompareMergeCherryPickRevertReset(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	ctx := context.Background()
	base := commitFile(t, repo, "a.txt", "a\n", "base")
	g(t, repo, "switch", "--quiet", "--create", "other")
	o1 := commitFile(t, repo, "o1.txt", "1\n", "other 1")
	o2 := commitFile(t, repo, "o2.txt", "2\n", "other 2")
	g(t, repo, "switch", "--quiet", "main")
	m1 := commitFile(t, repo, "m1.txt", "1\n", "main 1")
	cmp, err := repo.CompareBranches(ctx, "main", "other")
	if err != nil || len(cmp.AOnly) != 1 || cmp.AOnly[0].OID != m1 || len(cmp.BOnly) != 2 || cmp.BOnly[0].OID != o2 {
		t.Fatalf("compare: %+v %v", cmp, err)
	}
	res, err := repo.CherryPick(ctx, o1)
	if err != nil || res.Stopped {
		t.Fatalf("cherry-pick: %+v %v", res, err)
	}
	if !exists(repo, "o1.txt") || exists(repo, "o2.txt") {
		t.Fatal("cherry-pick applied the wrong commits")
	}
	res, err = repo.Revert(ctx, "HEAD")
	if err != nil || res.Stopped || exists(repo, "o1.txt") {
		t.Fatalf("revert: %+v %v", res, err)
	}
	if err := repo.Reset(ctx, base, ResetHard); err != nil {
		t.Fatal(err)
	}
	if exists(repo, "m1.txt") || g(t, repo, "rev-parse", "HEAD") != base {
		t.Fatal("hard reset failed")
	}
	if err := repo.Reset(ctx, base, "bogus"); err == nil {
		t.Fatal("unknown reset mode must fail")
	}
	res, err = repo.Merge(ctx, "other", MergeOptions{NoFF: true})
	if err != nil || res.Stopped {
		t.Fatalf("merge: %+v %v", res, err)
	}
	if got := g(t, repo, "rev-list", "--parents", "-n", "1", "HEAD"); len(strings.Fields(got)) != 3 {
		t.Fatalf("expected a merge commit: %s", got)
	}
	if err := repo.Reset(ctx, "HEAD~1", ResetSoft); err != nil {
		t.Fatal(err)
	}
	if !exists(repo, "o2.txt") {
		t.Fatal("soft reset keeps the worktree")
	}
	if err := repo.Reset(ctx, base, ResetHard); err != nil {
		t.Fatal(err)
	}
	res, err = repo.Merge(ctx, "other", MergeOptions{Squash: true})
	if err != nil || res.Stopped {
		t.Fatalf("squash merge: %+v %v", res, err)
	}
	st, _ := repo.Status(ctx)
	if len(st.Staged()) != 2 {
		t.Fatalf("squash must stage the changes: %+v", st.Files)
	}
	if err := repo.Reset(ctx, base, ResetHard); err != nil {
		t.Fatal(err)
	}
	res, err = repo.Merge(ctx, "other", MergeOptions{NoFF: true, NoCommit: true})
	if err != nil || !res.Stopped {
		t.Fatalf("no-commit merge stays in progress: %+v %v", res, err)
	}
	state, _ := repo.State(ctx)
	if state.Kind != StateMerging {
		t.Fatalf("state: %+v", state)
	}
	if _, err := repo.Continue(ctx); err != nil {
		t.Fatal(err)
	}
	if state, _ = repo.State(ctx); state.Kind != StateNone {
		t.Fatalf("state after continue: %+v", state)
	}
	if g(t, repo, "log", "-1", "--format=%s") != "Merge branch 'other'" {
		t.Fatalf("merge message: %s", g(t, repo, "log", "-1", "--format=%s"))
	}
}

func TestMergeFailureWithoutConflict(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	commitFile(t, repo, "a.txt", "a\n", "base")
	res, err := repo.Merge(context.Background(), "no-such-branch", MergeOptions{})
	if err == nil || res.Stopped {
		t.Fatalf("expected a plain error: %+v %v", res, err)
	}
}
