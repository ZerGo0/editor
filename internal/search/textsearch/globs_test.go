package textsearch

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/ZerGo0/editor/internal/testutil"
)

func TestParseGlobs(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"src/**/*.go, *.ts", []string{"src/**/*.go", "*.ts"}},
		{" a ,, b ,", []string{"a", "b"}},
		{"*.{ts,tsx}, docs", []string{"*.ts", "*.tsx", "docs"}},
		{"{a,b}/{c,d}", []string{"a/c", "a/d", "b/c", "b/d"}},
		{"{unclosed,x", []string{"{unclosed,x"}},
	}
	for _, tc := range tests {
		if got := ParseGlobs(tc.in); !slices.Equal(got, tc.want) {
			t.Errorf("ParseGlobs(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestGlobSemantics(t *testing.T) {
	const root = "/work/proj"
	tests := []struct {
		pattern string
		rel     string
		want    bool
	}{
		{"*.go", "main.go", true},
		{"*.go", "a/b/main.go", true},
		{"*.go", "main.txt", false},
		{"internal", "internal/a/x.go", true},
		{"internal", "pkg/internal/x.go", true},
		{"./internal", "internal/x.go", true},
		{"./internal", "pkg/internal/x.go", false},
		{"./main.go", "main.go", true},
		{"./main.go", "sub/main.go", false},
		{"/work/proj/cmd", "cmd/tool/main.go", true},
		{"src/**/*.go", "src/a/b.go", true},
		{"src/**/*.go", "lib/a/b.go", false},
		{"node_modules/", "web/node_modules/x/y.js", true},
		{"**/node_modules", "node_modules/x.js", true},
		{"a/b", "a/b", true},
		{"a/b", "a/c", false},
	}
	for _, tc := range tests {
		set := newGlobSet([]string{tc.pattern}, root)
		if got := set.match(tc.rel); got != tc.want {
			t.Errorf("%q vs %q: got %v want %v", tc.pattern, tc.rel, got, tc.want)
		}
	}
}

func TestWantsWithRoot(t *testing.T) {
	m := mustCompile(t, Query{Pattern: "x", Root: "/r", Include: ParseGlobs("./src, *.md"), Exclude: ParseGlobs("**/gen")})
	want := map[string]bool{"src/a.go": true, "README.md": true, "src/gen/a.go": false, "other/a.go": false}
	for rel, w := range want {
		if got := m.Wants(rel); got != w {
			t.Errorf("Wants(%q)=%v want %v", rel, got, w)
		}
	}
	if !m.SkipDir("src/gen") || m.SkipDir("src") {
		t.Error("SkipDir should follow the exclude globs")
	}
}

func TestWalkFiles(t *testing.T) {
	root, _ := testutil.TempProject(t, map[string][]byte{"a.go": nil, "node_modules/x.js": nil, ".git/config": nil, "sub/b.go": nil})
	if err := os.Symlink(filepath.Join(root, "a.go"), filepath.Join(root, "link.go")); err != nil {
		t.Fatal(err)
	}
	m := mustCompile(t, Query{Pattern: "x", Exclude: []string{"node_modules"}})
	got, err := WalkFiles(context.Background(), root, m.SkipDir)
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(got)
	if want := []string{"a.go", "sub/b.go"}; !slices.Equal(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	all, _ := WalkFiles(context.Background(), root, nil)
	if !slices.Contains(all, "node_modules/x.js") || slices.Contains(all, ".git/config") {
		t.Fatalf("unfiltered walk %v", all)
	}
}
