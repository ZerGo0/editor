package git

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func lineSet(pairs ...[2]int) func(int, int) bool {
	return func(h, l int) bool {
		for _, p := range pairs {
			if p[0] == h && p[1] == l {
				return true
			}
		}
		return false
	}
}

func modifiedDiff() FileDiff {
	return FileDiff{
		OldPath: "f.txt", NewPath: "f.txt",
		Hunks: []Hunk{
			{OldStart: 1, OldLines: 4, NewStart: 1, NewLines: 5, Lines: []Line{
				{Kind: ' ', Text: "a"},
				{Kind: '-', Text: "b"},
				{Kind: '+', Text: "B"},
				{Kind: '+', Text: "B2"},
				{Kind: ' ', Text: "c"},
				{Kind: ' ', Text: "d"},
			}},
			{OldStart: 20, OldLines: 2, NewStart: 21, NewLines: 2, Lines: []Line{
				{Kind: ' ', Text: "x"},
				{Kind: '-', Text: "y"},
				{Kind: '+', Text: "z"},
			}},
		},
	}
}

func TestBuildPatchForward(t *testing.T) {
	fd := modifiedDiff()
	got := string(BuildPatch(fd, lineSet([2]int{0, 3}, [2]int{1, 1}, [2]int{1, 2}), false))
	want := "diff --git a/f.txt b/f.txt\n--- a/f.txt\n+++ b/f.txt\n" +
		"@@ -1,4 +1,5 @@\n a\n b\n+B2\n c\n d\n" +
		"@@ -20,2 +21,2 @@\n x\n-y\n+z\n"
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
	if p := BuildPatch(fd, func(int, int) bool { return false }, false); p != nil {
		t.Fatalf("expected nil patch, got %q", p)
	}
	got = string(BuildPatch(fd, lineSet([2]int{1, 1}, [2]int{1, 2}), false))
	if !strings.Contains(got, "@@ -20,2 +20,2 @@") || strings.Contains(got, "@@ -1,") {
		t.Fatalf("second hunk only:\n%s", got)
	}
}

func TestBuildPatchReverse(t *testing.T) {
	fd := modifiedDiff()
	got := string(BuildPatch(fd, lineSet([2]int{0, 1}), true))
	want := "diff --git a/f.txt b/f.txt\n--- a/f.txt\n+++ b/f.txt\n" +
		"@@ -1,6 +1,5 @@\n a\n-b\n B\n B2\n c\n d\n"
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
	got = string(BuildPatch(fd, lineSet([2]int{0, 3}, [2]int{1, 2}), true))
	if !strings.Contains(got, "@@ -1,4 +1,5 @@\n a\n B\n+B2\n c\n d\n") || !strings.Contains(got, "@@ -20 +21,2 @@\n x\n+z\n") {
		t.Fatalf("got:\n%s", got)
	}
}

func TestBuildPatchNewAndDeletedFiles(t *testing.T) {
	added := FileDiff{NewPath: "n.txt", Header: []string{"new file mode 100755"}, Hunks: []Hunk{{OldStart: 0, OldLines: 0, NewStart: 1, NewLines: 3, Lines: []Line{
		{Kind: '+', Text: "1"}, {Kind: '+', Text: "2"}, {Kind: '+', Text: "3"},
	}}}}
	forward := string(BuildPatch(added, lineSet([2]int{0, 0}, [2]int{0, 2}), false))
	wantForward := "diff --git a/n.txt b/n.txt\nnew file mode 100755\n--- /dev/null\n+++ b/n.txt\n@@ -0,0 +1,2 @@\n+1\n+3\n"
	if forward != wantForward {
		t.Fatalf("forward:\n%s", forward)
	}
	partialReverse := string(BuildPatch(added, lineSet([2]int{0, 1}), true))
	wantReverse := "diff --git a/n.txt b/n.txt\n--- a/n.txt\n+++ b/n.txt\n@@ -1,3 +1,2 @@\n 1\n+2\n 3\n"
	wantReverse = strings.Replace(wantReverse, "@@ -1,3 +1,2 @@", "@@ -1,2 +1,3 @@", 1)
	if partialReverse != wantReverse {
		t.Fatalf("partial reverse:\n%s", partialReverse)
	}
	fullReverse := string(BuildPatch(added, func(int, int) bool { return true }, true))
	if !strings.Contains(fullReverse, "new file mode 100755") || !strings.Contains(fullReverse, "--- /dev/null") {
		t.Fatalf("full reverse:\n%s", fullReverse)
	}
	deleted := FileDiff{OldPath: "d.txt", Hunks: []Hunk{{OldStart: 1, OldLines: 2, NewStart: 0, NewLines: 0, Lines: []Line{
		{Kind: '-', Text: "1"}, {Kind: '-', Text: "2"},
	}}}}
	partial := string(BuildPatch(deleted, lineSet([2]int{0, 0}), false))
	if strings.Contains(partial, "deleted file") || strings.Contains(partial, "/dev/null") || !strings.Contains(partial, "@@ -1,2 +1 @@\n-1\n 2\n") {
		t.Fatalf("partial delete:\n%s", partial)
	}
	full := string(BuildPatch(deleted, func(int, int) bool { return true }, false))
	if !strings.Contains(full, "deleted file mode 100644") || !strings.Contains(full, "+++ /dev/null") {
		t.Fatalf("full delete:\n%s", full)
	}
	if BuildPatch(FileDiff{Binary: true, OldPath: "x", NewPath: "x"}, func(int, int) bool { return true }, false) != nil {
		t.Fatal("binary patch must be nil")
	}
	if got := quotePath("a\"b\n"); got != "\"a\\\"b\\n\"" {
		t.Fatalf("quotePath: %q", got)
	}
}

