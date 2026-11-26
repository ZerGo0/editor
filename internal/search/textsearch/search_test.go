package textsearch

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func mustCompile(t *testing.T, q Query) *Matcher {
	t.Helper()
	m, err := Compile(q)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestFindAll(t *testing.T) {
	tests := []struct {
		name string
		q    Query
		line string
		want [][2]int
	}{
		{"literal escapes", Query{Pattern: "a.b", MatchCase: true}, "a.b axb", [][2]int{{0, 3}}},
		{"case insensitive", Query{Pattern: "foo"}, "Foo foo FOO", [][2]int{{0, 3}, {4, 7}, {8, 11}}},
		{"match case", Query{Pattern: "foo", MatchCase: true}, "Foo foo", [][2]int{{4, 7}}},
		{"whole word", Query{Pattern: "foo", WholeWord: true}, "foo foobar _foo foo", [][2]int{{0, 3}, {16, 19}}},
		{"regex", Query{Pattern: "f.o+", Regex: true}, "fxoo fo", [][2]int{{0, 4}}},
		{"empty matches dropped", Query{Pattern: "x*", Regex: true}, "ab", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := mustCompile(t, tc.q).FindAll(tc.line)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v want %v", got, tc.want)
				}
			}
		})
	}
}

func TestCompileErrors(t *testing.T) {
	if _, err := Compile(Query{}); err == nil {
		t.Fatal("empty pattern accepted")
	}
	_, err := Compile(Query{Pattern: "(", Regex: true})
	var pe *PatternError
	if !errors.As(err, &pe) || strings.Contains(pe.Err.Error(), "(?i)") {
		t.Fatalf("invalid regex error should describe the user's pattern: %v", err)
	}
	if _, err := Compile(Query{Pattern: "("}); err != nil {
		t.Fatalf("literal paren rejected: %v", err)
	}
}

func TestWants(t *testing.T) {
	m := mustCompile(t, Query{Pattern: "x", Include: []string{"*.go"}, Exclude: []string{"vendor/**"}})
	cases := map[string]bool{"a/b.go": true, "a/b.txt": false, "vendor/x.go": false}
	for rel, want := range cases {
		if got := m.Wants(rel); got != want {
			t.Errorf("Wants(%q)=%v want %v", rel, got, want)
		}
	}
	if !mustCompile(t, Query{Pattern: "x"}).Wants("any/file") {
		t.Error("no globs should want everything")
	}
}

func TestMatchCutting(t *testing.T) {
	prefix := strings.Repeat("é", 500)
	line := prefix + "needle" + prefix
	m := mustCompile(t, Query{Pattern: "needle"})
	r := m.FindAll(line)[0]
	text, from, to := cutPreview(line, r[0], r[1])
	if len(text) > MaxMatchTextBytes || !utf8.ValidString(text) {
		t.Fatalf("bad cut: len=%d valid=%v", len(text), utf8.ValidString(text))
	}
	if text[from:to] != "needle" {
		t.Fatalf("bad range %q", text[from:to])
	}
	long := "ab" + strings.Repeat("é", 400)
	text, _, to = cutPreview(long, 0, len(long))
	if !utf8.ValidString(text) || to > len(text) {
		t.Fatalf("bad long match cut")
	}
}

func writeFiles(t *testing.T, files map[string][]byte) (string, []string) {
	t.Helper()
	root := t.TempDir()
	var paths []string
	for name, data := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}
	return root, paths
}

func run(t *testing.T, m *Matcher, o Options) ([]FileResult, Stats, error) {
	t.Helper()
	var rs []FileResult
	st, err := Search(context.Background(), m, o, func(r FileResult) { rs = append(rs, r) })
	SortResults(rs)
	return rs, st, err
}

