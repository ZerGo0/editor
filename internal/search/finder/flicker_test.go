package finder

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/palette"
	"github.com/ZerGo0/editor/internal/search/textsearch"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	itemFiles       = 12
	noResultsPrefix = "No matching results"
)

type shot struct {
	rows     int
	selected int
	labels   []string
	preview  *editor.State
}

func (f *fixture) watch() *[]shot {
	f.key(0, ui.KeyEnd)
	var shots []shot
	f.onFrame = func() {
		d := f.dialog
		shots = append(shots, shot{rows: len(d.rows), selected: d.results.Selected, labels: f.labels(), preview: d.preview.st})
	}
	return &shots
}

func (f *fixture) typeEach(text string) {
	f.t.Helper()
	for _, r := range text {
		f.tt.Type(string(r))
		f.settle()
	}
}

func itemProject() map[string]string {
	project := map[string]string{"main.go": "package main\n"}
	for i := range itemFiles {
		project["item"+strconv.Itoa(i)+".txt"] = "item needle alpha " + strconv.Itoa(i) + "\nitem needle alpine\n"
	}
	return project
}

func (f *fixture) addItemActions() {
	for i := range itemFiles {
		f.commands = append(f.commands, palette.Item{ID: "item" + strconv.Itoa(i), Label: "Item Action " + strconv.Itoa(i)})
	}
}

func requireRows(t *testing.T, shots []shot, what string) {
	t.Helper()
	if len(shots) == 0 {
		t.Fatalf("%s: no frames were recorded", what)
	}
	for i, s := range shots {
		if s.rows == 0 {
			t.Fatalf("%s: frame %d of %d shows no rows between two queries that both have results", what, i, len(shots))
		}
	}
}

func TestTabsKeepTheirRowsWhileTheQueryChanges(t *testing.T) {
	for _, tc := range []struct {
		tab   Tab
		seed  string
		typed string
	}{
		{TabFiles, "item", "1."},
		{TabActions, "item", " a"},
		{TabAll, "item", "1."},
		{TabText, "needle", " al"},
	} {
		f := newFixture(t, itemProject())
		f.addItemActions()
		f.open(tc.tab, tc.seed)
		if len(f.dialog.rows) == 0 {
			t.Fatalf("%v: no rows for the first query", tc.tab)
		}
		shots := f.watch()
		f.typeEach(tc.typed)
		requireRows(t, *shots, tc.tab.String())
	}
}

func TestSwitchingTabsKeepsRowsUntilTheNextTabIsReady(t *testing.T) {
	f := newFixture(t, itemProject())
	f.addItemActions()
	f.open(TabAll, "item")
	shots := f.watch()
	f.key(0, ui.KeyTab)
	f.key(0, ui.KeyTab)
	f.key(0, ui.KeyTab)
	requireRows(t, *shots, "tab cycle")
}

func TestEmptyStateWaitsForTheSpinnerDelay(t *testing.T) {
	f := newFixture(t, itemProject())
	f.open(TabText, "")
	f.tt.Type("needle")
	f.tt.Frame()
	if !f.dialog.busy() {
		t.Fatal("the search is not running")
	}
	for _, text := range []string{searchingText, noResultsPrefix} {
		if f.tt.HasText(text) {
			t.Fatalf("%q shows before the spinner delay", text)
		}
	}
	f.tt.Advance(token.SpinnerDelay)
	f.tt.Frame()
	if !f.tt.HasText(searchingText) {
		t.Fatal("a slow search shows no loading state")
	}
	f.settle()
	if f.tt.HasText(searchingText) || len(f.dialog.rows) == 0 {
		t.Fatal("the loading state stays after the results arrive")
	}
}

func TestNoResultsMessageStaysWhileTheNextSearchRuns(t *testing.T) {
	f := newFixture(t, itemProject())
	f.open(TabText, "")
	f.type_("qqq")
	if !f.tt.HasText(noResultsPrefix) {
		t.Fatal("no empty state for a query without matches")
	}
	f.tt.Type("q")
	f.tt.Frame()
	if f.tt.HasText(searchingText) || !f.tt.HasText(noResultsPrefix) {
		t.Fatal("the empty state changed while the next search had not shown progress yet")
	}
}

func TestSearchProgressDoesNotMoveTheResults(t *testing.T) {
	f := newFixture(t, itemProject())
	f.open(TabText, "needle")
	top := func() float32 {
		a, ok := f.tt.AccessByRole(ui.RoleList, resultsLabel)
		if !ok {
			t.Fatal("no results list")
		}
		return a.Bounds.Y
	}
	before := top()
	f.key(0, ui.KeyEnd)
	f.tt.Type(" ")
	f.tt.Frame()
	if got := top(); got != before {
		t.Fatalf("results moved from y %.1f to %.1f when a search started", before, got)
	}
	f.tt.Advance(token.SpinnerDelay)
	f.tt.Frame()
	if _, ok := f.tt.AccessByRole(ui.RoleProgress, ""); !ok {
		t.Fatal("a slow search shows no progress bar")
	}
	if got := top(); got != before {
		t.Fatalf("results moved from y %.1f to %.1f when the progress bar showed", before, got)
	}
}

func TestPreviewKeepsItsEditorWhileTheQueryChanges(t *testing.T) {
	for _, tc := range []struct {
		tab   Tab
		seed  string
		typed string
	}{{TabFiles, "item", "1."}, {TabText, "needle", " al"}} {
		f := newFixture(t, itemProject())
		f.dialog.st.Preview = true
		f.open(tc.tab, tc.seed)
		if f.dialog.preview.st == nil {
			t.Fatalf("%v: no preview of the first result", tc.tab)
		}
		shots := f.watch()
		f.typeEach(tc.typed)
		for i, s := range *shots {
			if s.preview == nil {
				t.Fatalf("%v: frame %d has no preview editor between two queries that both have results", tc.tab, i)
			}
		}
	}
}

