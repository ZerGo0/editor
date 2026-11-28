package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDescribe(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		err  error
		want []string
		not  []string
	}{
		{name: "not git", err: errors.New("x")},
		{name: "unavailable", err: ErrUnavailable, want: []string{"Git is not installed", "check again"}},
		{name: "hook output", err: &Error{Args: []string{"commit"}, ExitCode: 1, Stderr: "lint: checking files...\nlint: a.go\nlint: 3 problems found"},
			want: []string{"lint: checking files...\nlint: a.go\nlint: 3 problems found"}, not: []string{"exit"}},
		{name: "silent hook", err: &Error{Args: []string{"commit"}, ExitCode: 1}, want: []string{"exit code 1"}},
		{name: "index lock", err: &Error{Args: []string{"add"}, ExitCode: 128, Dir: "/private/var/x/repo",
			Stderr: "fatal: Unable to create '/private/var/x/repo/.git/index.lock': File exists."},
			want: []string{"Another Git process", "delete .git/index.lock", "'.git/index.lock'"}, not: []string{"/private/var"}},
		{name: "sign in", err: &Error{Args: []string{"fetch"}, ExitCode: 128,
			Stderr: "fatal: could not read Username for 'https://github.example.com': terminal prompts disabled"},
			want: []string{"could not sign in to github.example.com", "gh auth setup-git"}},
		{name: "ssh key", err: &Error{Args: []string{"push"}, ExitCode: 128, Stderr: "git@github.com: Permission denied (publickey).\nfatal: Could not read from remote repository."},
			want: []string{"sign in to github.com with SSH"}},
		{name: "offline", err: &Error{Args: []string{"fetch"}, ExitCode: 128, Stderr: "fatal: unable to access 'https://example.org/r.git/': Could not resolve host: example.org"},
			want: []string{"Cannot reach example.org"}},
		{name: "progress dropped", err: &Error{Args: []string{"fetch"}, ExitCode: 1, Stderr: "remote: Counting objects:  50% (1/2)\rremote: Counting objects: 100% (2/2), done.\nerror: boom"},
			want: []string{"error: boom"}, not: []string{"Counting"}},
		{name: "damaged", err: fmt.Errorf("%w: %w", ErrDamaged, &Error{Args: []string{"rev-parse"}, ExitCode: 128, Stderr: "fatal: not a git repository"}),
			want: []string{"cannot read it", "not a git repository"}},
	}
	for _, tc := range cases {
		got, ok := Describe(tc.err)
		if ok != (len(tc.want) > 0) {
			t.Errorf("%s: ok = %v", tc.name, ok)
		}
		for _, w := range tc.want {
			if !strings.Contains(got, w) {
				t.Errorf("%s: %q does not contain %q", tc.name, got, w)
			}
		}
		for _, n := range tc.not {
			if strings.Contains(got, n) {
				t.Errorf("%s: %q contains %q", tc.name, got, n)
			}
		}
	}
}

func TestDescribeKeepsTheTailOfLongOutput(t *testing.T) {
	t.Parallel()
	var lines []string
	for i := range describeMaxLines + 5 {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	got, _ := Describe(&Error{Args: []string{"commit"}, ExitCode: 1, Stderr: strings.Join(lines, "\n")})
	if !strings.HasPrefix(got, "(5 earlier lines not shown)") || !strings.HasSuffix(got, fmt.Sprintf("line %d", describeMaxLines+4)) {
		t.Errorf("Describe = %q", got)
	}
}

func TestOpenReportsDamagedRepository(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	if err := os.WriteFile(filepath.Join(repo.GitDir, "HEAD"), []byte("garbage\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Open(context.Background(), repo.Root)
	if !errors.Is(err, ErrDamaged) || errors.Is(err, ErrNotRepo) {
		t.Fatalf("Open = %v, want ErrDamaged", err)
	}
	if _, err := Open(context.Background(), t.TempDir()); !errors.Is(err, ErrNotRepo) {
		t.Fatalf("Open of a plain folder = %v, want ErrNotRepo", err)
	}
}
