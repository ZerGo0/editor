package finder

import (
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestPreviewToggleShowsSelectedMatch(t *testing.T) {
	f := newFixture(t, demoProject)
	f.open(TabText, "")
	f.type_("needle here")
	if _, ok := f.tt.AccessByRole(ui.RoleGroup, previewLabel); ok {
		t.Fatal("the preview is shown before it is turned on")
	}
	f.click(previewLabel)
	if !f.dialog.Snapshot().Preview {
		t.Fatal("the toggle did not turn the preview on")
	}
	f.settle()
	st := f.dialog.preview.st
	if st == nil {
		t.Fatalf("no preview editor: %q loading %v", f.dialog.preview.message, f.dialog.preview.pending)
	}
	if got := st.SelectedText(); got != "needle here" {
		t.Fatalf("preview selects %q; want the match", got)
	}
	if !strings.Contains(st.Buffer.String(), "package server") || !st.ReadOnly {
		t.Fatal("the preview is not the read-only text of the matched file")
	}
	f.key(0, ui.KeyEscape)
	again := NewDialog(f.env)
	again.Restore(f.dialog.Snapshot())
	if !again.Snapshot().Preview {
		t.Fatal("the preview state is not kept")
	}
	f.open(TabText, "")
	f.click(previewLabel)
	if f.dialog.Snapshot().Preview {
		t.Fatal("the toggle did not turn the preview off")
	}
}

func TestPreviewFollowsSelectionAndShowsFilesWithoutRange(t *testing.T) {
	f := newFixture(t, demoProject)
	f.dialog.st.Preview = true
	f.recents = []string{f.path("main.go"), f.path("docs/readme.md")}
	f.open(TabFiles, "")
	f.settle()
	if f.dialog.preview.st == nil || !strings.Contains(f.dialog.preview.st.Buffer.String(), "package main") {
		t.Fatalf("preview of the first file: %+v", f.dialog.preview)
	}
	f.key(0, ui.KeyDown)
	if f.dialog.preview.st == nil || !strings.Contains(f.dialog.preview.st.Buffer.String(), "# Title") {
		t.Fatal("the preview does not follow the selection")
	}
}

func TestPreviewReportsBinaryFiles(t *testing.T) {
	f := newFixture(t, map[string]string{"blob.bin": "a\x00b"})
	f.dialog.st.Preview = true
	f.recents = []string{f.path("blob.bin")}
	f.open(TabFiles, "")
	f.settle()
	if f.dialog.preview.st != nil || f.dialog.preview.message != msgBinary {
		t.Fatalf("binary preview: %+v", f.dialog.preview)
	}
}

func TestOpenInFindWindowSendsTextResults(t *testing.T) {
	f := newFixture(t, demoProject)
	f.open(TabText, "")
	f.type_("needle")
	f.click(findWindowLabel)
	if f.dialog.IsOpen() || len(f.found) != 1 {
		t.Fatalf("open %v found %d", f.dialog.IsOpen(), len(f.found))
	}
	res := f.found[0]
	if res.Total != 2 || len(res.Files) != 2 || !strings.Contains(res.Title, "needle") {
		t.Fatalf("results %+v", res)
	}
}

func TestOpenInFindWindowSendsFileResults(t *testing.T) {
	f := newFixture(t, demoProject)
	f.open(TabFiles, "")
	f.type_("go")
	f.click(findWindowLabel)
	if len(f.found) != 1 || len(f.found[0].Files) != 2 || f.found[0].Files[0].Rel != "main.go" {
		t.Fatalf("found %+v", f.found)
	}
}

func TestOpenInFindWindowIsDisabledOutsideTextAndFiles(t *testing.T) {
	f := newFixture(t, demoProject)
	f.open(TabActions, "")
	if f.dialog.canOpenInFind() {
		t.Fatal("Open in Find Window is enabled on the Actions tab")
	}
	f.open(TabAll, "")
	if f.dialog.canOpenInFind() {
		t.Fatal("Open in Find Window is enabled on the All tab")
	}
}

func TestFileTypeFilterNarrowsFilesAndText(t *testing.T) {
	f := newFixture(t, demoProject)
	f.open(TabFiles, "")
	f.type_(".")
	if all := len(f.dialog.entries); all != 3 {
		t.Fatalf("files %v", f.labels())
	}
	f.click(filterLabel)
	if _, ok := f.tt.Access(allTypesLabel); !ok {
		t.Fatal("the filter menu has no All File Types item")
	}
	f.click("Markdown")
	if got := f.labels(); !slices.Equal(got, []string{"readme.md"}) {
		t.Fatalf("Markdown filter: %v", got)
	}
	f.dialog.setTab(TabText)
	f.setQuery("needle")
	if got := f.labels(); len(got) != 1 || !strings.Contains(f.dialog.entries[0].detail, "readme.md") {
		t.Fatalf("Markdown text filter: %v", f.dialog.entries)
	}
	f.click(filterLabel)
	f.click(allTypesLabel)
	if got := f.labels(); len(got) != 2 {
		t.Fatalf("All File Types: %v", got)
	}
}

func TestNonProjectItemsAreHiddenUntilIncluded(t *testing.T) {
	f := newFixture(t, demoProject)
	outside := t.TempDir() + "/outside.go"
	f.recents = []string{outside, f.path("main.go")}
	f.open(TabFiles, "")
	if got := f.labels(); !slices.Equal(got, []string{"main.go"}) {
		t.Fatalf("project items only: %v", got)
	}
	f.click(filterLabel)
	f.click(nonProjectLabel)
	if got := f.labels(); !slices.Equal(got, []string{"outside.go", "main.go"}) {
		t.Fatalf("with non-project items: %v", got)
	}
}

func TestRecentFilesPopupListsFilesAndToolWindows(t *testing.T) {
	f := newFixture(t, demoProject)
	f.host.active = f.path("main.go")
	f.recents = []string{f.path("main.go"), f.path("docs/readme.md")}
	f.recent.Open()
	f.settle()
	var titles []string
	for _, r := range f.recent.rows {
		titles = append(titles, r.Title)
	}
	if !slices.Contains(titles, recentLabel) || !slices.Contains(titles, toolWindowsGroup) {
		t.Fatalf("headers %v", titles)
	}
	if len(f.recent.items) != 5 || f.recent.items[1].file.Label != "readme.md" {
		t.Fatalf("items %+v; want readme.md first and the active file left out", f.recent.items)
	}
	f.type_("prob")
	if len(f.recent.items) != 2 || f.recent.items[1].tool == nil || f.recent.items[1].tool.Title != "Problems" {
		t.Fatalf("filtered items %+v", f.recent.items)
	}
	f.key(0, ui.KeyEnter)
	if !slices.Equal(f.shown, []string{"problems"}) || f.recent.IsOpen() {
		t.Fatalf("shown %v open %v", f.shown, f.recent.IsOpen())
	}
	f.recent.Open()
	f.settle()
	f.key(0, ui.KeyEnter)
	if len(f.opened) != 1 || f.opened[0].Path != f.path("docs/readme.md") {
		t.Fatalf("opened %+v", f.opened)
	}
	f.recent.Open()
	f.settle()
	f.key(0, ui.KeyEscape)
	if f.recent.IsOpen() {
		t.Fatal("Escape did not close the popup")
	}
}

func TestFindWindowGroupsByFileAndOpensMatch(t *testing.T) {
	f := newFixture(t, demoProject)
	f.open(TabText, "")
	f.type_("needle")
	f.click(findWindowLabel)
	f.find.Show(f.found[0])
	f.settle()
	if f.find.Count() != 2 || len(f.find.rows) != 4 {
		t.Fatalf("count %d rows %d; want two files with one match each", f.find.Count(), len(f.find.rows))
	}
	f.tt.Frame()
	f.click("readme.md")
	if len(f.find.rows) != 3 {
		t.Fatalf("collapsing a file left %d rows", len(f.find.rows))
	}
	f.find.setAllCollapsed(false)
	f.opened = nil
	f.find.Next()
	f.find.Next()
	if len(f.opened) != 2 || f.opened[0].Path != f.path("docs/readme.md") || f.opened[1].Path != f.path("server/server.go") || !f.opened[1].HasRange {
		t.Fatalf("opened %+v", f.opened)
	}
	f.find.Prev()
	if f.opened[2].Path != f.path("docs/readme.md") {
		t.Fatalf("Prev opened %+v", f.opened[2])
	}
	f.find.Clear()
	f.settle()
	if f.find.Count() != 0 {
		t.Fatal("Clear Results left results")
	}
}

func TestTextRowsShowTheFileNameAndLineAtTheRight(t *testing.T) {
	f := newFixture(t, demoProject)
	f.open(TabText, "needle")
	got := map[string]bool{}
	for _, e := range f.dialog.entries {
		got[e.detail] = true
		if row := f.dialog.row(e); row.Item.Aside != e.detail || row.Item.Tooltip != e.file.Rel {
			t.Errorf("row aside %q tooltip %q; want %q and the path %q", row.Item.Aside, row.Item.Tooltip, e.detail, e.file.Rel)
		}
	}
	if !got["server.go 6"] || !got["readme.md 3"] || len(got) != 2 {
		t.Fatalf("file labels %v; want server.go 6 and readme.md 3", got)
	}
}

func TestTextRowsAddFoldersWhenFileNamesRepeat(t *testing.T) {
	f := newFixture(t, map[string]string{
		"ext/history/view.go": "// needle\n",
		"ext/stream/view.go":  "// needle\n",
		"ext/post/row.go":     "// needle\n",
	})
	f.open(TabText, "needle")
	got := map[string]bool{}
	for _, e := range f.dialog.entries {
		got[e.detail] = true
	}
	for _, want := range []string{"history/view.go 1", "stream/view.go 1", "row.go 1"} {
		if !got[want] {
			t.Errorf("file labels %v lack %q", got, want)
		}
	}
}

func TestFilesRowsShowTheFolderAsAsideWithThePathAsTooltip(t *testing.T) {
	f := newFixture(t, demoProject)
	f.open(TabFiles, "server")
	for _, e := range f.dialog.entries {
		if e.kind != kindFile {
			continue
		}
		row := f.dialog.row(e)
		if row.Item.Description != "" || row.Item.Aside != e.detail || row.Item.Tooltip != relTo(f.host.root, e.path) {
			t.Fatalf("file row %+v", row.Item)
		}
	}
}

func TestFindWindowRowMenu(t *testing.T) {
	f := newFixture(t, demoProject)
	f.open(TabText, "")
	f.type_("needle")
	f.click(findWindowLabel)
	f.find.Show(f.found[0])
	f.settle()
	if err := f.tt.RightClick("readme.md"); err != nil {
		t.Fatal(err)
	}
	if err := f.tt.ChooseMenuItem("Copy Path"); err != nil {
		t.Fatal(err)
	}
	if got := f.tt.Clipboard(); got != f.path("docs/readme.md") {
		t.Fatalf("clipboard %q", got)
	}
	f.opened, f.sides = nil, nil
	if err := f.tt.RightClick("readme.md"); err != nil {
		t.Fatal(err)
	}
	if err := f.tt.ChooseMenuItem("Open to the Side"); err != nil {
		t.Fatal(err)
	}
	if len(f.opened) != 1 || f.opened[0].Path != f.path("docs/readme.md") || !f.sides[0] {
		t.Fatalf("opened %+v sides %v", f.opened, f.sides)
	}
}
