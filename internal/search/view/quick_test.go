package searchview

import (
	"os"
	"slices"
	"testing"

	"github.com/ZerGo0/editor/internal/palette"
	"github.com/ZerGo0/editor/internal/search/everywhere"
	"github.com/ZerGo0/editor/internal/search/symbols"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

func (f *fixture) useProviders(env everywhere.Env) {
	env.Root, env.Files = f.host.root, f.idx
	f.view.env.Providers = everywhere.New(&env)
	f.view.sess = nil
}

func TestActionsScopeRunsAndShowsKeys(t *testing.T) {
	f := newFixture(t, nil)
	f.useProviders(everywhere.Env{Actions: func() []palette.Item {
		return []palette.Item{{ID: "save", Label: "Save File", Keys: "⌘S"}, {ID: "close", Label: "Close Tab", Keys: "⌘W"}}
	}})
	f.search(ScopeActions, "sav")
	if len(f.view.rows) != 1 || f.view.rows[0].res.ActionID != "save" {
		t.Fatalf("rows %v", f.labels())
	}
	if !f.tt.HasText("⌘S") {
		t.Errorf("key label missing: %q", f.tt.Texts())
	}
	f.focusField()
	f.tt.Key(0, ui.KeyEnter)
	f.settle()
	if !slices.Equal(f.runs, []string{"save"}) {
		t.Fatalf("ran %v", f.runs)
	}
	f.view.st.Query = ""
	f.search(ScopeActions, "")
	if len(f.view.rows) != 2 {
		t.Fatalf("all actions %v", f.labels())
	}
}

func TestSymbolsScopeAndGrouping(t *testing.T) {
	f := newFixture(t, map[string]string{"a.go": "package a\n\nfunc HelloWorld() {}\n\nfunc HelloMoon() {}\n", "b.go": "package b\n\nfunc HelloSun() {}\n"})
	syms := symbols.NewIndex()
	for _, name := range []string{"a.go", "b.go"} {
		text, err := readFile(f.path(name))
		if err != nil {
			t.Fatal(err)
		}
		syms.Update(f.path(name), "Go", text)
	}
	f.useProviders(everywhere.Env{Symbols: syms})
	f.search(ScopeSymbols, "hello")
	if len(f.view.rows) != 3 {
		t.Fatalf("symbols %v", f.labels())
	}
	f.view.st.SymbolsTree = true
	f.view.rowsDirty = true
	f.tt.Frame()
	if len(f.view.rows) != 5 || f.view.rows[0].kind != rowGroup {
		t.Fatalf("grouped %v", f.labels())
	}
	f.view.activateRow(1, OpenOptions{Focus: true})
	got := f.lastOpen()
	if got.loc.Start.Line != 2 || !got.loc.HasRange {
		t.Fatalf("symbol opened at %+v", got.loc)
	}
	f.view.toggleAll()
	f.view.rebuild()
	if len(f.view.rows) != 2 {
		t.Fatalf("collapsed groups %v", f.labels())
	}
}

func TestEditorSymbolsUseTheBufferText(t *testing.T) {
	f := newFixture(t, map[string]string{"a.go": "package a\n"})
	f.host.active = f.path("a.go")
	f.host.buffers[f.path("a.go")] = editor.NewBuffer("package a\n\nfunc Unsaved() {}\n\nfunc Other() {}\n")
	f.view.FocusEditorSymbols()
	f.settle()
	if f.view.scope != ScopeSymbols || !f.view.editorOnly || len(f.view.rows) != 2 {
		t.Fatalf("scope %v editorOnly %v rows %v", f.view.scope, f.view.editorOnly, f.labels())
	}
	f.view.st.Query = "unsav"
	f.view.searchSoon(0)
	f.settle()
	if len(f.view.rows) != 1 || f.view.rows[0].res.Label != "Unsaved" || f.view.rows[0].res.Line != 2 {
		t.Fatalf("filtered rows %v", f.labels())
	}
	f.view.activateRow(0, OpenOptions{Focus: true})
	if got := f.lastOpen(); got.loc.Path != f.path("a.go") || got.loc.Start.Line != 2 {
		t.Fatalf("open %+v", got)
	}
}

func TestAllScopeSectionsAndShowAll(t *testing.T) {
	tree := map[string]string{}
	for _, n := range []string{"alpha1.go", "alpha2.go", "alpha3.go", "alpha4.go", "alpha5.go", "alpha6.go", "alpha7.go"} {
		tree[n] = "package x // alpha\n"
	}
	f := newFixture(t, tree)
	f.useProviders(everywhere.Env{Actions: func() []palette.Item { return []palette.Item{{ID: "a", Label: "Alpha Action"}} }})
	f.search(ScopeAll, "alpha")
	var headers []string
	for _, r := range f.view.rows {
		if r.kind == rowHeader {
			headers = append(headers, r.title)
		}
	}
	if !slices.Equal(headers, []string{"Files", "Text", "Actions"}) {
		t.Fatalf("sections %v rows %v", headers, f.labels())
	}
	if !f.tt.HasText("Show all 7") {
		t.Errorf("show all link missing: %q", f.tt.Texts())
	}
	if err := f.tt.Click("Show all 7"); err != nil {
		t.Fatal(err)
	}
	f.settle()
	if f.view.scope != ScopeFiles || len(f.view.rows) != 7 {
		t.Fatalf("scope %v rows %d", f.view.scope, len(f.view.rows))
	}
}

func TestEmptyQueryShowsRecentFilesAndSearches(t *testing.T) {
	f := newFixture(t, map[string]string{"a.go": "x", "b.go": "y"})
	f.useProviders(everywhere.Env{Recent: func() []string { return []string{f.path("b.go"), f.path("a.go")} }})
	f.view.st.History = []string{"old query"}
	f.search(ScopeAll, "")
	want := []string{"", "b.go", "a.go", "", "old query"}
	if got := f.labels(); !slices.Equal(got, want) {
		t.Fatalf("rows %q want %q", got, want)
	}
	f.view.activateRow(4, OpenOptions{})
	if f.view.scope != ScopeText || f.view.st.Query != "old query" {
		t.Fatalf("recent search %v %q", f.view.scope, f.view.st.Query)
	}
}

func TestOpenFilesRankFirst(t *testing.T) {
	f := newFixture(t, map[string]string{"src/main_test.go": "x", "src/main.go": "y", "docs/main.md": "z"})
	f.host.buffers[f.path("src/main_test.go")] = editor.NewBuffer("x")
	f.useProviders(everywhere.Env{Recent: func() []string { return []string{f.path("docs/main.md")} }})
	f.search(ScopeFiles, "main")
	if len(f.view.rows) != 3 {
		t.Fatalf("rows %v", f.labels())
	}
	first := f.view.rows[0].res
	if first.Label != "main_test.go" || !first.Open {
		t.Fatalf("open editor not first: %+v", first)
	}
	byName := map[string]everywhere.Result{}
	for _, r := range f.view.rows {
		byName[r.res.Label+r.res.Detail] = r.res
	}
	if !byName["main.mddocs"].Recent || byName["main.gosrc"].Recent {
		t.Fatalf("recent flags %+v", byName)
	}
}

func readFile(p string) (string, error) {
	data, err := os.ReadFile(p)
	return string(data), err
}
