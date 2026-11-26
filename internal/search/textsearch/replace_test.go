package textsearch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func replaceAll(t *testing.T, q Query, text, tpl string, preserve bool) string {
	t.Helper()
	m := mustCompile(t, q)
	return m.PlanReplace(text, m.Replacement(tpl, preserve), nil).Apply()
}

func TestReplaceGroups(t *testing.T) {
	tests := []struct {
		name, text, tpl, want string
		q                     Query
	}{
		{"numbered", "a=1 b=2", "$2:$1", "1:a 2:b", Query{Pattern: `(\w)=(\d)`, Regex: true}},
		{"whole match", "foo bar", "[$&]", "[foo] [bar]", Query{Pattern: `\w+`, Regex: true}},
		{"dollar escape", "x", "$$1", "$1", Query{Pattern: "x", Regex: true}},
		{"missing group stays", "x", "$3", "$3", Query{Pattern: "x", Regex: true}},
		{"escapes", "x", `a\nb\t\\`, "a\nb\t\\", Query{Pattern: "x", Regex: true}},
		{"literal ignores groups", "a.b", "$1\\n", "$1\\n", Query{Pattern: "a.b"}},
		{"two digit group", "abcdefghijk", "$11", "k", Query{Pattern: "(a)(b)(c)(d)(e)(f)(g)(h)(i)(j)(k)", Regex: true}},
		{"unmatched optional group", "ac", "<$1>", "<>", Query{Pattern: "a(b)?c", Regex: true}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := replaceAll(t, tc.q, tc.text, tc.tpl, false); got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestPreserveCase(t *testing.T) {
	tests := []struct{ matched, repl, want string }{
		{"FOO", "bar", "BAR"},
		{"foo", "BAR", "bar"},
		{"Foo", "bar", "Bar"},
		{"fOo", "Bar", "bar"},
		{"foo-bar", "baz-qux", "baz-qux"},
		{"Foo-bar", "baz-qux", "Baz-qux"},
		{"FOO_BAR", "baz_qux", "BAZ_QUX"},
		{"Foo_bar", "baz_qux", "Baz_qux"},
	}
	for _, tc := range tests {
		r := Replacement{Template: tc.repl, PreserveCase: true}
		if got := r.Expand(tc.matched, nil); got != tc.want {
			t.Errorf("preserve(%q,%q)=%q want %q", tc.matched, tc.repl, got, tc.want)
		}
	}
	got := replaceAll(t, Query{Pattern: "hello"}, "hello Hello HELLO", "world", true)
	if got != "world World WORLD" {
		t.Fatalf("got %q", got)
	}
}

func TestPlanAccept(t *testing.T) {
	m := mustCompile(t, Query{Pattern: "a"})
	text := "a a\na"
	plan := m.PlanReplace(text, m.Replacement("X", false), func(mt Match) bool { return mt.Start == Pos{Line: 0, Col: 2} })
	if got := plan.Apply(); got != "a X\na" || len(plan.Edits) != 1 {
		t.Fatalf("got %q edits %v", got, plan.Edits)
	}
}

func TestPlanMultilineCRLF(t *testing.T) {
	m := mustCompile(t, Query{Pattern: "a\nb"})
	plan := m.PlanReplace("x\r\na\r\nb\r\ny\r\n", m.Replacement("one line", false), nil)
	if got := plan.Apply(); got != "x\r\none line\r\ny\r\n" {
		t.Fatalf("got %q", got)
	}
	single := mustCompile(t, Query{Pattern: "a"})
	if got := single.PlanReplace("a\r\na\r\n", single.Replacement("b", false), nil).Apply(); got != "b\r\nb\r\n" {
		t.Fatalf("single-line crlf: %q", got)
	}
}

func TestReplaceFileAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte("one needle\ntwo needle\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	m := mustCompile(t, Query{Pattern: "needle"})
	called := 0
	n, err := ReplaceFile(path, m, m.Replacement("pin", false), nil, 0, func() { called++ })
	if err != nil || n != 2 || called != 1 {
		t.Fatalf("n=%d err=%v called=%d", n, err, called)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "one pin\ntwo pin\n" {
		t.Fatalf("content %q", data)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("mode %v", info.Mode().Perm())
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("temp file left behind: %v", entries)
	}
	n, err = ReplaceFile(path, m, m.Replacement("x", false), nil, 0, func() { called++ })
	if err != nil || n != 0 || called != 1 {
		t.Fatalf("no-match rewrite: n=%d err=%v called=%d", n, err, called)
	}
}

func TestReplaceFileRefusals(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "b.bin")
	if err := os.WriteFile(bin, []byte("needle\x00needle"), 0o644); err != nil {
		t.Fatal(err)
	}
	big := filepath.Join(dir, "big.txt")
	if err := os.WriteFile(big, []byte(strings.Repeat("needle\n", 100)), 0o644); err != nil {
		t.Fatal(err)
	}
	m := mustCompile(t, Query{Pattern: "needle"})
	if _, err := ReplaceFile(bin, m, m.Replacement("x", false), nil, 0, nil); err == nil {
		t.Error("binary file accepted")
	}
	if _, err := ReplaceFile(big, m, m.Replacement("x", false), nil, 10, nil); err == nil {
		t.Error("oversized file accepted")
	}
	if _, err := ReplaceFile(filepath.Join(dir, "missing"), m, m.Replacement("x", false), nil, 0, nil); err == nil {
		t.Error("missing file accepted")
	}
	if data, _ := os.ReadFile(bin); !strings.Contains(string(data), "needle") {
		t.Error("refused file was modified")
	}
}