func fileDiff(t *testing.T, repo *Repo, staged bool) FileDiff {
	t.Helper()
	var files []FileDiff
	var err error
	if staged {
		files, err = repo.DiffStaged(context.Background())
	} else {
		files, err = repo.DiffWorkTree(context.Background())
	}
	if err != nil || len(files) != 1 {
		t.Fatalf("diff: %+v %v", files, err)
	}
	return files[0]
}

func changeLine(fd FileDiff, kind byte, text string) func(int, int) bool {
	return func(h, l int) bool {
		line := fd.Hunks[h].Lines[l]
		return line.Kind == kind && line.Text == text
	}
}

func TestPartialStaging(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	original := numbered(30)
	commitFile(t, repo, "f.txt", original, "init")
	modified := strings.Replace(original, "line 3\n", "THREE\nEXTRA\n", 1)
	modified = strings.Replace(modified, "line 25\n", "", 1)
	modified = strings.Replace(modified, "line 12\n", "twelve\n", 1)
	put(t, repo, "f.txt", modified)

	fd := fileDiff(t, repo, false)
	if len(fd.Hunks) != 3 {
		t.Fatalf("want 3 hunks, got %d", len(fd.Hunks))
	}
	patch := BuildPatch(fd, func(h, l int) bool {
		line := fd.Hunks[h].Lines[l]
		return line.Text == "EXTRA" || line.Text == "line 25" || line.Text == "line 12"
	}, false)
	if err := repo.Apply(ctx, patch, true, false); err != nil {
		t.Fatalf("apply: %v\n%s", err, patch)
	}
	want := strings.Replace(original, "line 3\n", "line 3\nEXTRA\n", 1)
	want = strings.Replace(want, "line 25\n", "", 1)
	want = strings.Replace(want, "line 12\n", "", 1)
	if got := g(t, repo, "show", ":f.txt") + "\n"; got != want {
		t.Fatalf("index mismatch:\n%s", got)
	}
	if got := slurp(t, repo, "f.txt"); got != modified {
		t.Fatal("worktree must stay untouched")
	}

	staged := fileDiff(t, repo, true)
	unstage := BuildPatch(staged, changeLine(staged, '+', "EXTRA"), true)
	if err := repo.Apply(ctx, unstage, true, true); err != nil {
		t.Fatalf("unstage: %v\n%s", err, unstage)
	}
	want = strings.Replace(original, "line 25\n", "", 1)
	want = strings.Replace(want, "line 12\n", "", 1)
	if got := g(t, repo, "show", ":f.txt") + "\n"; got != want {
		t.Fatalf("index after unstage:\n%s", got)
	}

	work := fileDiff(t, repo, false)
	discard := BuildPatch(work, changeLine(work, '+', "EXTRA"), true)
	if err := repo.Apply(ctx, discard, false, true); err != nil {
		t.Fatalf("discard: %v\n%s", err, discard)
	}
	if strings.Contains(slurp(t, repo, "f.txt"), "EXTRA") || !strings.Contains(slurp(t, repo, "f.txt"), "THREE") {
		t.Fatalf("worktree after discard:\n%s", slurp(t, repo, "f.txt"))
	}
	if err := repo.Apply(ctx, nil, true, false); !errors.Is(err, ErrEmptyPatch) {
		t.Fatalf("empty patch: %v", err)
	}
}

