package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestWorktrees(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	ctx := context.Background()
	commitFile(t, repo, "a.txt", "a\n", "init")
	trees, err := repo.Worktrees(ctx)
	if err != nil || len(trees) != 1 || trees[0].Path != repo.Root || trees[0].Branch != "main" || trees[0].HEAD == "" {
		t.Fatalf("initial: %+v %v", trees, err)
	}
	parent, _ := filepath.EvalSymlinks(t.TempDir())
	feature := filepath.Join(parent, "wt feature")
	if err := repo.AddWorktree(ctx, feature, "feature", true); err != nil {
		t.Fatal(err)
	}
	detached := filepath.Join(parent, "wt-detached")
	g(t, repo, "worktree", "add", "--quiet", "--detach", detached)
	existing := filepath.Join(parent, "wt-existing")
	g(t, repo, "branch", "other")
	if err := repo.AddWorktree(ctx, existing, "other", false); err != nil {
		t.Fatal(err)
	}
	g(t, repo, "worktree", "lock", existing)
	trees, err = repo.Worktrees(ctx)
	if err != nil || len(trees) != 4 {
		t.Fatalf("worktrees: %+v %v", trees, err)
	}
	byPath := map[string]Worktree{}
	for _, tr := range trees {
		byPath[tr.Path] = tr
	}
	if tr := byPath[feature]; tr.Branch != "feature" || tr.Detached || tr.Locked {
		t.Fatalf("feature: %+v", tr)
	}
	if tr := byPath[detached]; !tr.Detached || tr.Branch != "" {
		t.Fatalf("detached: %+v", tr)
	}
	if tr := byPath[existing]; tr.Branch != "other" || !tr.Locked {
		t.Fatalf("existing: %+v", tr)
	}
	if err := repo.AddWorktree(ctx, filepath.Join(parent, "x"), "", true); err == nil {
		t.Fatal("new branch without name must fail")
	}
	if err := repo.RemoveWorktree(ctx, feature, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(feature); !os.IsNotExist(err) {
		t.Fatal("worktree directory must be removed")
	}
	if err := os.RemoveAll(detached); err != nil {
		t.Fatal(err)
	}
	trees, _ = repo.Worktrees(ctx)
	prunable := 0
	for _, tr := range trees {
		if tr.Prunable {
			prunable++
		}
	}
	if len(trees) != 3 || prunable != 1 {
		t.Fatalf("after removal: %+v", trees)
	}
	put(t, &Repo{Root: existing}, "dirty.txt", "d\n")
	g(t, repo, "worktree", "unlock", existing)
	if err := repo.RemoveWorktree(ctx, existing, false); err != nil {
		t.Logf("untracked files do not block removal: %v", err)
	}
}

func TestSubmodules(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	lib := newRepo(t)
	commitFile(t, lib, "lib.txt", "lib\n", "lib init")
	main := newRepo(t)
	commitFile(t, main, "a.txt", "a\n", "init")
	g(t, main, "submodule", "--quiet", "add", lib.Root, "libs/lib")
	commitAll(t, main, "add submodule")
	subs, err := main.Submodules(ctx)
	if err != nil || len(subs) != 1 || subs[0].Path != "libs/lib" || subs[0].Status != ' ' || subs[0].OID != g(t, lib, "rev-parse", "HEAD") {
		t.Fatalf("submodules: %+v %v", subs, err)
	}
	commitFile(t, &Repo{Root: filepath.Join(main.Root, "libs/lib")}, "more.txt", "m\n", "sub change")
	subs, _ = main.Submodules(ctx)
	if len(subs) != 1 || subs[0].Status != '+' {
		t.Fatalf("modified: %+v", subs)
	}
	dir := t.TempDir()
	rawGit(t, dir, "", "clone", "--quiet", main.Root, "clone")
	clone, err := Open(ctx, filepath.Join(dir, "clone"))
	if err != nil {
		t.Fatal(err)
	}
	subs, err = clone.Submodules(ctx)
	if err != nil || len(subs) != 1 || subs[0].Status != '-' {
		t.Fatalf("uninitialized: %+v %v", subs, err)
	}
	if err := clone.UpdateSubmodules(ctx, true, true); err != nil {
		t.Fatal(err)
	}
	subs, _ = clone.Submodules(ctx)
	if len(subs) != 1 || subs[0].Status != ' ' {
		t.Fatalf("initialized: %+v", subs)
	}
}
