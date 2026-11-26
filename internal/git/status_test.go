package git

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

const sampleStatus = "# branch.oid 1111111111111111111111111111111111111111\x00" +
	"# branch.head main\x00" +
	"# branch.upstream origin/main\x00" +
	"# branch.ab +2 -1\x00" +
	"1 .M N... 100644 100644 100644 aaaa bbbb dir/with space.txt\x00" +
	"1 A. N... 000000 100644 100644 0000 cccc héllo wörld.txt\x00" +
	"2 R. N... 100644 100644 100644 aaaa aaaa R100 new name.txt\x00old name.txt\x00" +
	"u UU N... 100644 100644 100644 100644 aaaa bbbb cccc conflict.txt\x00" +
	"u AA N... 000000 100644 100644 100644 0000 bbbb cccc both added.txt\x00" +
	"? untracked/file.txt\x00" +
	"! ignored.log\x00"

func TestParseStatus(t *testing.T) {
	st, err := parseStatus([]byte(sampleStatus))
	if err != nil {
		t.Fatal(err)
	}
	if st.Branch != "main" || st.Upstream != "origin/main" || st.Ahead != 2 || st.Behind != 1 || st.Detached {
		t.Fatalf("branch header: %+v", st)
	}
	if st.OID != "1111111111111111111111111111111111111111" {
		t.Fatalf("oid: %q", st.OID)
	}
	want := []FileStatus{
		{Path: "dir/with space.txt", Index: '.', WorkTree: 'M'},
		{Path: "héllo wörld.txt", Index: 'A', WorkTree: '.'},
		{Path: "new name.txt", OrigPath: "old name.txt", Index: 'R', WorkTree: '.'},
		{Path: "conflict.txt", Index: 'U', WorkTree: 'U', Conflict: true},
		{Path: "both added.txt", Index: 'A', WorkTree: 'A', Conflict: true},
		{Path: "untracked/file.txt", Index: '?', WorkTree: '?'},
		{Path: "ignored.log", Index: '!', WorkTree: '!'},
	}
	if !reflect.DeepEqual(st.Files, want) {
		t.Fatalf("files:\n got %+v\nwant %+v", st.Files, want)
	}
	if got := paths(st.Staged()); !equalStrings(got, []string{"héllo wörld.txt", "new name.txt"}) {
		t.Errorf("staged: %v", got)
	}
	if got := paths(st.Unstaged()); !equalStrings(got, []string{"dir/with space.txt"}) {
		t.Errorf("unstaged: %v", got)
	}
	if got := paths(st.Untracked()); !equalStrings(got, []string{"untracked/file.txt"}) {
		t.Errorf("untracked: %v", got)
	}
	if got := paths(st.Conflicted()); !equalStrings(got, []string{"conflict.txt", "both added.txt"}) {
		t.Errorf("conflicted: %v", got)
	}
}

func TestParseStatusDetachedAndInitial(t *testing.T) {
	st, err := parseStatus([]byte("# branch.oid (initial)\x00# branch.head (detached)\x00"))
	if err != nil {
		t.Fatal(err)
	}
	if !st.Detached || st.OID != "" || st.Branch != "" {
		t.Fatalf("unexpected: %+v", st)
	}
}

func TestParseStatusMalformed(t *testing.T) {
	bad := []string{
		"1 .M N...\x00",
		"2 R. N... 100644 100644 100644 aaaa aaaa R100 only-new\x00",
		"u UU N... 1 2 3\x00",
		"? \x00x",
		"?nospace\x00",
		"# branch.ab +x -y\x00",
		"# branch.ab +1\x00",
		"Z what\x00",
		"1 M N... 100644 100644 100644 aaaa bbbb file\x00",
	}
	for _, input := range bad {
		if _, err := parseStatus([]byte(input)); err == nil {
			t.Errorf("expected error for %q", input)
		}
	}
	for i := range sampleStatus {
		_, _ = parseStatus([]byte(sampleStatus[:i]))
	}
}

