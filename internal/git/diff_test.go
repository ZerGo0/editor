package git

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

const sampleDiff = "diff --git a/main.go b/main.go\n" +
	"index 1111111..2222222 100644\n" +
	"--- a/main.go\n" +
	"+++ b/main.go\n" +
	"@@ -1,4 +1,5 @@ func main() {\n" +
	" a\n" +
	"-b\n" +
	"+B\n" +
	"+B2\n" +
	" c\n" +
	" d\n" +
	"@@ -20,2 +21,2 @@\n" +
	" x\n" +
	"-y\n" +
	"\\ No newline at end of file\n" +
	"+z\n" +
	"\\ No newline at end of file\n" +
	"diff --git a/new file.txt b/new file.txt\n" +
	"new file mode 100644\n" +
	"index 0000000..3333333\n" +
	"--- /dev/null\n" +
	"+++ b/new file.txt\t\n" +
	"@@ -0,0 +1 @@\n" +
	"+hello\n" +
	"diff --git a/gone.txt b/gone.txt\n" +
	"deleted file mode 100644\n" +
	"index 4444444..0000000\n" +
	"--- a/gone.txt\n" +
	"+++ /dev/null\n" +
	"@@ -1,2 +0,0 @@\n" +
	"-one\n" +
	"-two\n" +
	"diff --git a/old name.txt b/renamed dir/new name.txt\n" +
	"similarity index 90%\n" +
	"rename from old name.txt\n" +
	"rename to renamed dir/new name.txt\n" +
	"index 5555555..6666666 100644\n" +
	"--- a/old name.txt\n" +
	"+++ b/renamed dir/new name.txt\n" +
	"@@ -1 +1 @@\n" +
	"-before\n" +
	"+after\n" +
	"diff --git a/img.png b/img.png\n" +
	"index 7777777..8888888 100644\n" +
	"Binary files a/img.png and b/img.png differ\n" +
	"diff --git \"a/q\\\"uote\\303\\244.txt\" \"b/q\\\"uote\\303\\244.txt\"\n" +
	"index 9999999..aaaaaaa 100644\n" +
	"--- \"a/q\\\"uote\\303\\244.txt\"\n" +
	"+++ \"b/q\\\"uote\\303\\244.txt\"\n" +
	"@@ -1 +1 @@\n" +
	"-p\n" +
	"+q\n" +
	"diff --git a/mode.sh b/mode.sh\n" +
	"old mode 100644\n" +
	"new mode 100755\n"

func TestParseDiffSample(t *testing.T) {
	t.Parallel()
	files, err := parseDiff([]byte(sampleDiff))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 7 {
		t.Fatalf("got %d files", len(files))
	}
	main := files[0]
	if main.OldPath != "main.go" || main.NewPath != "main.go" || len(main.Hunks) != 2 {
		t.Fatalf("main: %+v", main)
	}
	h := main.Hunks[0]
	if h.OldStart != 1 || h.OldLines != 4 || h.NewStart != 1 || h.NewLines != 5 || h.Header != "func main() {" {
		t.Fatalf("hunk header: %+v", h)
	}
	wantLines := []Line{
		{Kind: ' ', Text: "a", OldNo: 1, NewNo: 1},
		{Kind: '-', Text: "b", OldNo: 2},
		{Kind: '+', Text: "B", NewNo: 2},
		{Kind: '+', Text: "B2", NewNo: 3},
		{Kind: ' ', Text: "c", OldNo: 3, NewNo: 4},
		{Kind: ' ', Text: "d", OldNo: 4, NewNo: 5},
	}
	if !reflect.DeepEqual(h.Lines, wantLines) {
		t.Fatalf("lines:\n got %+v\nwant %+v", h.Lines, wantLines)
	}
	second := main.Hunks[1]
	if !second.Lines[1].NoEOL || !second.Lines[2].NoEOL || second.Lines[0].NoEOL {
		t.Fatalf("no-eol flags: %+v", second.Lines)
	}
	if !files[1].IsNew() || files[1].NewPath != "new file.txt" || files[1].Hunks[0].OldLines != 0 {
		t.Fatalf("new: %+v", files[1])
	}
	if !files[2].IsDeleted() || files[2].OldPath != "gone.txt" || len(files[2].Hunks[0].Lines) != 2 {
		t.Fatalf("deleted: %+v", files[2])
	}
	if !files[3].IsRename() || files[3].OldPath != "old name.txt" || files[3].NewPath != "renamed dir/new name.txt" {
		t.Fatalf("rename: %+v", files[3])
	}
	if !files[4].Binary || files[4].NewPath != "img.png" || len(files[4].Hunks) != 0 {
		t.Fatalf("binary: %+v", files[4])
	}
	if files[5].NewPath != "q\"uoteä.txt" || files[5].OldPath != "q\"uoteä.txt" || len(files[5].Hunks) != 1 {
		t.Fatalf("quoted: %+v", files[5])
	}
	if files[6].Path() != "mode.sh" || len(files[6].Hunks) != 0 {
		t.Fatalf("mode: %+v", files[6])
	}
}

