package searchview

import (
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/ZerGo0/editor/internal/search/everywhere"
	"github.com/ZerGo0/editor/internal/search/textsearch"
	"github.com/egoist/mygo/ui"
)

const largeFiles, largeMatchesPerFile = 2000, 10

func TestRowAccessibility(t *testing.T) {
	f := newFixture(t, map[string]string{"sub/a.go": "one\nneedle two\n"})
	f.search(ScopeText, "needle")
	for _, name := range []string{"sub/a.go", "line 2: needle two"} {
		a, ok := f.tt.Access(name)
		if !ok {
			t.Errorf("no element named %q: %q", name, f.tt.Texts())
			continue
		}
		t.Logf("%q role %v", name, a.Role)
	}
	for _, a := range f.tt.AccessAll() {
		if a.Role == ui.RoleButton && a.Name == "" {
			t.Errorf("unnamed button at %v", a.Bounds)
		}
	}
}

func TestFieldGrowsWithLines(t *testing.T) {
	f := newFixture(t, map[string]string{"a.txt": "x"})
	f.view.setScope(ScopeText)
	f.focusField()
	before, ok := f.tt.Find("Search")
	if !ok {
		t.Fatal("no search field")
	}
	for range 3 {
		f.tt.Type("line")
		f.tt.Key(ui.Shift, ui.KeyEnter)
	}
	after, _ := f.tt.Find("Search")
	if after.H <= before.H {
		t.Fatalf("field did not grow: %v -> %v", before.H, after.H)
	}
}

func TestLargeResultSetStaysFast(t *testing.T) {
	f := newFixture(t, map[string]string{"a.txt": "x"})
	f.view.setScope(ScopeText)
	f.view.st.Query = "needle"
	f.view.text.matcher, _ = textsearch.Compile(textsearch.Query{Pattern: "needle"})
	for i := range largeFiles {
		name := "pkg" + strconv.Itoa(i%40) + "/file" + strconv.Itoa(i) + ".go"
		var ms []textsearch.Match
		for j := range largeMatchesPerFile {
			ms = append(ms, textsearch.Match{Start: textsearch.Pos{Line: j}, End: textsearch.Pos{Line: j, Col: 6}, Preview: "needle in line", PreviewEnd: 6, Matched: "needle"})
		}
		f.view.text.files = append(f.view.text.files, newFileNode(f.path(name), name, ms))
	}
	f.view.recount()
	f.view.sortFiles()
	f.view.rowsDirty = true
	start := time.Now()
	f.tt.Frame()
	first := time.Since(start)
	if f.view.text.total != largeFiles*largeMatchesPerFile || len(f.view.rows) < largeFiles*largeMatchesPerFile {
		t.Fatalf("rows %d total %d", len(f.view.rows), f.view.text.total)
	}
	start = time.Now()
	for range 20 {
		f.tt.Frame()
	}
	steady := time.Since(start) / 20
	t.Logf("first frame %v, steady frame %v for %d rows", first, steady, len(f.view.rows))
	if steady > 40*time.Millisecond {
		t.Errorf("steady frame too slow: %v", steady)
	}
}

func TestSelectionStaysOnItemWhenResultsChange(t *testing.T) {
	f := newFixture(t, map[string]string{"a.go": "x", "b.go": "x", "c.go": "x"})
	f.search(ScopeFiles, ".go")
	f.view.userMoved = true
	f.view.choose(1)
	key := f.view.selectedKey
	f.view.sections[ScopeFiles].items = append([]everywhere.Result{{Kind: everywhere.KindFile, Label: "0.go", Path: f.path("0.go")}}, f.view.sections[ScopeFiles].items...)
	f.view.rowsDirty = true
	f.tt.Frame()
	if r, ok := f.view.rowAt(f.view.selected); !ok || r.key != key {
		t.Fatalf("selection moved off its item: %d %v", f.view.selected, f.labels())
	}
}

func TestScopeMenuListsTextAndGit(t *testing.T) {
	f := newFixture(t, map[string]string{"a.go": "needle\nneedle\n"})
	f.search(ScopeText, "needle")
	if !f.tt.HasText("2 results in 1 file") {
		t.Errorf("count missing from the message line: %q", f.tt.Texts())
	}
	if f.view.scope != ScopeText {
		t.Fatalf("the Search view opens in Text: %v", f.view.scope)
	}
	f.view.setScope(ScopeActions)
	if got := f.view.scopeChoicesNow(); !slices.Equal(got, []Scope{ScopeText, ScopeGit, ScopeActions}) {
		t.Fatalf("scope choices %v", got)
	}
}
