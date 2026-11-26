package git

import (
	"context"
	"strings"
	"testing"
)

func TestStash(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	commitFile(t, repo, "a.txt", "a\n", "init")
	commitFile(t, repo, "b.txt", "b\n", "second")
	if made, err := repo.StashPush(ctx, "", false, false); err != nil || made {
		t.Fatalf("nothing to stash: %v %v", made, err)
	}
	put(t, repo, "a.txt", "a changed\n")
	put(t, repo, "b.txt", "b changed\n")
	put(t, repo, "u.txt", "untracked\n")
	if made, err := repo.StashPush(ctx, "first stash", true, false); err != nil || !made {
		t.Fatalf("stash push: %v %v", made, err)
	}
	if exists(repo, "u.txt") || slurp(t, repo, "a.txt") != "a\n" {
		t.Fatal("stash must clean the tree")
	}
	put(t, repo, "a.txt", "again\n")
	put(t, repo, "star*.txt", "x\n")
	g(t, repo, "add", "star*.txt")
	if made, err := repo.StashPush(ctx, "path stash", false, false, "star*.txt"); err != nil || !made {
		t.Fatalf("path stash: %v %v", made, err)
	}
	if slurp(t, repo, "a.txt") != "again\n" || exists(repo, "star*.txt") {
		t.Fatal("path stash touched the wrong files")
	}
	stashes, err := repo.Stashes(ctx)
	if err != nil || len(stashes) != 2 {
		t.Fatalf("stashes: %+v %v", stashes, err)
	}
	if stashes[0].Index != 0 || stashes[0].Ref != "stash@{0}" || !strings.Contains(stashes[0].Message, "path stash") || stashes[0].OID == "" || stashes[0].Time.IsZero() {
		t.Fatalf("stash 0: %+v", stashes[0])
	}
	if !strings.Contains(stashes[1].Message, "first stash") {
		t.Fatalf("stash 1: %+v", stashes[1])
	}
	diffs, err := repo.StashShow(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, d := range diffs {
		names = append(names, d.Path())
	}
	if !equalStrings(names, []string{"a.txt", "b.txt", "u.txt"}) {
		t.Fatalf("stash diff files: %v", names)
	}
	g(t, repo, "checkout", "--", "a.txt")
	res, err := repo.StashApply(ctx, 1, true)
	if err != nil || res.Stopped {
		t.Fatalf("pop: %+v %v", res, err)
	}
	if slurp(t, repo, "u.txt") != "untracked\n" || slurp(t, repo, "b.txt") != "b changed\n" {
		t.Fatal("pop did not restore stashed files")
	}
	if left, _ := repo.Stashes(ctx); len(left) != 1 {
		t.Fatalf("pop must drop the stash: %+v", left)
	}
	if err := repo.StashDrop(ctx, 0); err != nil {
		t.Fatal(err)
	}
	if left, _ := repo.Stashes(ctx); len(left) != 0 {
		t.Fatalf("stashes left: %+v", left)
	}
	if err := repo.StashDrop(ctx, -1); err == nil {
		t.Fatal("negative index must fail")
	}
	if err := repo.StashDrop(ctx, 5); err == nil {
		t.Fatal("missing stash must fail")
	}
}

func TestStashApplyConflict(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	commitFile(t, repo, "a.txt", "a\n", "init")
	put(t, repo, "a.txt", "stashed\n")
	if _, err := repo.StashPush(ctx, "", false, false); err != nil {
		t.Fatal(err)
	}
	commitFile(t, repo, "a.txt", "committed\n", "diverge")
	res, err := repo.StashApply(ctx, 0, false)
	if err != nil || !res.Stopped {
		t.Fatalf("conflicting apply must stop: %+v %v", res, err)
	}
	conflicts, _ := repo.Conflicts(ctx)
	if len(conflicts) != 1 || conflicts[0].Path != "a.txt" {
		t.Fatalf("conflicts: %+v", conflicts)
	}
}
