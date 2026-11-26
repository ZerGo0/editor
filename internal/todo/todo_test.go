package todo

import (
	"reflect"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui/editor"

	"github.com/ZerGo0/editor/internal/syntax"
)

func TestScan(t *testing.T) {
	goLang := syntax.ForName("a.go")
	pyLang := syntax.ForName("a.py")
	tests := []struct {
		name     string
		lang     *editor.Language
		text     string
		patterns []string
		want     []Item
	}{
		{"line comment", goLang, "x := 1 // TODO: fix this\n", nil, []Item{{0, 10, "TODO", "fix this"}}},
		{"with name", goLang, "// FIXME(bob): broken", nil, []Item{{0, 3, "FIXME", "broken"}}},
		{"no colon", goLang, "// HACK works for now", nil, []Item{{0, 3, "HACK", "works for now"}}},
		{"string is not a comment", goLang, "s := \"TODO: no\"\n", nil, nil},
		{"identifier is not a tag", goLang, "// TODOS and MYTODO and todo\n", nil, nil},
		{"case sensitive", goLang, "// todo: lower\n", nil, nil},
		{"block comment lines", goLang, "/* start\n * NOTE: inside\n */\nx := 1\n/* BUG: one */", nil, []Item{
			{1, 3, "NOTE", "inside"}, {4, 3, "BUG", "one"},
		}},
		{"python hash", pyLang, "x = 1  # XXX: later\n", nil, []Item{{0, 9, "XXX", "later"}}},
		{"techdebt", goLang, "// TECHDEBT: bounded; stream it", nil, []Item{{0, 3, "TECHDEBT", "bounded; stream it"}}},
		{"custom patterns", goLang, "// TODO: a\n// REVIEW: b\n", []string{"REVIEW"}, []Item{{1, 3, "REVIEW", "b"}}},
		{"crlf", goLang, "// TODO: a\r\nx\r\n// FIXME: b\r\n", nil, []Item{{0, 3, "TODO", "a"}, {2, 3, "FIXME", "b"}}},
		{"empty", goLang, "", nil, nil},
		{"fallback hash", nil, "  # TODO: x\nplain TODO\n", nil, []Item{{0, 4, "TODO", "x"}}},
		{"fallback slashes", nil, "// FIXME y\n-- NOTE: z\n; HACK q\n/* BUG w */\n * XXX v\n", nil, []Item{
			{0, 3, "FIXME", "y"}, {1, 3, "NOTE", "z"}, {2, 2, "HACK", "q"}, {3, 3, "BUG", "w */"}, {4, 3, "XXX", "v"},
		}},
		{"nil tokenizer", &editor.Language{Name: "x"}, "# TODO: a\n", nil, []Item{{0, 2, "TODO", "a"}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Scan(tc.lang, tc.text, tc.patterns)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Scan = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestScanSkipsLongLines(t *testing.T) {
	long := "// TODO: " + strings.Repeat("a", maxLineLen)
	got := Scan(syntax.ForName("a.go"), long+"\n// TODO: short\n", nil)
	want := []Item{{1, 3, "TODO", "short"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Scan = %+v, want %+v", got, want)
	}
}

func TestScanKeepsStateAcrossLongLines(t *testing.T) {
	long := "/* " + strings.Repeat("a", maxLineLen)
	got := Scan(syntax.ForName("a.go"), long+"\nTODO: still comment\n*/\n", nil)
	want := []Item{{1, 0, "TODO", "still comment"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Scan = %+v, want %+v", got, want)
	}
}
