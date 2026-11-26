package git

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func shelfRepo(t *testing.T) (*Repo, Shelf) {
	t.Helper()
	repo := newRepo(t)
	commitFile(t, repo, "a.txt", numbered(20), "init")
	commitFile(t, repo, "b.txt", "b\n", "b")
	return repo, Shelf{Dir: filepath.Join(t.TempDir(), "shelf")}
}

func TestShelfRoundTrip(t *testing.T) {
	t.Parallel()
	repo, shelf := shelfRepo(t)
	ctx := context.Background()
	modified := strings.Replace(numbered(20), "line 5\n", "five\n", 1)
	put(t, repo, "a.txt", modified)
	put(t, repo, "b.txt", "b changed\n")
	put(t, repo, "new file.txt", "brand new\n")
	g(t, repo, "add", "new file.txt")
	binary := []byte{0, 1, 2, 255, 254, 0, 10, 13}
	if err := os.WriteFile(filepath.Join(repo.Root, "bin.dat"), binary, 0o644); err != nil {
		t.Fatal(err)
	}
	g(t, repo, "add", "bin.dat")

	entry, err := shelf.Shelve(ctx, repo, "My work: a/b", []string{"a.txt", "new file.txt", "bin.dat"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := validShelfID(entry.ID); err != nil || entry.Name != "My work: a/b" || entry.Base != g(t, repo, "rev-parse", "HEAD") {
		t.Fatalf("entry: %+v %v", entry, err)
	}
	if slurp(t, repo, "a.txt") != numbered(20) || exists(repo, "new file.txt") || exists(repo, "bin.dat") {
		t.Fatal("shelved paths must be reverted")
	}
	if slurp(t, repo, "b.txt") != "b changed\n" {
		t.Fatal("unlisted paths must stay")
	}
	st, _ := repo.Status(ctx)
	if len(st.Staged()) != 0 || len(st.Unstaged()) != 1 {
		t.Fatalf("status after shelve: %+v", st.Files)
	}
	list, err := shelf.List()
	if err != nil || len(list) != 1 || list[0].ID != entry.ID || list[0].Name != entry.Name || list[0].Created.IsZero() {
		t.Fatalf("list: %+v %v", list, err)
	}
	if _, err := os.Stat(filepath.Join(shelf.Dir, entry.ID, "meta.json")); err != nil {
		t.Fatal(err)
	}
	commitFile(t, repo, "c.txt", "moved on\n", "head moved")
	res, err := shelf.Unshelve(ctx, repo, entry.ID, true)
	if err != nil || res.Stopped {
		t.Fatalf("unshelve: %+v %v", res, err)
	}
	if slurp(t, repo, "a.txt") != modified || slurp(t, repo, "new file.txt") != "brand new\n" {
		t.Fatal("unshelve did not restore the changes")
	}
	got, _ := os.ReadFile(filepath.Join(repo.Root, "bin.dat"))
	if !bytes.Equal(got, binary) {
		t.Fatalf("binary round trip: %v", got)
	}
	if list, _ = shelf.List(); len(list) != 0 {
		t.Fatalf("remove=true must delete the entry: %+v", list)
	}
}

func TestShelfKeepAndFallbackToPlainApply(t *testing.T) {
	t.Parallel()
	repo, shelf := shelfRepo(t)
	ctx := context.Background()
	put(t, repo, "a.txt", strings.Replace(numbered(20), "line 5\n", "five\n", 1))
	entry, err := shelf.Shelve(ctx, repo, "kept", nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(slurp(t, repo, "a.txt"), "five") {
		t.Fatal("keep must leave the worktree alone")
	}
	g(t, repo, "checkout", "--", "a.txt")
	put(t, repo, "a.txt", strings.Replace(numbered(20), "line 15\n", "fifteen\n", 1))
	res, err := shelf.Unshelve(ctx, repo, entry.ID, false)
	if err != nil || res.Stopped {
		t.Fatalf("unshelve onto a dirty file: %+v %v", res, err)
	}
	if got := slurp(t, repo, "a.txt"); !strings.Contains(got, "five") || !strings.Contains(got, "fifteen") {
		t.Fatalf("merged content: %q", got)
	}
	if list, _ := shelf.List(); len(list) != 1 {
		t.Fatalf("remove=false must keep the entry: %+v", list)
	}
	g(t, repo, "checkout", "--", "a.txt")
	put(t, repo, "a.txt", strings.Replace(numbered(20), "line 5\n", "conflicting\n", 1))
	g(t, repo, "add", "a.txt")
	g(t, repo, "commit", "--quiet", "-m", "conflicting")
	res, err = shelf.Unshelve(ctx, repo, entry.ID, true)
	if err != nil || !res.Stopped {
		t.Fatalf("conflicting unshelve must stop: %+v %v", res, err)
	}
	if list, _ := shelf.List(); len(list) != 1 {
		t.Fatal("a conflicting unshelve must keep the entry")
	}
}

func TestShelfErrorsAndIDs(t *testing.T) {
	t.Parallel()
	repo, shelf := shelfRepo(t)
	ctx := context.Background()
	if _, err := shelf.Shelve(ctx, repo, "nothing", nil, false); !errors.Is(err, ErrNothingToShelve) {
		t.Fatalf("clean tree: %v", err)
	}
	if _, err := shelf.Shelve(ctx, repo, "  ", nil, false); err == nil {
		t.Fatal("empty name must fail")
	}
	put(t, repo, "a.txt", "x\n")
	first, err := shelf.Shelve(ctx, repo, "same name", nil, true)
	if err != nil {
		t.Fatal(err)
	}
	second, err := shelf.Shelve(ctx, repo, "same name", nil, true)
	if err != nil || first.ID == second.ID {
		t.Fatalf("ids must be unique: %v %v %v", first.ID, second.ID, err)
	}
	if list, _ := shelf.List(); len(list) != 2 {
		t.Fatalf("list: %+v", list)
	}
	for _, id := range []string{"", "..", "../x", "a/b", ".hidden", "a\\b", "x..y", strings.Repeat("a", 200)} {
		if _, err := shelf.Unshelve(ctx, repo, id, false); err == nil {
			t.Errorf("Unshelve(%q) must fail", id)
		}
		if err := shelf.Delete(id); err == nil {
			t.Errorf("Delete(%q) must fail", id)
		}
	}
	if _, err := shelf.Unshelve(ctx, repo, "missing-id", false); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
	if err := shelf.Delete("missing-id"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing delete: %v", err)
	}
	if err := shelf.Delete(first.ID); err != nil {
		t.Fatal(err)
	}
	if list, _ := shelf.List(); len(list) != 1 || list[0].ID != second.ID {
		t.Fatalf("after delete: %+v", list)
	}
	if err := os.WriteFile(filepath.Join(shelf.Dir, second.ID, "meta.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := shelf.List(); err == nil {
		t.Fatal("corrupt metadata must fail")
	}
	if list, err := (Shelf{Dir: filepath.Join(t.TempDir(), "none")}).List(); err != nil || len(list) != 0 {
		t.Fatalf("missing dir: %+v %v", list, err)
	}
	if _, err := (Shelf{}).List(); err == nil {
		t.Fatal("empty dir must fail")
	}
}

func TestShelfDeletedFile(t *testing.T) {
	t.Parallel()
	repo, shelf := shelfRepo(t)
	ctx := context.Background()
	if err := removePath(repo, "b.txt"); err != nil {
		t.Fatal(err)
	}
	entry, err := shelf.Shelve(ctx, repo, "deletion", []string{"b.txt"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if slurp(t, repo, "b.txt") != "b\n" {
		t.Fatal("deleted file must be restored by shelving")
	}
	if _, err := shelf.Unshelve(ctx, repo, entry.ID, true); err != nil {
		t.Fatal(err)
	}
	if exists(repo, "b.txt") {
		t.Fatal("unshelve must delete the file again")
	}
}