func TestPartialStagingNoNewlineAtEOF(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	commitFile(t, repo, "f.txt", "a\nb\nc", "init")
	put(t, repo, "f.txt", "A\nb\nc\nd")
	fd := fileDiff(t, repo, false)
	patch := BuildPatch(fd, func(h, l int) bool { return fd.Hunks[h].Lines[l].Text == "A" || fd.Hunks[h].Lines[l].Text == "a" }, false)
	if err := repo.Apply(ctx, patch, true, false); err != nil {
		t.Fatalf("apply: %v\n%s", err, patch)
	}
	if got := g(t, repo, "show", ":f.txt"); got != "A\nb\nc" {
		t.Fatalf("index: %q", got)
	}
}

func TestPartialStagingDeletedFile(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	commitFile(t, repo, "d.txt", "1\n2\n3\n", "init")
	put(t, repo, "d.txt", "")
	g(t, repo, "rm", "--quiet", "--cached", "--force", "d.txt")
	g(t, repo, "reset", "--quiet")
	if err := repo.Discard(ctx, "d.txt"); err != nil {
		t.Fatal(err)
	}
	if err := removePath(repo, "d.txt"); err != nil {
		t.Fatal(err)
	}
	fd := fileDiff(t, repo, false)
	if !fd.IsDeleted() {
		t.Fatalf("expected deletion: %+v", fd)
	}
	patch := BuildPatch(fd, changeLine(fd, '-', "2"), false)
	if err := repo.Apply(ctx, patch, true, false); err != nil {
		t.Fatalf("apply: %v\n%s", err, patch)
	}
	if got := g(t, repo, "show", ":d.txt"); got != "1\n3" {
		t.Fatalf("index: %q", got)
	}
}

func TestStageUnstageDiscard(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	put(t, repo, "star*.txt", "literal\n")
	put(t, repo, "starX.txt", "other\n")
	if err := repo.StageFiles(ctx, "star*.txt"); err != nil {
		t.Fatal(err)
	}
	st, _ := repo.Status(ctx)
	if got := paths(st.Staged()); !equalStrings(got, []string{"star*.txt"}) {
		t.Fatalf("staged on unborn branch: %v", got)
	}
	if err := repo.UnstageFiles(ctx, "star*.txt"); err != nil {
		t.Fatalf("unstage on unborn branch: %v", err)
	}
	st, _ = repo.Status(ctx)
	if len(st.Staged()) != 0 || len(st.Untracked()) != 2 {
		t.Fatalf("after unstage: %+v", st.Files)
	}
	commitAll(t, repo, "init")
	put(t, repo, "star*.txt", "changed\n")
	put(t, repo, "starX.txt", "changed\n")
	put(t, repo, "new.txt", "new\n")
	if err := repo.StageFiles(ctx, "star*.txt", "new.txt"); err != nil {
		t.Fatal(err)
	}
	st, _ = repo.Status(ctx)
	if got := paths(st.Staged()); !equalStrings(got, []string{"new.txt", "star*.txt"}) {
		t.Fatalf("staged: %v", got)
	}
	if err := repo.UnstageFiles(ctx, "star*.txt", "new.txt"); err != nil {
		t.Fatal(err)
	}
	st, _ = repo.Status(ctx)
	if len(st.Staged()) != 0 {
		t.Fatalf("still staged: %+v", st.Files)
	}
	if err := repo.Discard(ctx, "star*.txt"); err != nil {
		t.Fatal(err)
	}
	if slurp(t, repo, "star*.txt") != "literal\n" || slurp(t, repo, "starX.txt") != "changed\n" {
		t.Fatal("literal pathspec must discard only the star file")
	}
	if err := repo.Discard(ctx, "new.txt"); err == nil {
		t.Log("discard of an untracked path is a no-op or error")
	}
	if !exists(repo, "new.txt") {
		t.Fatal("Discard must keep untracked files")
	}
	if err := repo.DiscardUntracked(ctx, "new.txt"); err != nil {
		t.Fatal(err)
	}
	if exists(repo, "new.txt") || !exists(repo, "starX.txt") {
		t.Fatal("DiscardUntracked removed the wrong files")
	}
	if err := repo.StageFiles(ctx, "/etc/passwd"); err == nil {
		t.Fatal("absolute path must be rejected")
	}
}
