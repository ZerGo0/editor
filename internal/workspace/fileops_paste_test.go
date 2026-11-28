package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ZerGo0/editor/internal/testutil"
)

func TestCopyEntryCopiesATreeWithLinks(t *testing.T) {
	t.Parallel()
	src := filepath.Join(t.TempDir(), "pkg")
	testutil.WriteFile(t, filepath.Join(src, "a.txt"), "a")
	testutil.WriteFile(t, filepath.Join(src, "sub", "b.txt"), "b")
	if err := os.Symlink("a.txt", filepath.Join(src, "link")); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	to, err := copyEntry(context.Background(), src, dir)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(to, "sub", "b.txt")); err != nil || string(got) != "b" {
		t.Fatalf("sub/b.txt = %q, %v", got, err)
	}
	if link, err := os.Readlink(filepath.Join(to, "link")); err != nil || link != "a.txt" {
		t.Fatalf("link = %q, %v", link, err)
	}
}

func TestCopyEntryCancelLeavesNoHalfCopy(t *testing.T) {
	t.Parallel()
	src := filepath.Join(t.TempDir(), "pkg")
	testutil.WriteFile(t, filepath.Join(src, "a.txt"), "a")
	file := filepath.Join(t.TempDir(), "one.txt")
	testutil.WriteFile(t, file, "one")
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, p := range []string{src, file} {
		to, err := copyEntry(ctx, p, dir)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("copyEntry(%s) = %v, want context.Canceled", p, err)
		}
		if _, err := os.Lstat(to); !os.IsNotExist(err) {
			t.Fatalf("%s is left after a cancelled copy: %v", to, err)
		}
	}
}

func TestCopyEntryRefusesToCopyAFolderIntoItself(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	testutil.WriteFile(t, filepath.Join(src, "sub", "a.txt"), "a")
	if _, err := copyEntry(context.Background(), src, filepath.Join(src, "sub")); !errors.Is(err, errIntoItself) {
		t.Fatalf("copyEntry into itself = %v", err)
	}
}

func TestPasteTitle(t *testing.T) {
	t.Parallel()
	if got := pasteTitle([]string{"/a/b.txt"}); got != "Copying b.txt..." {
		t.Errorf("one = %q", got)
	}
	if got := pasteTitle([]string{"/a", "/b"}); got != "Copying 2 items..." {
		t.Errorf("two = %q", got)
	}
}