func TestParseDiffMalformed(t *testing.T) {
	t.Parallel()
	bad := []string{
		"@@ -1 +1 @@\n-a\n+b\n",
		"diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -1,3 +1,3 @@\n a\n",
		"diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ nonsense @@\n",
		"diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -1 +1 @@\n?bad\n",
		"diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -1 +1 @@\n-a\n-b\n+c\n",
	}
	for _, input := range bad {
		if _, err := parseDiff([]byte(input)); err == nil {
			t.Errorf("expected error for %q", input)
		}
	}
	for i := range sampleDiff {
		_, _ = parseDiff([]byte(sampleDiff[:i]))
	}
	files, err := parseDiff([]byte("diff --cc conflict.txt\nindex 1,2..3\n--- a/conflict.txt\n+++ b/conflict.txt\n@@@ -1,3 -1,3 +1,7 @@@\n  a\n++<<<<<<< HEAD\n"))
	if err != nil || len(files) != 1 || files[0].Path() != "conflict.txt" || len(files[0].Hunks) != 0 {
		t.Fatalf("combined diff: %+v %v", files, err)
	}
}

func TestDiffCommands(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	ctx := context.Background()
	first := commitFile(t, repo, "a.txt", numbered(10), "first")
	put(t, repo, "a.txt", strings.Replace(numbered(10), "line 5\n", "five\n", 1))
	put(t, repo, "sp ace/ü.txt", "new\n")
	g(t, repo, "add", "sp ace/ü.txt")
	work, err := repo.DiffWorkTree(ctx)
	if err != nil || len(work) != 1 || work[0].Path() != "a.txt" || len(work[0].Hunks) != 1 {
		t.Fatalf("worktree: %+v %v", work, err)
	}
	staged, err := repo.DiffStaged(ctx)
	if err != nil || len(staged) != 1 || staged[0].Path() != "sp ace/ü.txt" || !staged[0].IsNew() {
		t.Fatalf("staged: %+v %v", staged, err)
	}
	only, err := repo.DiffWorkTree(ctx, "nothing.txt")
	if err != nil || len(only) != 0 {
		t.Fatalf("filtered: %+v %v", only, err)
	}
	second := commitAll(t, repo, "second")
	rootDiff, err := repo.DiffCommit(ctx, first)
	if err != nil || len(rootDiff) != 1 || !rootDiff[0].IsNew() || len(rootDiff[0].Hunks[0].Lines) != 10 {
		t.Fatalf("root commit: %+v %v", rootDiff, err)
	}
	last, err := repo.DiffCommit(ctx, second)
	if err != nil || len(last) != 2 {
		t.Fatalf("second commit: %+v %v", last, err)
	}
	between, err := repo.DiffCommits(ctx, first, second, "a.txt")
	if err != nil || len(between) != 1 || between[0].Hunks[0].Lines[3].Text != "line 5" || between[0].Hunks[0].Lines[3].Kind != '-' {
		t.Fatalf("between: %+v %v", between, err)
	}
	g(t, repo, "mv", "a.txt", "b.txt")
	renamed, err := repo.DiffStaged(ctx, "a.txt", "b.txt")
	if err != nil || len(renamed) != 1 || !renamed[0].IsRename() {
		t.Fatalf("rename: %+v %v", renamed, err)
	}
	if _, err := repo.DiffCommit(ctx, "--output=x"); err == nil {
		t.Error("expected error for option-like rev")
	}
}
