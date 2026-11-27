package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureCommitGraphWritesOnce(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	if err := repo.EnsureCommitGraph(ctx); err != nil {
		t.Fatalf("unborn repo: %v", err)
	}
	commitFile(t, repo, "a.txt", "a\n", "first")
	if err := repo.EnsureCommitGraph(ctx); err != nil {
		t.Fatal(err)
	}
	graph := filepath.Join(repo.GitDir, commitGraphPaths[0])
	before, err := os.Stat(graph)
	if err != nil {
		t.Fatalf("commit-graph not written: %v", err)
	}
	commitFile(t, repo, "b.txt", "b\n", "second")
	if err := repo.EnsureCommitGraph(ctx); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(graph)
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModTime().Equal(before.ModTime()) || after.Size() != before.Size() {
		t.Fatal("an existing commit-graph was rewritten")
	}
}