func TestPreviewKeepsItsStateForTheSameFileWhileTyping(t *testing.T) {
	f := newFixture(t, map[string]string{"a.txt": "needle alpha\nneedle alpine\n"})
	f.dialog.st.Preview = true
	f.open(TabText, "needle")
	st := f.dialog.preview.st
	shots := f.watch()
	f.typeEach(" al")
	for i, s := range *shots {
		if s.preview != st {
			t.Fatalf("frame %d rebuilt the preview editor for the same file", i)
		}
	}
}

func TestClickAcceptsOnlyWithoutAPreviewAndDoubleClickAcceptsWithOne(t *testing.T) {
	setup := func(tab Tab, preview bool) *fixture {
		f := newFixture(t, demoProject)
		f.dialog.st.Preview = preview
		f.recents = []string{f.path("main.go"), f.path("docs/readme.md")}
		f.open(tab, "")
		return f
	}
	f := setup(TabFiles, true)
	f.click("readme.md")
	if len(f.opened) != 0 || !f.dialog.IsOpen() {
		t.Fatalf("a single click accepted a row of a tab with a preview: %+v", f.opened)
	}
	if e, ok := f.dialog.current(); !ok || e.label != "readme.md" || !f.dialog.moved {
		t.Fatalf("a single click did not select the row: %+v", e)
	}
	if st := f.dialog.preview.st; st == nil || !strings.Contains(st.Buffer.String(), "# Title") {
		t.Fatal("the preview does not follow the clicked row")
	}
	f.click("readme.md")
	if len(f.opened) != 1 || f.opened[0].Path != f.path("docs/readme.md") || f.dialog.IsOpen() {
		t.Fatalf("a double click did not accept the row: %+v", f.opened)
	}

	f = setup(TabFiles, true)
	f.tt.ClickWith(ui.Shift, "readme.md")
	f.tt.ClickWith(ui.Shift, "readme.md")
	f.settle()
	if len(f.opened) != 0 || !f.dialog.IsOpen() {
		t.Fatalf("a modified double click accepted a row: %+v", f.opened)
	}

	f = setup(TabFiles, false)
	f.click("readme.md")
	if len(f.opened) != 1 || f.dialog.IsOpen() {
		t.Fatalf("a single click without a preview did not accept: %+v", f.opened)
	}

	f = setup(TabFiles, true)
	f.dialog.st.Query = prefixHelp
	f.dialog.queryChanged()
	f.settle()
	f.click("Go to Symbol in Editor")
	if f.dialog.Query() != prefixEditorSymbols || !f.dialog.IsOpen() {
		t.Fatalf("a single click on a help row with a preview did not apply it: query %q", f.dialog.Query())
	}

	f = setup(TabActions, true)
	f.click("File: Save")
	if !slices.Equal(f.ran, []string{"save"}) {
		t.Fatalf("a single click on a tab without a preview did not accept: %v", f.ran)
	}
}

func textFile(name string, lines ...int) textsearch.FileResult {
	fr := textsearch.FileResult{Path: name}
	for _, line := range lines {
		pos := textsearch.Pos{Line: line}
		fr.Matches = append(fr.Matches, textsearch.Match{Start: pos, End: pos, Preview: "needle " + strconv.Itoa(line)})
	}
	return fr
}

func (f *fixture) stream(frs ...textsearch.FileResult) {
	f.t.Helper()
	r := &f.dialog.text
	for _, fr := range frs {
		r.enqueue(r.currentGen(), fr)
	}
	f.waitQueue()
	f.settle()
}

func TestStreamedBatchesAppendWithoutReorderingOrMovingTheSelection(t *testing.T) {
	f := newFixture(t, map[string]string{"zzz.txt": "nothing\n"})
	f.open(TabText, "needle")
	if len(f.dialog.rows) != 0 {
		t.Fatalf("expected no matches yet, got %v", f.labels())
	}
	shots := f.watch()
	f.stream(textFile(f.path("m.txt"), 1, 2))
	f.key(0, ui.KeyDown)
	selected, ok := f.dialog.current()
	if !ok {
		t.Fatal("no selected match")
	}
	before := slices.Clone(f.labels())
	f.stream(textFile(f.path("a.txt"), 7), textFile(f.path("z.txt"), 9))
	after := f.labels()
	if len(after) != len(before)+2 || !slices.Equal(after[:len(before)], before) {
		t.Fatalf("a batch reordered the rows: before %v after %v", before, after)
	}
	if now, _ := f.dialog.current(); now.path != selected.path || now.match != selected.match {
		t.Fatalf("a batch moved the selection from %s to %s", selected.path, now.path)
	}
	prev := 0
	for i, s := range *shots {
		if s.rows < prev {
			t.Fatalf("frame %d dropped rows from %d to %d while results streamed in", i, prev, s.rows)
		}
		prev = s.rows
	}
}

func TestStreamedBatchesKeepTheFirstRowSelectedUntilTheUserMoves(t *testing.T) {
	f := newFixture(t, map[string]string{"zzz.txt": "nothing\n"})
	f.open(TabText, "needle")
	f.stream(textFile(f.path("m.txt"), 1))
	first := f.dialog.results.Selected
	f.stream(textFile(f.path("a.txt"), 7))
	if got := f.dialog.results.Selected; got != first {
		t.Fatalf("selection moved from row %d to %d", first, got)
	}
}
