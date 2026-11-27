package finder

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ZerGo0/editor/internal/palette"
	"github.com/egoist/mygo/ui"
)

var demoProject = map[string]string{
	"main.go":          "package main\n\nfunc main() {\n\tstartServer()\n}\n",
	"server/server.go": "package server\n\ntype Handler struct{}\n\nfunc startServer() {\n\t// needle here\n}\n",
	"docs/readme.md":   "# Title\n\nNothing to see, one needle.\n",
}

func TestTabKeysSwitchTabsAndKeepQuery(t *testing.T) {
	f := newFixture(t, demoProject)
	f.open(TabAll, "")
	f.type_("serv")
	f.key(0, ui.KeyTab)
	if f.dialog.Tab() != TabActions || f.dialog.Query() != "serv" {
		t.Fatalf("after Tab: tab %v query %q; want Actions and serv", f.dialog.Tab(), f.dialog.Query())
	}
	f.key(0, ui.KeyTab)
	f.key(0, ui.KeyTab)
	if f.dialog.Tab() != TabText {
		t.Fatalf("tab %v; want Text after three Tab keys", f.dialog.Tab())
	}
	f.key(0, ui.KeyTab)
	if f.dialog.Tab() != TabAll {
		t.Fatalf("tab %v; want All after wrapping", f.dialog.Tab())
	}
	f.key(ui.Shift, ui.KeyTab)
	if f.dialog.Tab() != TabText || f.dialog.Query() != "serv" {
		t.Fatalf("after Shift+Tab: tab %v query %q; want Text and serv", f.dialog.Tab(), f.dialog.Query())
	}
	f.click("Files")
	if f.dialog.Tab() != TabFiles || f.dialog.Query() != "serv" {
		t.Fatalf("after click: tab %v query %q; want Files and serv", f.dialog.Tab(), f.dialog.Query())
	}
}

func TestDialogKeepsQueryAndTabForNextOpening(t *testing.T) {
	f := newFixture(t, demoProject)
	f.open(TabFiles, "")
	f.type_("srv")
	f.key(0, ui.KeyEscape)
	if f.dialog.IsOpen() {
		t.Fatal("Escape did not close the dialog")
	}
	st := f.dialog.Snapshot()
	if st.Tab != "Files" || st.Query != "srv" {
		t.Fatalf("snapshot %+v; want Files and srv", st)
	}
	again := NewDialog(f.env)
	again.Restore(st)
	again.Open(again.Tab(), "")
	if again.Tab() != TabFiles || again.Query() != "srv" {
		t.Fatalf("restored tab %v query %q", again.Tab(), again.Query())
	}
}

func TestFilesTabListsRecentlyOpenedThenFiles(t *testing.T) {
	f := newFixture(t, demoProject)
	f.recents = []string{f.path("docs/readme.md"), f.path("main.go")}
	f.host.active = f.path("docs/readme.md")
	f.open(TabFiles, "")
	if got := f.labels(); !slices.Equal(got, []string{"main.go"}) || !slices.Equal(f.groups(), []string{groupRecentlyOpened}) {
		t.Fatalf("empty query: %v groups %v; want main.go only, the active file left out", got, f.groups())
	}
	f.type_("go")
	if got := f.labels(); len(got) != 2 || got[0] != "main.go" || got[1] != "server.go" || !slices.Equal(f.groups(), []string{groupRecentlyOpened, groupFileResults}) {
		t.Fatalf("typed query: %v groups %v", got, f.groups())
	}
	f.key(0, ui.KeyDown)
	f.key(0, ui.KeyEnter)
	if len(f.opened) != 1 || f.opened[0].Path != f.path("server/server.go") || f.dialog.IsOpen() {
		t.Fatalf("opened %v, open %v", f.opened, f.dialog.IsOpen())
	}
	f.open(TabFiles, "")
	f.key(ui.Cmd, ui.KeyEnter)
	if len(f.sides) != 2 || !f.sides[1] {
		t.Fatalf("Cmd+Enter did not open to the side: %v", f.sides)
	}
}

