package filetree

import (
	"context"
	"os/exec"
	"testing"

	"github.com/ZerGo0/editor/internal/testutil"
)

func TestParseIgnored(t *testing.T) {
	s := ParseIgnored([]byte("build/\x00debug.log\x00../outside\x00\x00nested/cache/\x00"))
	cases := map[string]bool{
		"build":             true,
		"build/out/app":     true,
		"debug.log":         true,
		"nested/cache/a.go": true,
		"nested":            false,
		"main.go":           false,
		"../outside":        false,
		"":                  false,
	}
	for rel, want := range cases {
		if got := s.Has(rel); got != want {
			t.Errorf("Has(%q) = %v, want %v", rel, got, want)
		}
	}
	if s.Len() != 3 {
		t.Errorf("Len = %d, want 3", s.Len())
	}
}

func TestLoadIgnoredOutsideRepo(t *testing.T) {
	s, err := LoadIgnored(context.Background(), t.TempDir())
	if err != nil || s.Len() != 0 {
		t.Fatalf("outside a repo: %v, %d entries", err, s.Len())
	}
}

func TestLoadIgnoredInRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	root := t.TempDir()
	if out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	files := map[string]string{".gitignore": "*.log\nbuild/\n", "a.log": "x", "build/out.txt": "x", "main.go": "x"}
	testutil.WriteFiles(t, root, files)
	s, err := LoadIgnored(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !s.Has("a.log") || !s.Has("build/out.txt") || s.Has("main.go") || s.Has(".gitignore") {
		t.Fatalf("ignored set is wrong: %+v", s)
	}
}
