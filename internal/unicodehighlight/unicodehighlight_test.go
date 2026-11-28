package unicodehighlight

import (
	"slices"
	"testing"
)

func defaults() *Options {
	return &Options{Control: true, Invisible: true, Ambiguous: true}
}

func kinds(line string, o *Options) []Kind {
	var out []Kind
	for _, m := range Scan(line, 0, o, nil) {
		out = append(out, m.Kind)
	}
	return out
}

func TestScanMarksTrojanSourceCharacters(t *testing.T) {
	line := "/*\u202E } \u2066if (isAdmin)\u2069 \u2066 begin admins only */"
	ms := Scan(line, 10, defaults(), nil)
	if len(ms) != 4 {
		t.Fatalf("marks = %+v, want 4 bidi controls", ms)
	}
	for _, m := range ms {
		if m.Kind != Control || line[m.Start-10:m.End-10] != string(m.Rune) {
			t.Fatalf("mark %+v does not cover a control character", m)
		}
	}
	if got := ControlText(0x202E); got != "[U+202E]" {
		t.Fatalf("ControlText(U+202E) = %q", got)
	}
	if got := ControlText(0); got != "\u2400" {
		t.Fatalf("ControlText(NUL) = %q", got)
	}
}

func TestScanMarksInvisibleAndAmbiguousCharacters(t *testing.T) {
	if got := kinds("a\u200Bb x\u00A0y", defaults()); !slices.Equal(got, []Kind{Invisible, Invisible}) {
		t.Fatalf("invisible kinds = %v", got)
	}
	ms := Scan("if (isAdmin && \u0441ount)", 0, defaults(), nil)
	if len(ms) != 1 || ms[0].Kind != Ambiguous || ms[0].Confusable != 'c' {
		t.Fatalf("ambiguous marks = %+v", ms)
	}
	if got := kinds("\uFF58 = 1", defaults()); !slices.Equal(got, []Kind{Ambiguous}) {
		t.Fatalf("fullwidth kinds = %v", got)
	}
}

func TestScanLeavesWordsOfOneScriptAlone(t *testing.T) {
	for _, line := range []string{"привет мир", "// Ελληνικά", "日本語のテキスト", "café naïve", "👨\u200D👩\u200D👧 ❤\uFE0F", "plain ascii\ttab"} {
		if ms := Scan(line, 0, defaults(), nil); len(ms) != 0 {
			t.Errorf("%q: marks %+v, want none", line, ms)
		}
	}
}

func TestScanFollowsOptions(t *testing.T) {
	line := "x\u202Ey \u0441at"
	if ms := Scan(line, 0, &Options{}, nil); len(ms) != 0 {
		t.Fatalf("all off: %+v", ms)
	}
	if got := kinds(line, &Options{Control: true}); !slices.Equal(got, []Kind{Control}) {
		t.Fatalf("control only: %v", got)
	}
	if got := kinds(line, &Options{Invisible: true}); !slices.Equal(got, []Kind{Invisible}) {
		t.Fatalf("invisible only boxes the bidi control: %v", got)
	}
	o := defaults()
	o.Allowed = map[rune]bool{'с': true}
	if got := kinds(line, o); !slices.Equal(got, []Kind{Control}) {
		t.Fatalf("allowed character still marked: %v", got)
	}
	o = defaults()
	o.Locales = ForLanguages([]string{"en-US", "ru-RU"})
	if got := kinds(line, o); !slices.Equal(got, []Kind{Control}) {
		t.Fatalf("Cyrillic marked in a Russian locale: %v", got)
	}
	if got := kinds("café", &Options{NonBasicASCII: true}); !slices.Equal(got, []Kind{NonBasicASCII}) {
		t.Fatalf("non-basic ASCII: %v", got)
	}
}

func TestDescribeNamesTheCharacter(t *testing.T) {
	m := Scan("\u0441at", 0, defaults(), nil)[0]
	want := "The character U+0441 \"с\" could be confused with the ASCII character U+0063 \"c\", which is more common in source code"
	if got := Describe(m); got != want {
		t.Fatalf("Describe = %q", got)
	}
}

func BenchmarkScanASCII(b *testing.B) {
	line := "func (w *Workspace) decorate(c *ui.Context, tab *layout.Tab) { return nil } // comment"
	var dst []Mark
	for b.Loop() {
		dst = Scan(line, 0, defaults(), dst[:0])
	}
}