func TestSearchBasics(t *testing.T) {
	root, paths := writeFiles(t, map[string][]byte{
		"a.go":        []byte("one\r\nTwo needle\r\nthree"),
		"b.txt":       []byte("needle"),
		"bin.dat":     []byte("needle\x00needle"),
		"sub/c.go":    []byte("none\nneedle needle\n"),
		"vendor/d.go": []byte("needle"),
	})
	m := mustCompile(t, Query{Pattern: "needle", Include: []string{"*.go", "*.dat"}, Exclude: []string{"vendor/**"}})
	rs, st, err := run(t, m, Options{Root: root, Files: paths})
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 2 || filepath.Base(rs[0].Path) != "a.go" || filepath.Base(rs[1].Path) != "c.go" {
		t.Fatalf("results %+v", rs)
	}
	a := rs[0].Matches[0]
	if a.Start != (Pos{Line: 1, Col: 4}) || a.End != (Pos{Line: 1, Col: 10}) || a.Preview != "Two needle" || a.PreviewStart != 4 || a.PreviewEnd != 10 {
		t.Fatalf("crlf match %+v", a)
	}
	if len(rs[1].Matches) != 2 || rs[1].Matches[0].Start.Line != 1 {
		t.Fatalf("sub matches %+v", rs[1].Matches)
	}
	if st.Skipped != 1 || st.Matched != 2 || st.Files != 2 || st.Truncated {
		t.Fatalf("stats %+v", st)
	}
}

func TestSearchSizeSkip(t *testing.T) {
	root, paths := writeFiles(t, map[string][]byte{"big.txt": []byte(strings.Repeat("needle\n", 100)), "small.txt": []byte("needle")})
	rs, st, err := run(t, mustCompile(t, Query{Pattern: "needle"}), Options{Root: root, Files: paths, MaxFileSize: 100})
	if err != nil || len(rs) != 1 || st.Skipped != 1 {
		t.Fatalf("rs=%v st=%+v err=%v", rs, st, err)
	}
}

func TestSearchOverlay(t *testing.T) {
	root, paths := writeFiles(t, map[string][]byte{"a.txt": []byte("disk"), "b.txt": []byte("disk")})
	overlay := func(p string) (string, bool) {
		if filepath.Base(p) == "a.txt" {
			return "buffer text", true
		}
		return "", false
	}
	rs, _, err := run(t, mustCompile(t, Query{Pattern: "buffer"}), Options{Root: root, Files: paths, Overlay: overlay})
	if err != nil || len(rs) != 1 || filepath.Base(rs[0].Path) != "a.txt" {
		t.Fatalf("rs=%v err=%v", rs, err)
	}
	rs, _, _ = run(t, mustCompile(t, Query{Pattern: "disk"}), Options{Root: root, Files: paths, Overlay: overlay})
	if len(rs) != 1 || filepath.Base(rs[0].Path) != "b.txt" {
		t.Fatalf("overlay should hide disk text: %v", rs)
	}
}

func TestSearchLimit(t *testing.T) {
	files := map[string][]byte{}
	for _, n := range []string{"a", "b", "c", "d"} {
		files[n+".txt"] = []byte(strings.Repeat("needle\n", 10))
	}
	root, paths := writeFiles(t, files)
	rs, st, err := run(t, mustCompile(t, Query{Pattern: "needle"}), Options{Root: root, Files: paths, Limit: 25, Workers: 2})
	if err != nil || !st.Truncated {
		t.Fatalf("st=%+v err=%v", st, err)
	}
	total := 0
	for _, r := range rs {
		total += len(r.Matches)
	}
	if total != 25 {
		t.Fatalf("total %d", total)
	}
	_, st, _ = run(t, mustCompile(t, Query{Pattern: "needle"}), Options{Root: root, Files: paths, Limit: 40})
	if st.Truncated {
		t.Fatal("exact limit must not truncate")
	}
}

func TestSearchCancel(t *testing.T) {
	root, paths := writeFiles(t, map[string][]byte{"a.txt": []byte("needle")})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Search(ctx, mustCompile(t, Query{Pattern: "needle"}), Options{Root: root, Files: paths}, func(FileResult) {})
	if err != context.Canceled {
		t.Fatalf("err=%v", err)
	}
}
