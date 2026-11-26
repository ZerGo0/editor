package textsearch

import (
	"context"
	"strings"
	"testing"
)

func scan(t *testing.T, q Query, text string) []Match {
	t.Helper()
	return scanText(context.Background(), mustCompile(t, q), text, 100)
}

func TestMultilineLiteral(t *testing.T) {
	text := "alpha\r\nfoo\r\nbar\r\nomega\r\n"
	got := scan(t, Query{Pattern: "foo\r\nbar"}, text)
	if len(got) != 1 {
		t.Fatalf("got %+v", got)
	}
	m := got[0]
	if m.Start != (Pos{1, 0}) || m.End != (Pos{2, 3}) || !m.Multiline() {
		t.Fatalf("range %+v", m)
	}
	if m.Preview != "foo" || m.PreviewStart != 0 || m.PreviewEnd != 3 {
		t.Fatalf("preview %+v", m)
	}
	if m.Matched != "foo\nbar" {
		t.Fatalf("matched %q", m.Matched)
	}
}

func TestMultilineRegex(t *testing.T) {
	text := "func a() {\n\treturn 1\n}\nfunc b() {\n}\n"
	got := scan(t, Query{Pattern: `\{\n\s*return`, Regex: true}, text)
	if len(got) != 1 || got[0].Start != (Pos{0, 9}) || got[0].End.Line != 1 || got[0].End.Col != 7 {
		t.Fatalf("got %+v", got)
	}
	anchored := scan(t, Query{Pattern: `^func \w+\(\) \{\n\}$`, Regex: true}, text)
	if len(anchored) != 1 || anchored[0].Start.Line != 3 || anchored[0].End.Line != 4 {
		t.Fatalf("anchored %+v", anchored)
	}
}

func TestMultilineDetection(t *testing.T) {
	tests := []struct {
		q    Query
		want bool
	}{
		{Query{Pattern: "a\nb"}, true},
		{Query{Pattern: "a\r\nb"}, true},
		{Query{Pattern: `a\nb`}, false},
		{Query{Pattern: `a\nb`, Regex: true}, true},
		{Query{Pattern: `a\\nb`, Regex: true}, false},
		{Query{Pattern: `a\Wb`, Regex: true}, true},
		{Query{Pattern: `a\sb`, Regex: true}, false},
	}
	for _, tc := range tests {
		if got := mustCompile(t, tc.q).Multiline(); got != tc.want {
			t.Errorf("%q regex=%v: got %v want %v", tc.q.Pattern, tc.q.Regex, got, tc.want)
		}
	}
}

func TestRuneColumns(t *testing.T) {
	got := scan(t, Query{Pattern: "x"}, "éé x é x\nλλ\nx")
	if len(got) != 3 {
		t.Fatalf("got %+v", got)
	}
	if got[0].Start.Col != 5 || got[0].StartRune != 3 || got[0].EndRune != 4 {
		t.Fatalf("first %+v", got[0])
	}
	if got[1].Start.Col != 10 || got[1].StartRune != 7 {
		t.Fatalf("second %+v", got[1])
	}
	span := scan(t, Query{Pattern: "é\nλλ"}, "éé\nλλ\n")
	if len(span) != 1 || span[0].StartRune != 1 || span[0].EndRune != 2 || span[0].End.Col != 4 {
		t.Fatalf("span %+v", span)
	}
}

func TestSmartCase(t *testing.T) {
	text := "Foo foo"
	if n := len(scan(t, Query{Pattern: "foo", SmartCase: true}, text)); n != 2 {
		t.Errorf("lowercase pattern matched %d", n)
	}
	if n := len(scan(t, Query{Pattern: "Foo", SmartCase: true}, text)); n != 1 {
		t.Errorf("uppercase pattern matched %d", n)
	}
	if n := len(scan(t, Query{Pattern: `\Sfoo`, Regex: true, SmartCase: true}, "xFoo xfoo")); n != 2 {
		t.Errorf("escape counted as uppercase: %d", n)
	}
}

func TestGroupsAndLongLines(t *testing.T) {
	got := scan(t, Query{Pattern: `(\w+)=(\d+)`, Regex: true}, "a=1 b=22")
	if len(got) != 2 || got[1].Groups[0] != "b" || got[1].Groups[1] != "22" || got[1].Matched != "b=22" {
		t.Fatalf("got %+v", got)
	}
	long := strings.Repeat("é", 1000) + "needle" + strings.Repeat("é", 1000)
	m := scan(t, Query{Pattern: "needle"}, long)[0]
	if m.Preview[m.PreviewStart:m.PreviewEnd] != "needle" || m.StartRune != 1000 {
		t.Fatalf("long %+v", m.StartRune)
	}
}
