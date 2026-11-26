package searchview

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui/editor"
)

func (f *fixture) read(rel string) string {
	f.t.Helper()
	data, err := os.ReadFile(f.path(rel))
	if err != nil {
		f.t.Fatal(err)
	}
	return string(data)
}

func (f *fixture) replaceSetup(query, with string) {
	f.view.st.ShowReplace = true
	f.view.st.Replace = with
	f.search(ScopeText, query)
}

func TestReplaceAllWritesDiskAndEditsBuffers(t *testing.T) {
	f := newFixture(t, map[string]string{"a.txt": "needle one\nneedle two\n", "b.txt": "a needle\n", "c.txt": "needle\n"})
	buf := editor.NewBuffer("open needle\n")
	f.host.buffers[f.path("c.txt")] = buf
	f.replaceSetup("needle", "pin")
	f.view.ReplaceAll()
	if len(f.confirms) != 1 || f.confirms[0] != "Replace 4 occurrences across 3 files?" {
		t.Fatalf("confirm %q", f.confirms)
	}
	f.settle()
	if f.read("a.txt") != "pin one\npin two\n" || f.read("b.txt") != "a pin\n" {
		t.Fatalf("disk a=%q b=%q", f.read("a.txt"), f.read("b.txt"))
	}
	if f.read("c.txt") != "needle\n" {
		t.Fatalf("open file was written to disk: %q", f.read("c.txt"))
	}
	if f.view.text.total != 0 {
		t.Fatalf("results did not refresh: total %d", f.view.text.total)
	}
	if buf.String() != "open pin\n" || !buf.Modified() {
		t.Fatalf("buffer %q modified %v", buf.String(), buf.Modified())
	}
	if !buf.Undo() || buf.String() != "open needle\n" {
		t.Fatalf("undo did not restore the buffer: %q", buf.String())
	}
	slices.Sort(f.before)
	if want := []string{f.path("a.txt"), f.path("b.txt")}; !slices.Equal(f.before, want) {
		t.Fatalf("BeforeWrite %v want %v", f.before, want)
	}
	if len(f.host.notices) == 0 || !strings.Contains(f.host.notices[len(f.host.notices)-1], "Replaced 4 occurrences in 3 files") {
		t.Fatalf("notices %v", f.host.notices)
	}
}

func TestReplaceAllDeclinedAndDismissed(t *testing.T) {
	f := newFixture(t, map[string]string{"a.txt": "needle needle\n"})
	f.replaceSetup("needle", "x")
	f.approve = false
	f.view.ReplaceAll()
	f.settle()
	if f.read("a.txt") != "needle needle\n" {
		t.Fatalf("declined replace changed the file: %q", f.read("a.txt"))
	}
	f.approve = true
	f.view.dismiss(f.view.rows[1])
	f.view.rebuild()
	f.view.ReplaceAll()
	f.settle()
	if f.read("a.txt") != "needle x\n" {
		t.Fatalf("dismissed match was replaced: %q", f.read("a.txt"))
	}
	if f.confirms[len(f.confirms)-1] != "Replace 1 occurrence across 1 file?" {
		t.Fatalf("singular confirm %q", f.confirms)
	}
}

func TestReplaceOneAndFile(t *testing.T) {
	f := newFixture(t, map[string]string{"a.txt": "needle\nneedle\n", "b.txt": "needle\n"})
	f.view.st.ListView = true
	f.replaceSetup("needle", "pin")
	f.view.replaceMatch(f.view.text.files[0], 1)
	f.settle()
	if f.read("a.txt") != "needle\npin\n" || f.read("b.txt") != "needle\n" {
		t.Fatalf("one: a=%q b=%q", f.read("a.txt"), f.read("b.txt"))
	}
	if f.view.text.total != 2 {
		t.Fatalf("results after one: %d", f.view.text.total)
	}
	f.view.replaceFile(f.view.text.files[0])
	f.settle()
	if f.read("a.txt") != "pin\npin\n" || f.read("b.txt") != "needle\n" {
		t.Fatalf("file: a=%q b=%q", f.read("a.txt"), f.read("b.txt"))
	}
	if len(f.confirms) != 0 {
		t.Fatalf("single replaces must not ask: %v", f.confirms)
	}
}

func TestReplaceRegexGroupsAndPreserveCase(t *testing.T) {
	f := newFixture(t, map[string]string{"a.txt": "key=1\nkey=22\nHello hello\n"})
	f.view.st.Regex = true
	f.replaceSetup(`(\w+)=(\d+)`, "$2:$1 [$&]")
	f.view.ReplaceAll()
	f.settle()
	if got := f.read("a.txt"); got != "1:key [key=1]\n22:key [key=22]\nHello hello\n" {
		t.Fatalf("groups: %q", got)
	}
	g := newFixture(t, map[string]string{"a.txt": "Hello hello HELLO\n"})
	g.view.st.PreserveCase = true
	g.replaceSetup("hello", "world")
	g.view.ReplaceAll()
	g.settle()
	if got := g.read("a.txt"); got != "World world WORLD\n" {
		t.Fatalf("preserve case: %q", got)
	}
}

func TestReplaceMultilineMatch(t *testing.T) {
	f := newFixture(t, map[string]string{"a.txt": "x\r\nfoo\r\nbar\r\ny\r\n"})
	f.replaceSetup("foo\nbar", "joined")
	f.view.ReplaceAll()
	f.settle()
	if got := f.read("a.txt"); got != "x\r\njoined\r\ny\r\n" {
		t.Fatalf("multi-line replace: %q", got)
	}
}

func TestReplacePreviewAndDisabledStates(t *testing.T) {
	f := newFixture(t, map[string]string{"a.txt": "needle\n"})
	f.replaceSetup("needle", "pin")
	if !f.tt.HasText("pin") {
		t.Errorf("replacement preview missing: %q", f.tt.Texts())
	}
	if a, ok := f.tt.Access("Replace All"); !ok || a.Disabled {
		t.Errorf("Replace All: %+v %v", a, ok)
	}
	g := newFixture(t, map[string]string{"a.txt": "x"})
	g.replaceSetup("needle", "pin")
	if a, ok := g.tt.Access("Replace All"); !ok || !a.Disabled {
		t.Errorf("Replace All without results should be disabled: %+v %v", a, ok)
	}
	g.view.ReplaceAll()
	if len(g.confirms) != 0 {
		t.Error("asked to replace nothing")
	}
}

func TestReplaceFieldMultilineLabels(t *testing.T) {
	f := newFixture(t, map[string]string{"a.txt": "needle\n"})
	f.replaceSetup("needle", "")
	if _, ok := f.tt.Access("Replace"); !ok {
		t.Fatal("replace field has no label")
	}
}
