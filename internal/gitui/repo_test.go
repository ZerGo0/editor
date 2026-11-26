package gitui

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/egoist/mygo/ui/editor"
)

func openTemp(t *testing.T, dir string) *cliRepo {
	t.Helper()
	r, err := openCLI(context.Background(), dir, tempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	return r.(*cliRepo)
}

func TestHunkPatchStagesOneHunk(t *testing.T) {
	dir := newTempRepo(t)
	lines := make([]string, 20)
	for i := range lines {
		lines[i] = "line " + strconv.Itoa(i)
	}
	old := strings.Join(lines, "\n") + "\n"
	writeFile(t, dir, "f.txt", old)
	commitAll(t, dir, "init")
	lines[1], lines[17] = "first", "second"
	next := strings.Join(lines, "\n") + "\n"
	writeFile(t, dir, "f.txt", next)
	d := &editor.DiffState{Old: editor.NewBuffer(old), New: editor.NewBuffer(next)}
	hunks := d.Hunks()
	if len(hunks) != 2 {
		t.Fatalf("hunks = %+v", hunks)
	}
	r := openTemp(t, dir)
	ctx := context.Background()
	if err := r.Apply(ctx, hunkPatch("f.txt", old, next, hunks[0], false), true, false); err != nil {
		t.Fatal(err)
	}
	staged, err := r.Show(ctx, indexRev, "f.txt")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(staged), "first") || strings.Contains(string(staged), "second") {
		t.Fatalf("index = %q", staged)
	}
	if err := r.Apply(ctx, hunkPatch("f.txt", old, string(staged), editor.DiffHunk{OldStart: 1, OldEnd: 2, NewStart: 1, NewEnd: 2}, true), true, true); err != nil {
		t.Fatal(err)
	}
	if back, _ := r.Show(ctx, indexRev, "f.txt"); string(back) != old {
		t.Fatalf("unstage left %q", back)
	}
}

func TestCLIExtras(t *testing.T) {
	dir := newTempRepo(t)
	writeFile(t, dir, "a.txt", "a\n")
	commitAll(t, dir, "one")
	writeFile(t, dir, "b.txt", "b\n")
	commitAll(t, dir, "two")
	r := openTemp(t, dir)
	ctx := context.Background()
	if err := r.CreateTag(ctx, "v1", "HEAD", "release"); err != nil {
		t.Fatal(err)
	}
	tags, err := r.Tags(ctx)
	if err != nil || len(tags) != 1 || !tags[0].Annotated {
		t.Fatalf("tags = %+v, %v", tags, err)
	}
	if err := r.DeleteTag(ctx, "v1"); err != nil {
		t.Fatal(err)
	}
	if err := r.CreateTag(ctx, "-bad", "HEAD", ""); err == nil {
		t.Fatal("bad tag name accepted")
	}
	remote := tempDir(t)
	rawGit(t, remote, "init", "--quiet", "--bare")
	if err := r.AddRemote(ctx, "origin", remote); err != nil {
		t.Fatal(err)
	}
	if err := r.RenameRemote(ctx, "origin", "up"); err != nil {
		t.Fatal(err)
	}
	if err := r.SetRemoteURL(ctx, "up", remote); err != nil {
		t.Fatal(err)
	}
	if err := r.PushRef(ctx, "up", "HEAD:refs/heads/main"); err != nil {
		t.Fatal(err)
	}
	if err := r.RemoveRemote(ctx, "up"); err != nil {
		t.Fatal(err)
	}
	head := rawGit(t, dir, "rev-parse", "HEAD")
	names, err := r.BranchesContaining(ctx, head)
	if err != nil || len(names) != 1 || names[0] != "main" {
		t.Fatalf("containing = %v, %v", names, err)
	}
	writeFile(t, dir, "a.txt", "changed\n")
	if err := os.Remove(filepath.Join(dir, "b.txt")); err != nil {
		t.Fatal(err)
	}
	files, err := r.ChangedSince(ctx, "HEAD")
	if err != nil || len(files) != 2 {
		t.Fatalf("changed = %+v, %v", files, err)
	}
	root := rawGit(t, dir, "rev-list", "--max-parents=0", "HEAD")
	fds, err := diffOfCommit(ctx, r, git.Commit{OID: root})
	if err != nil || len(fds) != 1 || !fds[0].IsNew() {
		t.Fatalf("root diff = %+v, %v", fds, err)
	}
	if !strings.Contains(string(joinPatches(fds)), "+a") {
		t.Fatalf("patch = %q", joinPatches(fds))
	}
}

func TestShelfDirIsPerRepo(t *testing.T) {
	dir := newTempRepo(t)
	data := tempDir(t)
	r, err := openCLI(context.Background(), dir, data)
	if err != nil {
		t.Fatal(err)
	}
	shelfDir := r.(*cliRepo).shelf.Dir
	if !strings.HasPrefix(shelfDir, filepath.Join(data, "data", "shelf")) || len(filepath.Base(shelfDir)) != shelfHashLength {
		t.Fatalf("shelf dir = %s", shelfDir)
	}
	if _, err := r.ShelfPatch("../x"); err == nil {
		t.Fatal("unsafe shelf id accepted")
	}
	if used, err := r.ShelfUsage(); err != nil || used != 0 {
		t.Fatalf("usage = %d, %v", used, err)
	}
}

func TestDiffTexts(t *testing.T) {
	dir := newTempRepo(t)
	writeFile(t, dir, "a.txt", "old\n")
	commitAll(t, dir, "init")
	writeFile(t, dir, "a.txt", "new\n")
	r := openTemp(t, dir)
	abs := filepath.Join(dir, "a.txt")
	o, n, err := loadDiffTexts(context.Background(), r, diffSource{kind: diffHead, path: "a.txt"}, abs, true)
	if err != nil || o != "old\n" || n != "new\n" {
		t.Fatalf("texts = %q %q %v", o, n, err)
	}
	writeFile(t, dir, "bin", "a\x00b")
	if _, _, err := loadDiffTexts(context.Background(), r, diffSource{kind: diffUntracked, path: "bin"}, filepath.Join(dir, "bin"), true); err != errBinary {
		t.Fatalf("binary err = %v", err)
	}
}