func TestStatusRepo(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	st, err := repo.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.Branch != "main" || st.OID != "" {
		t.Fatalf("unborn: %+v", st)
	}
	commitFile(t, repo, "keep.txt", "keep\n", "init")
	commitFile(t, repo, "old name.txt", numbered(20), "second")
	put(t, repo, "keep.txt", "changed\n")
	put(t, repo, "dür/sp ace.txt", "x\n")
	put(t, repo, "star*.txt", "x\n")
	g(t, repo, "mv", "old name.txt", "new name.txt")
	st, err = repo.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := paths(st.Untracked()); !equalStrings(got, []string{"dür/sp ace.txt", "star*.txt"}) {
		t.Errorf("untracked: %v", got)
	}
	if got := st.Staged(); len(got) != 1 || got[0].Path != "new name.txt" || got[0].OrigPath != "old name.txt" || got[0].Index != 'R' {
		t.Errorf("staged: %+v", got)
	}
	if got := paths(st.Unstaged()); !equalStrings(got, []string{"keep.txt"}) {
		t.Errorf("unstaged: %v", got)
	}
	if st.Branch != "main" || st.OID == "" {
		t.Errorf("header: %+v", st)
	}
}

func TestOpenNotRepo(t *testing.T) {
	_, err := Open(context.Background(), t.TempDir())
	if !errors.Is(err, ErrNotRepo) {
		t.Fatalf("got %v", err)
	}
}

func TestOpenSubdirectory(t *testing.T) {
	repo := newRepo(t)
	put(t, repo, "a/b/file.txt", "x")
	sub, err := Open(context.Background(), repo.Root+"/a/b")
	if err != nil {
		t.Fatal(err)
	}
	if sub.Root != repo.Root || !strings.HasSuffix(sub.GitDir, ".git") {
		t.Fatalf("unexpected repo %+v", sub)
	}
}

func TestErrorMessage(t *testing.T) {
	sample := &Error{Args: []string{"commit", "-m", "x"}, Stderr: "fatal: first line\nsecond", ExitCode: 128}
	if got := sample.Error(); got != "git commit: fatal: first line (exit 128)" {
		t.Fatalf("got %q", got)
	}
	if got := (&Error{Args: []string{"push"}, ExitCode: 1}).Error(); got != "git push: exit status 1" {
		t.Fatalf("got %q", got)
	}
	repo := newRepo(t)
	_, err := repo.read(context.Background(), "show", "nonexistent-rev")
	var ge *Error
	if !errors.As(err, &ge) || ge.ExitCode == 0 || ge.Stderr == "" {
		t.Fatalf("got %v", err)
	}
}

func TestCancelledContext(t *testing.T) {
	repo := newRepo(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := repo.Status(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}

func TestAvailable(t *testing.T) {
	if !Available() {
		t.Fatal("git should be on PATH")
	}
}

func TestShow(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	if _, err := repo.Show(ctx, "HEAD", "a.txt"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unborn HEAD: %v", err)
	}
	commitFile(t, repo, "a.txt", "one\n", "one")
	put(t, repo, "a.txt", "two\n")
	g(t, repo, "add", "a.txt")
	put(t, repo, "a.txt", "three\n")
	for rev, want := range map[string]string{"HEAD": "one\n", "": "two\n"} {
		got, err := repo.Show(ctx, rev, "a.txt")
		if err != nil || string(got) != want {
			t.Errorf("Show(%q) = %q, %v; want %q", rev, got, err, want)
		}
	}
	for _, c := range []struct{ rev, path string }{{"HEAD", "missing.txt"}, {"", "missing.txt"}, {":1", "a.txt"}, {"nosuchrev", "a.txt"}} {
		if _, err := repo.Show(ctx, c.rev, c.path); !errors.Is(err, ErrNotFound) {
			t.Errorf("Show(%q, %q): %v", c.rev, c.path, err)
		}
	}
	if _, err := repo.Show(ctx, "-x", "a.txt"); err == nil {
		t.Error("expected error for option-like rev")
	}
}