func TestFilesTabForgetsRecentFile(t *testing.T) {
	f := newFixture(t, demoProject)
	f.recents = []string{f.path("main.go")}
	f.open(TabFiles, "")
	f.tt.Frame()
	f.click(removeRecentLabel)
	if !slices.Equal(f.forgot, []string{f.path("main.go")}) {
		t.Fatalf("forgot %v", f.forgot)
	}
}

func TestFilesTabPrefixModes(t *testing.T) {
	f := newFixture(t, demoProject)
	main := f.path("main.go")
	f.host.active = main
	buf := newBuffer(main, "package main\n\nfunc main() {}\n\nfunc helper() {}\n")
	f.host.buffers[main] = buf

	f.open(TabFiles, ":2")
	if got := f.labels(); !slices.Equal(got, []string{"Go to line 2"}) {
		t.Fatalf("line mode: %v", got)
	}
	f.key(0, ui.KeyEnter)
	if len(f.opened) != 1 || f.opened[0].Start.Line != 1 || f.opened[0].Path != main {
		t.Fatalf("opened %+v", f.opened)
	}

	f.open(TabFiles, "@help")
	if got := f.labels(); !slices.Equal(got, []string{"helper"}) {
		t.Fatalf("editor symbols: %v", got)
	}
	if f.dialog.label() != labelEditorSyms {
		t.Fatalf("label %q", f.dialog.label())
	}

	f.open(TabFiles, "?")
	if got := f.labels(); len(got) != len(quickHelp) {
		t.Fatalf("help: %v", got)
	}
	f.open(TabFiles, ">sav")
	if f.dialog.Tab() != TabActions || f.dialog.Query() != "sav" {
		t.Fatalf("> did not switch to Actions: %v %q", f.dialog.Tab(), f.dialog.Query())
	}
	f.open(TabFiles, "%needle")
	if f.dialog.Tab() != TabText || f.dialog.Query() != "needle" {
		t.Fatalf("%% did not switch to Text: %v %q", f.dialog.Tab(), f.dialog.Query())
	}
}

func TestWorkspaceSymbolsRefreshWhenIndexIsReady(t *testing.T) {
	f := newFixture(t, demoProject)
	f.open(TabFiles, "#Handler")
	if got := f.labels(); len(got) != 0 {
		t.Fatalf("symbols before indexing: %v", got)
	}
	server := f.path("server/server.go")
	f.env.Engine = nil
	engine := newEngineWithSymbols(f, server)
	f.dialog.env.Engine = engine
	f.dialog.IndexChanged()
	f.settle()
	if got := f.labels(); !slices.Equal(got, []string{"Handler"}) {
		t.Fatalf("symbols after the index is ready: %v", got)
	}
}

func TestActionsTabKeepsRecentlyUsedBehavior(t *testing.T) {
	f := newFixture(t, demoProject)
	f.open(TabActions, "")
	if got := f.labels(); !slices.Equal(got, []string{"File: Save", "View: Close Editor", "Edit: Build"}) || len(f.groups()) != 0 {
		t.Fatalf("no history: %v groups %v", got, f.groups())
	}
	f.used = []palette.Item{{ID: "build", Label: "Edit: Build"}, {ID: "close", Label: "View: Close Editor"}}
	f.open(TabActions, "")
	if got := f.labels(); !slices.Equal(got, []string{"Edit: Build", "View: Close Editor", "File: Save"}) || !slices.Equal(f.groups(), []string{groupRecentlyUsed, groupOtherCommands}) {
		t.Fatalf("history: %v groups %v", got, f.groups())
	}
	f.type_("clo")
	if got := f.labels(); !slices.Equal(got, []string{"View: Close Editor"}) {
		t.Fatalf("typed: %v", got)
	}
	f.key(0, ui.KeyEnter)
	if !slices.Equal(f.ran, []string{"close"}) || f.dialog.IsOpen() {
		t.Fatalf("ran %v open %v", f.ran, f.dialog.IsOpen())
	}
}

