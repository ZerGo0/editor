package searchview

import (
	"context"
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/ZerGo0/editor/internal/search/everywhere"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

func (f *fixture) focusField() {
	f.t.Helper()
	f.view.field.Focus()
	f.tt.Frame()
	if !f.tt.Focused("Search") {
		f.t.Fatalf("search field has no focus")
	}
}

func (f *fixture) lastOpen() opened {
	f.t.Helper()
	if len(f.opens) == 0 {
		f.t.Fatal("nothing was opened")
	}
	return f.opens[len(f.opens)-1]
}

func TestTextSearchRowsAndLabels(t *testing.T) {
	f := newFixture(t, map[string]string{"a.go": "one\nneedle two\n", "sub/b.txt": "needle\n"})
	f.search(ScopeText, "needle")
	if f.view.text.total != 2 || len(f.view.text.files) != 2 {
		t.Fatalf("total %d rows %v", f.view.text.total, f.labels())
	}
	want := []string{"sub", "sub/b.txt", "line 1: needle", "a.go", "line 2: needle two"}
	if got := f.labels(); !slices.Equal(got, want) {
		t.Fatalf("rows %q want %q", got, want)
	}
	if !f.tt.HasText("2 results in 2 files") {
		t.Errorf("summary missing: %q", f.tt.Texts())
	}
	for _, name := range []string{"Search", "Search results", "Match Case", "Match Whole Word", "Use Regular Expression", "Toggle Replace", "Refresh"} {
		if _, ok := f.tt.Access(name); !ok {
			t.Errorf("no accessible element named %q", name)
		}
	}
	if a, ok := f.tt.AccessByRole(ui.RoleTextField, "Search"); !ok || a.Name != "Search" {
		t.Errorf("search field: %+v %v", a, ok)
	}
	if _, ok := f.tt.AccessByRole(ui.RoleTab, "Files"); ok {
		t.Error("scope chips are gone: Files is a Quick Open prefix")
	}
	if _, ok := f.tt.Access("Search Scope"); !ok {
		t.Error("no scope menu in the view actions")
	}
}

func TestMultilineSearchTyping(t *testing.T) {
	f := newFixture(t, map[string]string{"a.txt": "alpha\nbeta\ngamma\n"})
	f.view.setScope(ScopeText)
	f.focusField()
	f.tt.Type("alpha")
	f.tt.Key(ui.Shift, ui.KeyEnter)
	f.tt.Type("beta")
	if f.view.st.Query != "alpha\nbeta" {
		t.Fatalf("query %q", f.view.st.Query)
	}
	f.settle()
	if f.view.text.total != 1 {
		t.Fatalf("multi-line match total %d", f.view.text.total)
	}
	m := f.view.text.files[0].matches[0]
	if !m.Multiline() || m.Start.Line != 0 || m.End.Line != 1 {
		t.Fatalf("match %+v", m)
	}
	f.tt.Key(ui.Cmd, ui.KeyEnter)
	f.tt.Type("x")
	if f.view.st.Query != "alpha\nbeta\nx" {
		t.Fatalf("cmd+enter query %q", f.view.st.Query)
	}
}

func TestRegexError(t *testing.T) {
	f := newFixture(t, map[string]string{"a.txt": "x"})
	f.view.st.Regex = true
	f.search(ScopeText, "(")
	if f.view.text.err == "" {
		t.Fatal("no pattern error")
	}
	if !f.tt.HasText(f.view.text.err) {
		t.Errorf("error not shown inline: %q", f.tt.Texts())
	}
	if len(f.view.rows) != 0 {
		t.Errorf("rows for an invalid pattern: %v", f.labels())
	}
}

func TestPrefixRoutingAndScopeKeys(t *testing.T) {
	f := newFixture(t, map[string]string{"a.go": "x"})
	f.view.setScope(ScopeAll)
	f.focusField()
	f.tt.Type(">")
	f.tt.Frame()
	if f.view.scope != ScopeActions || f.view.st.Query != "" {
		t.Fatalf("scope %v query %q", f.view.scope, f.view.st.Query)
	}
	f.tt.Key(0, ui.KeyTab)
	f.tt.Key(ui.Cmd, ui.Key4)
	if f.view.scope != ScopeActions {
		t.Fatalf("tab or cmd+4 changed the scope to %v", f.view.scope)
	}
	f.view.switchScope(ScopeAll)
	f.focusField()
	f.tt.Type("#foo")
	if f.view.scope != ScopeSymbols || f.view.st.Query != "foo" || f.view.editorOnly {
		t.Fatalf("# routed to %v %q", f.view.scope, f.view.st.Query)
	}
	if got := routePrefix("@x"); got.scope != ScopeSymbols || !got.inEditor || got.query != "x" {
		t.Errorf("@ route %+v", got)
	}
	if got := routePrefix("/x"); got.scope != ScopeText || got.query != "x" {
		t.Errorf("/ route %+v", got)
	}
	if got := routePrefix(":12"); got.scope != ScopeFiles || got.query != ":12" {
		t.Errorf(": route %+v", got)
	}
}

func TestFilesScopeOpensAndGoToLine(t *testing.T) {
	f := newFixture(t, map[string]string{"main.go": "x", "util/helper.go": "y"})
	f.search(ScopeFiles, "helper")
	if len(f.view.rows) != 1 || f.view.rows[0].res.Label != "helper.go" {
		t.Fatalf("rows %v", f.labels())
	}
	if f.view.selected != 0 {
		t.Fatalf("top result not selected: %d", f.view.selected)
	}
	f.focusField()
	f.tt.Key(0, ui.KeyEnter)
	f.settle()
	got := f.lastOpen()
	if got.loc.Path != f.path("util/helper.go") || !got.opts.Focus || got.opts.Preview {
		t.Fatalf("open %+v", got)
	}
	f.search(ScopeFiles, "main:42:3")
	f.focusField()
	f.tt.Key(ui.Cmd, ui.KeyEnter)
	f.settle()
	got = f.lastOpen()
	if got.loc.Path != f.path("main.go") || got.loc.Start.Line != 41 || got.loc.Start.Col != 2 || !got.opts.Side {
		t.Fatalf("path:line open %+v", got)
	}
	f.host.active = f.path("main.go")
	f.search(ScopeFiles, ":7")
	if len(f.view.rows) == 0 || f.view.rows[0].kind != rowGoto || f.view.rows[0].line != 6 {
		t.Fatalf("go to line row %v", f.labels())
	}
}

func TestKeyboardMovesBetweenFieldAndList(t *testing.T) {
	f := newFixture(t, map[string]string{"a.go": "needle\n", "b.go": "needle\n"})
	f.search(ScopeText, "needle")
	f.focusField()
	f.tt.Key(0, ui.KeyDown)
	f.tt.Frame()
	if f.tt.Focused("Search") {
		t.Fatalf("focus did not leave the field")
	}
	if f.view.selected < 0 {
		t.Fatalf("no row selected after Down")
	}
	f.tt.Key(0, ui.KeyDown)
	f.tt.Frame()
	if len(f.opens) == 0 || !f.lastOpen().opts.Preview || f.lastOpen().opts.Focus {
		t.Fatalf("arrow movement did not preview: %+v", f.opens)
	}
	f.view.selected = f.view.firstSelectable()
	f.tt.Frame()
	f.tt.Key(0, ui.KeyUp)
	f.tt.Frame()
	if !f.tt.Focused("Search") {
		t.Fatalf("Up from the first row did not return to the field")
	}
}

func TestEscapeClearsThenFocusesEditor(t *testing.T) {
	f := newFixture(t, map[string]string{"a.go": "needle\n"})
	focused := 0
	f.view.env.FocusEditor = func() { focused++ }
	f.search(ScopeText, "needle")
	f.focusField()
	f.tt.Key(0, ui.KeyEscape)
	if f.view.st.Query != "" || focused != 0 {
		t.Fatalf("first escape: query %q focused %d", f.view.st.Query, focused)
	}
	f.settle()
	f.focusField()
	f.tt.Key(0, ui.KeyEscape)
	if focused != 1 {
		t.Fatalf("second escape focused editor %d times", focused)
	}
}

func TestHistoryWalk(t *testing.T) {
	f := newFixture(t, map[string]string{"a.go": "one two three\n"})
	for _, q := range []string{"one", "two", "three"} {
		f.search(ScopeText, q)
	}
	if !slices.Equal(f.view.st.History, []string{"three", "two", "one"}) {
		t.Fatalf("history %q", f.view.st.History)
	}
	f.view.st.Query = ""
	f.search(ScopeText, "")
	f.view.st.Query = "draft"
	f.focusField()
	f.tt.Key(0, ui.KeyUp)
	f.tt.Key(0, ui.KeyUp)
	if f.view.st.Query != "two" {
		t.Fatalf("after two Up: %q", f.view.st.Query)
	}
	f.tt.Key(0, ui.KeyDown)
	f.tt.Key(0, ui.KeyDown)
	if f.view.st.Query != "draft" {
		t.Fatalf("after Down: %q", f.view.st.Query)
	}
}

func TestSnapshotRestore(t *testing.T) {
	f := newFixture(t, map[string]string{"a.go": "needle\n"})
	f.view.st.MatchCase, f.view.st.Include, f.view.st.ShowReplace, f.view.st.Replace = true, "*.go", true, "pin"
	f.search(ScopeText, "needle")
	data, err := json.Marshal(f.view.Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		t.Fatal(err)
	}
	g := newFixture(t, map[string]string{"a.go": "needle\n"})
	g.view.Restore(st)
	g.settle()
	if g.view.scope != ScopeText || g.view.st.Query != "needle" || !g.view.st.MatchCase || g.view.st.Include != "*.go" || g.view.text.total != 1 {
		t.Fatalf("restored %+v total %d", g.view.st, g.view.text.total)
	}
	if !slices.Equal(g.view.st.History, []string{"needle"}) {
		t.Fatalf("history %q", g.view.st.History)
	}
}

func TestIncludeExcludeFilters(t *testing.T) {
	f := newFixture(t, map[string]string{"a.go": "needle", "b.ts": "needle", "node_modules/c.go": "needle", "src/d.go": "needle"})
	f.view.st.Include = "*.go"
	f.view.st.Exclude = "src"
	f.search(ScopeText, "needle")
	if got := f.labelsOfFiles(); !slices.Equal(got, []string{"a.go", "node_modules/c.go"}) {
		t.Fatalf("files %v", got)
	}
}

func (f *fixture) labelsOfFiles() []string {
	var out []string
	for _, n := range f.view.text.files {
		out = append(out, n.rel)
	}
	return out
}

func TestOpenEditorsOnlyUsesBufferText(t *testing.T) {
	f := newFixture(t, map[string]string{"a.go": "disk text", "b.go": "needle on disk"})
	f.host.buffers[f.path("a.go")] = editor.NewBuffer("needle in buffer")
	f.view.st.OnlyOpen = true
	f.search(ScopeText, "needle")
	if got := f.labelsOfFiles(); !slices.Equal(got, []string{"a.go"}) {
		t.Fatalf("files %v", got)
	}
	f.view.st.OnlyOpen = false
	f.search(ScopeText, "needle in buffer")
	if f.view.text.total != 1 {
		t.Fatalf("buffer text not searched: %d", f.view.text.total)
	}
}

func TestResultLimitNotice(t *testing.T) {
	f := newFixture(t, map[string]string{"a.go": "needle\n"})
	f.search(ScopeText, "needle")
	f.view.text.truncated = true
	f.tt.Frame()
	if !f.tt.HasText(truncatedNotice) {
		t.Errorf("limit notice missing")
	}
}

func TestSeedSelectsAndSearches(t *testing.T) {
	f := newFixture(t, map[string]string{"a.go": "seedword\n"})
	f.view.Focus(ScopeText, "seedword")
	f.settle()
	if f.view.text.total != 1 || !f.tt.Focused("Search") {
		t.Fatalf("seed: total %d focused %v", f.view.text.total, f.tt.Focused("Search"))
	}
	f.view.Focus(ScopeFiles, "")
	f.settle()
	if f.view.scope != ScopeFiles || f.view.st.Query != "seedword" {
		t.Fatalf("empty seed should keep the query: %v %q", f.view.scope, f.view.st.Query)
	}
}

func TestActionsScope(t *testing.T) {
	f := newFixture(t, nil)
	prov := everywhere.New(&everywhere.Env{Root: f.host.root, Actions: nil})
	f.view.env.Providers = prov
	f.view.sess = nil
	f.search(ScopeActions, "x")
	if len(f.view.rows) != 1 || f.view.rows[0].kind != rowMessage {
		t.Fatalf("empty actions should hint: %v", f.labels())
	}
}

func TestNextPrevWalkAllMatches(t *testing.T) {
	f := newFixture(t, map[string]string{"a.go": "needle\nneedle\n", "b.go": "needle\n"})
	f.search(ScopeText, "needle")
	f.view.Next()
	f.view.Next()
	f.view.Next()
	if len(f.opens) != 3 || f.opens[2].loc.Path != f.path("b.go") {
		t.Fatalf("opens %+v", f.opens)
	}
	f.view.Next()
	if f.lastOpen().loc.Path != f.path("a.go") || f.lastOpen().loc.Start.Line != 0 {
		t.Fatalf("Next did not wrap: %+v", f.lastOpen())
	}
	f.view.Prev()
	if f.lastOpen().loc.Path != f.path("b.go") {
		t.Fatalf("Prev did not wrap back: %+v", f.lastOpen())
	}
	if !f.lastOpen().opts.Preview || !f.lastOpen().opts.Focus {
		t.Fatalf("options %+v", f.lastOpen().opts)
	}
}

func TestDismissAndCollapse(t *testing.T) {
	f := newFixture(t, map[string]string{"a.go": "needle\nneedle\n", "b.go": "needle\n"})
	f.view.st.ListView = true
	f.search(ScopeText, "needle")
	f.view.dismiss(f.view.rows[1])
	f.view.rebuild()
	if f.view.text.total != 2 {
		t.Fatalf("total after dismiss %d", f.view.text.total)
	}
	f.view.dismiss(f.view.rows[0])
	f.view.rebuild()
	if len(f.view.text.files) != 1 {
		t.Fatalf("file not dismissed: %v", f.labelsOfFiles())
	}
	f.view.toggleAll()
	f.view.rebuild()
	if len(f.view.rows) != 1 || f.view.anyExpanded() {
		t.Fatalf("collapse all: %v", f.labels())
	}
	f.view.toggleAll()
	f.view.rebuild()
	if len(f.view.rows) != 2 {
		t.Fatalf("expand all: %v", f.labels())
	}
}

func TestChangedRefreshesFiles(t *testing.T) {
	f := newFixture(t, map[string]string{"a.go": "needle\n", "b.go": "needle\n"})
	f.search(ScopeText, "needle")
	if err := os.WriteFile(f.path("a.go"), []byte("nothing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f.view.Changed([]string{f.path("a.go")})
	f.settle()
	if got := f.labelsOfFiles(); !slices.Equal(got, []string{"b.go"}) || f.view.text.total != 1 {
		t.Fatalf("files %v total %d", got, f.view.text.total)
	}
	if err := os.WriteFile(f.path("a.go"), []byte("needle\nneedle\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f.view.Changed([]string{f.path("a.go")})
	f.settle()
	if f.view.text.total != 3 {
		t.Fatalf("total %d", f.view.text.total)
	}
	g := newFixture(t, map[string]string{"a.go": "x"})
	g.view.Changed([]string{g.path("a.go")})
	if g.view.text.matcher != nil || len(g.host.queue) != 0 {
		t.Fatal("Changed searched without an active search")
	}
}

func TestChangedWithDirectoryHints(t *testing.T) {
	f := newFixture(t, map[string]string{"src/a.go": "needle\n", "b.go": "x\n"})
	f.search(ScopeText, "needle")
	if err := os.WriteFile(f.path("src/a.go"), []byte("nothing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.path("src/c.go"), []byte("needle\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f.view.Changed([]string{f.path("src")})
	f.settle()
	if got := f.labelsOfFiles(); !slices.Equal(got, []string{"src/c.go"}) || f.view.text.total != 1 {
		t.Fatalf("files %v total %d", got, f.view.text.total)
	}
}

func TestIndexChangedRefreshesQuickScopes(t *testing.T) {
	f := newFixture(t, map[string]string{"a.go": "x"})
	f.search(ScopeFiles, "late")
	if got := f.labels(); slices.Contains(got, "late.go") {
		t.Fatalf("labels before the file exists: %v", got)
	}
	if err := os.WriteFile(f.path("late.go"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := f.idx.Refresh(context.Background(), func(string) bool { return false }); err != nil {
		t.Fatal(err)
	}
	f.view.IndexChanged()
	f.settle()
	if got := f.labels(); !slices.Contains(got, "late.go") {
		t.Fatalf("labels after the index changed: %v", got)
	}
}

func TestRecentSearchRows(t *testing.T) {
	f := newFixture(t, map[string]string{"a.go": "x"})
	f.view.st.History = []string{"one", "two"}
	f.search(ScopeText, "")
	if got := f.labels(); !slices.Equal(got, []string{"", "one", "two"}) {
		t.Fatalf("history rows %q", got)
	}
	f.view.activateRow(1, OpenOptions{})
	if f.view.scope != ScopeText || f.view.st.Query != "one" {
		t.Fatalf("recent search not applied: %v %q", f.view.scope, f.view.st.Query)
	}
	_ = strings.TrimSpace
}

func TestFlagTogglesRunTheSearchAgain(t *testing.T) {
	f := newFixture(t, map[string]string{"a.txt": "Needle needle\n"})
	f.search(ScopeText, "needle")
	before := f.view.text.total
	if err := f.tt.Click("Match Case"); err != nil {
		t.Fatal(err)
	}
	if !f.view.st.MatchCase {
		t.Fatal("Match Case did not toggle")
	}
	f.settle()
	if f.view.text.total >= before {
		t.Errorf("matches %d -> %d: the toggle did not search again", before, f.view.text.total)
	}
}
