package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const (
	cancelWithin = 2 * time.Second
	pollInterval = 10 * time.Millisecond
)

func TestCloneDirNameAndURLCheck(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"https://github.com/acme/widgets.git": "widgets",
		"git@github.com:acme/tools.git":       "tools",
		"/srv/repos/app/":                     "app",
		"https://host/":                       "host",
		".git":                                cloneDefaultName,
	} {
		if got := CloneDirName(in); got != want {
			t.Errorf("CloneDirName(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]error{"": ErrNoCloneURL, "-u x": ErrBadCloneURL, "a b": ErrBadCloneURL, "https://x/y": nil} {
		if err := CheckCloneURL(in); !errors.Is(err, want) {
			t.Errorf("CheckCloneURL(%q) = %v, want %v", in, err, want)
		}
	}
}

func TestCloneCopiesARepositoryWithProgress(t *testing.T) {
	t.Parallel()
	src := newRepo(t)
	commitFile(t, src, "a.txt", "a\n", "first")
	dest := filepath.Join(t.TempDir(), "copy")
	var lines []string
	if err := Clone(context.Background(), "file://"+src.Root, dest, func(s string) { lines = append(lines, s) }); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "a.txt")); err != nil {
		t.Fatal(err)
	}
	if len(lines) == 0 {
		t.Error("clone reported no progress lines")
	}
}

func TestCancelStopsAHangingFetch(t *testing.T) {
	t.Parallel()
	remote := newBareRemote(t)
	repo := newRepo(t)
	rawGit(t, repo.Root, "", "remote", "add", "origin", remote)
	started := filepath.Join(t.TempDir(), "started")
	rawGit(t, repo.Root, "", "config", "remote.origin.uploadpack", "touch '"+started+"'; while true; do sleep 1; done; git-upload-pack")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- repo.Fetch(ctx, "origin", false, func(string) {}) }()
	deadline := time.Now().Add(cancelWithin)
	for _, err := os.Stat(started); err != nil; _, err = os.Stat(started) {
		if time.Now().After(deadline) {
			t.Fatal("the remote did not start")
		}
		time.Sleep(pollInterval)
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Fetch = %v, want context.Canceled", err)
		}
	case <-time.After(cancelWithin):
		t.Fatal("Fetch did not stop after cancel")
	}
}