func TestTextTabFindsMatchesWithToggles(t *testing.T) {
	f := newFixture(t, demoProject)
	f.open(TabText, "")
	f.type_("needle")
	if got := f.labels(); len(got) != 2 || !strings.Contains(got[0], "needle") {
		t.Fatalf("matches %v", got)
	}
	f.setQuery("NEEDLE")
	if got := f.labels(); len(got) != 2 {
		t.Fatalf("case-insensitive matches %v", got)
	}
	f.click("Match Case")
	if got := f.labels(); len(got) != 0 {
		t.Fatalf("Match Case still found %v", got)
	}
	f.click("Match Case")
	f.click("Use Regular Expression")
	f.setQuery("need[l]e")
	if got := f.labels(); len(got) != 2 {
		t.Fatalf("regex matches %v", got)
	}
	f.setQuery("need[l")
	if f.dialog.queryError() == "" {
		t.Fatal("an invalid regular expression shows no error")
	}
	f.click("Use Regular Expression")
	f.click("Match Whole Word")
	f.setQuery("need")
	if got := f.labels(); len(got) != 0 {
		t.Fatalf("Words matched a partial word: %v", got)
	}
}

func TestTextMatchOpensAtItsPosition(t *testing.T) {
	f := newFixture(t, demoProject)
	f.open(TabText, "")
	f.type_("needle here")
	f.key(0, ui.KeyEnter)
	if len(f.opened) != 1 || f.opened[0].Path != f.path("server/server.go") || f.opened[0].Start.Line != 5 || !f.opened[0].HasRange {
		t.Fatalf("opened %+v", f.opened)
	}
}

func TestAllTabShowsSectionsWithMoreItems(t *testing.T) {
	project := map[string]string{}
	f := newFixture(t, project)
	for i := range 7 {
		name := "item" + strconv.Itoa(i) + ".txt"
		project[name] = "item line\nitem again\n"
		f.writeProjectFile(name, "item line\nitem again\n")
		f.commands = append(f.commands, palette.Item{ID: "item" + strconv.Itoa(i), Label: "Item Action " + strconv.Itoa(i)})
	}
	f.reindex()
	f.open(TabAll, "item")
	if !slices.Equal(f.groups(), []string{groupFiles, groupActions, groupText}) {
		t.Fatalf("groups %v", f.groups())
	}
	var more []string
	for _, e := range f.dialog.entries {
		if e.kind == kindMore {
			more = append(more, e.label)
		}
	}
	if !slices.Equal(more, []string{"Show 2 more files", "Show 2 more actions", "Show 9 more matches"}) {
		t.Fatalf("more items %v", more)
	}
	f.recents = nil
	f.tt.Key(ui.Cmd, ui.KeyA)
	f.key(0, ui.KeyBackspace)
	if len(f.dialog.entries) != 0 {
		t.Fatalf("no recents: %v", f.labels())
	}
}

func TestAllTabListsRecentFilesFirst(t *testing.T) {
	f := newFixture(t, demoProject)
	f.recents = []string{f.path("main.go"), f.path("docs/readme.md")}
	f.open(TabAll, "")
	if got := f.labels(); !slices.Equal(got, []string{"main.go", "readme.md"}) || !slices.Equal(f.groups(), []string{groupRecentFiles}) {
		t.Fatalf("%v groups %v", got, f.groups())
	}
}

func TestMoreItemSwitchesTab(t *testing.T) {
	f := newFixture(t, map[string]string{})
	for i := range 7 {
		f.writeProjectFile("item"+strconv.Itoa(i)+".txt", "x\n")
	}
	f.reindex()
	f.open(TabAll, "item")
	last := -1
	for i, e := range f.dialog.entries {
		if e.kind == kindMore && e.tab == TabFiles {
			last = i
		}
	}
	if last < 0 {
		t.Fatalf("no more item for files: %v", f.labels())
	}
	f.click("Show 2 more files")
	if f.dialog.Tab() != TabFiles || f.dialog.Query() != "item" || len(f.dialog.entries) != 7 {
		t.Fatalf("tab %v query %q entries %d", f.dialog.Tab(), f.dialog.Query(), len(f.dialog.entries))
	}
}
