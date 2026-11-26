package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommit(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	if _, err := repo.LastCommitMessage(ctx); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unborn: %v", err)
	}
	put(t, repo, "a.txt", "a\n")
	g(t, repo, "add", "a.txt")
	oid, err := repo.Commit(ctx, CommitOptions{Message: "first line\n\nbody text\n"})
	if err != nil {
		t.Fatal(err)
	}
	if oid != g(t, repo, "rev-parse", "HEAD") {
		t.Fatal("returned oid is not HEAD")
	}
	msg, err := repo.LastCommitMessage(ctx)
	if err != nil || msg != "first line\n\nbody text" {
		t.Fatalf("message %q, %v", msg, err)
	}

	put(t, repo, "a.txt", "a2\n")
	g(t, repo, "add", "a.txt")
	amended, err := repo.Commit(ctx, CommitOptions{Amend: true})
	if err != nil || amended == oid {
		t.Fatalf("amend without message: %s %v", amended, err)
	}
	if msg, _ := repo.LastCommitMessage(ctx); msg != "first line\n\nbody text" {
		t.Fatalf("message after --no-edit amend: %q", msg)
	}
	if _, err := repo.Commit(ctx, CommitOptions{Message: "renamed", Amend: true, SignOff: true, Author: "Bob <bob@example.com>"}); err != nil {
		t.Fatal(err)
	}
	if msg, _ := repo.LastCommitMessage(ctx); !strings.HasPrefix(msg, "renamed") || !strings.Contains(msg, "Signed-off-by: Ada Lovelace <ada@example.com>") {
		t.Fatalf("message after amend: %q", msg)
	}
	if got := g(t, repo, "log", "-1", "--format=%an <%ae>"); got != "Bob <bob@example.com>" {
		t.Fatalf("author: %q", got)
	}
	if got := g(t, repo, "rev-list", "--count", "HEAD"); got != "1" {
		t.Fatalf("amend must not add commits: %s", got)
	}
}

func TestCommitErrors(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	if _, err := repo.Commit(ctx, CommitOptions{Message: "  \n"}); !errors.Is(err, ErrEmptyMessage) {
		t.Fatalf("empty message: %v", err)
	}
	commitFile(t, repo, "a.txt", "a\n", "init")
	_, err := repo.Commit(ctx, CommitOptions{Message: "nothing"})
	var ge *Error
	if !errors.As(err, &ge) || !strings.Contains(ge.Stderr, "nothing to commit") {
		t.Fatalf("nothing to commit: %v", err)
	}
	if _, err := repo.Commit(ctx, CommitOptions{Message: "empty", AllowEmpty: true}); err != nil {
		t.Fatalf("allow empty: %v", err)
	}
}

func TestCommitOnlyPathsAndNoVerify(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	commitFile(t, repo, "a.txt", "a\n", "init")
	put(t, repo, "a.txt", "a changed\n")
	put(t, repo, "b.txt", "b\n")
	g(t, repo, "add", "b.txt")
	if _, err := repo.Commit(ctx, CommitOptions{Message: "only a", Paths: []string{"a.txt"}}); err != nil {
		t.Fatal(err)
	}
	if got := g(t, repo, "show", "--name-only", "--format=", "HEAD"); got != "a.txt" {
		t.Fatalf("committed: %q", got)
	}
	st, _ := repo.Status(ctx)
	if got := paths(st.Staged()); !equalStrings(got, []string{"b.txt"}) {
		t.Fatalf("still staged: %v", got)
	}
	hook := filepath.Join(repo.GitDir, "hooks", "pre-commit")
	if err := os.MkdirAll(filepath.Dir(hook), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hook, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Commit(ctx, CommitOptions{Message: "blocked"}); err == nil {
		t.Fatal("pre-commit hook should block the commit")
	}
	if _, err := repo.Commit(ctx, CommitOptions{Message: "skipped", NoVerify: true}); err != nil {
		t.Fatalf("no-verify: %v", err)
	}
}
