package bracketcolor

import (
	"context"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui/editor"
)

type lines []string

func (l lines) LineCount() int    { return len(l) }
func (l lines) Line(i int) string { return l[i] }
func (l lines) LineStart(i int) int {
	n := 0
	for _, s := range l[:i] {
		n += len(s) + 1
	}
	return n
}

func depths(t *testing.T, src string, lang *editor.Language) string {
	t.Helper()
	text := lines(strings.Split(src, "\n"))
	got, err := Scan(context.Background(), text, lang)
	if err != nil {
		t.Fatal(err)
	}
	out := []byte(strings.Repeat(".", len(src)))
	for _, b := range got {
		if src[b.Offset] != b.Char {
			t.Fatalf("bracket at %d is %q, text has %q", b.Offset, b.Char, src[b.Offset])
		}
		switch {
		case b.Unexpected:
			out[b.Offset] = '!'
		default:
			out[b.Offset] = byte('0' + b.Depth)
		}
	}
	return string(out)
}

func TestScanNestsAndSkipsStringsAndComments(t *testing.T) {
	goLang := editor.LanguageNamed("Go")
	src := "f(a[b{c}]) // (x\ns := \"(\" + g()\n)"
	want := ".0.1.2.210...................00.!"
	if got := depths(t, src, goLang); got != want {
		t.Fatalf("depths\n got %s\nwant %s", got, want)
	}
}

func TestScanAcrossLinesAndMismatch(t *testing.T) {
	goLang := editor.LanguageNamed("Go")
	src := "{\n  (]\n}"
	want := "0...1!.0"
	if got := depths(t, src, goLang); got != want {
		t.Fatalf("depths\n got %s\nwant %s", got, want)
	}
}

func TestScanWithoutLanguage(t *testing.T) {
	if got, err := Scan(context.Background(), lines{"(a)"}, nil); got != nil || err != nil {
		t.Fatalf("Scan without language = %v, %v", got, err)
	}
	if Supports(nil) || !Supports(editor.LanguageNamed("Go")) {
		t.Fatal("Supports gave a wrong result")
	}
}

func TestScanStopsWhenCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Scan(ctx, lines{"()"}, editor.LanguageNamed("Go")); err == nil {
		t.Fatal("Scan ignored a canceled context")
	}
}
