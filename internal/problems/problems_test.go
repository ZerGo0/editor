package problems

import (
	"reflect"
	"strings"
	"testing"

	"github.com/ZerGo0/editor/internal/syntax"
)

func TestCheck(t *testing.T) {
	tests := []struct {
		name, file, text string
		want             []Problem
	}{
		{"balanced go", "a.go", "func f() {\n\tx := []int{1}\n}\n", nil},
		{"unmatched closer", "a.go", "x := 1)\n", []Problem{{0, 6, 7, Error, "Unmatched ')'", "brackets"}}},
		{"unclosed opener", "a.go", "func f() {\n\tx := 1\n", []Problem{{0, 9, 10, Error, "'{' is not closed", "brackets"}}},
		{"mismatch", "a.go", "x := (]\n", []Problem{
			{0, 5, 6, Error, "'(' is not closed", "brackets"},
			{0, 6, 7, Error, "Unmatched ']'", "brackets"},
		}},
		{"strings and comments skipped", "a.go", "s := \"(\" // {\n/* [ */\nr := `(\n`\n", nil},
		{"python strings", "a.py", "x = '('\n# )\nprint(x)\n", nil},
		{"markdown skipped", "a.md", "smile :)\n", nil},
		{"csv skipped", "a.csv", "a,(\n", nil},
		{"plain text", "a.unknownext", "(\n", nil},
		{"conflict", "a.txt", "a\n<<<<<<< HEAD\nb\n=======\nc\n>>>>>>> other\nd\n", []Problem{
			{1, 0, 7, Error, "Merge conflict marker", "git"},
			{3, 0, 7, Error, "Merge conflict marker", "git"},
			{5, 0, 7, Error, "Merge conflict marker", "git"},
		}},
		{"partial conflict", "a.txt", "<<<<<<< HEAD\nb\n=======\n", nil},
		{"markdown underline", "a.md", "Title\n=======\n", nil},
		{"json ok", "a.json", `{"a": [1, 2]}`, nil},
		{"json error", "a.json", "{\n  \"a\": 1,\n  \"b\" 2\n}", []Problem{
			{2, 6, 7, Error, "invalid character '2' after object key", "json"},
		}},
		{"json truncated", "a.json", "{\"a\": 1", []Problem{
			{0, 0, 1, Error, "'{' is not closed", "brackets"},
			{0, 7, 7, Error, "unexpected end of JSON input", "json"},
		}},
		{"json bom", "a.json", "\ufeff{\"a\" 1}", []Problem{{0, 8, 9, Error, "invalid character '1' after object key", "json"}}},
		{"json empty", "a.json", "  \n", nil},
		{"jsonc ext", "a.jsonc", "{\"a\": 1,}", nil},
		{"json5 ext", "a.json5", "{a: 1}", nil},
		{"jsonc settings", "dir/settings.json", "{\"a\": 1,}", nil},
		{"vscode dir", "/p/.vscode/x.json", "{\"a\": 1,}", nil},
		{"yaml tab", "a.yaml", "a:\n\tb: 1\n  c: 2\n", []Problem{{1, 0, 1, Warning, "Tab used for indentation", "yaml"}}},
		{"yaml tab after spaces", "a.yml", "a:\n  \tb: 1\n", []Problem{{1, 2, 3, Warning, "Tab used for indentation", "yaml"}}},
		{"yaml tab in value", "a.yaml", "a:\tb\n\t# comment\n\t\n", nil},
		{"mixed eol", "a.txt", "a\r\nb\nc\r\n", []Problem{{0, 0, 0, Info, "Mixed line endings (2 CRLF, 1 LF)", "eol"}}},
		{"crlf only", "a.txt", "a\r\nb\r\n", nil},
		{"lf only", "a.txt", "a\nb\n", nil},
		{"crlf brackets", "a.go", "f(\r\n)\r\n", nil},
		{"empty", "a.go", "", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Check(syntax.ForName(tc.file), tc.file, tc.text)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Check = %+v\nwant    %+v", got, tc.want)
			}
		})
	}
}

func TestCheckNilLanguage(t *testing.T) {
	got := Check(nil, "a.txt", "(\n<<<<<<< x\n=======\n>>>>>>> y\n")
	if len(got) != 3 {
		t.Fatalf("Check = %+v, want 3 conflict markers", got)
	}
}

func TestCheckCapsProblems(t *testing.T) {
	got := Check(syntax.ForName("a.go"), "a.go", strings.Repeat(")", MaxProblemsPerKind*3))
	if len(got) != MaxProblemsPerKind {
		t.Fatalf("closers: %d problems, want %d", len(got), MaxProblemsPerKind)
	}
	got = Check(syntax.ForName("a.go"), "a.go", strings.Repeat("(", MaxProblemsPerKind*3))
	if len(got) != MaxProblemsPerKind {
		t.Fatalf("openers: %d problems, want %d", len(got), MaxProblemsPerKind)
	}
}

func TestCheckStopsOverMaxSize(t *testing.T) {
	text := ")" + strings.Repeat("x", MaxTextSize)
	if got := Check(syntax.ForName("a.go"), "a.go", text); got != nil {
		t.Fatalf("Check = %d problems, want nil", len(got))
	}
}

func TestCheckSortedByPosition(t *testing.T) {
	text := "a)\n<<<<<<< x\n=======\n>>>>>>> y\nb)\n"
	got := Check(syntax.ForName("a.go"), "a.go", text)
	for i := 1; i < len(got); i++ {
		if got[i].Line < got[i-1].Line {
			t.Fatalf("problems not sorted: %+v", got)
		}
	}
	if len(got) != 5 {
		t.Fatalf("got %d problems, want 5: %+v", len(got), got)
	}
}

func TestSeverityString(t *testing.T) {
	for sev, want := range map[Severity]string{Error: "error", Warning: "warning", Info: "info"} {
		if got := sev.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", sev, got, want)
		}
	}
}
