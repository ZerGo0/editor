package workspace

import (
	"slices"
	"strconv"
	"strings"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/fuzzy"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/ZerGo0/editor/internal/palette"
	"github.com/ZerGo0/editor/internal/search/everywhere"
	"github.com/ZerGo0/editor/internal/search/symbols"
	searchview "github.com/ZerGo0/editor/internal/search/view"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	quickOpenLimit      = 50
	commandsPrefix      = ">"
	editorSymPrefix     = "@"
	projectSymPrefix    = "#"
	linePrefix          = ":"
	textPrefix          = "%"
	helpPrefix          = "?"
	lineColSeparator    = ":"
	commandsTitle       = "Command Palette"
	goToFileTitle       = "Go to File"
	textSearchFormat    = "Search for \"%s\" in files"
	recentlyUsedGroup   = "recently used"
	otherCommandsGroup  = "other commands"
	recentlyOpenedGroup = "recently opened"
	fileResultsGroup    = "file results"
	removeRecentLabel   = "Remove from Recently Opened"
)

type quickLocation struct {
	path      string
	line, col int
	hasPos    bool
}

func (w *Workspace) initQuickOpen() {
	w.palette.Modes = []palette.Mode{
		{Label: goToFileTitle, Placeholder: "Search files by name (type ? for help)", Search: w.searchFiles, Accept: w.acceptLocation,
			AcceptSide: w.acceptLocationSide, Background: w.acceptLocationBackground, RemoveLabel: removeRecentLabel, Remove: w.forgetRecent,
			Empty: "No matching files"},
		{Prefix: commandsPrefix, Label: commandsTitle, Placeholder: "Type the name of a command to run", Search: w.searchCommands,
			Accept: w.runFromPalette, Empty: "No matching commands"},
		{Prefix: editorSymPrefix, Label: "Go to Symbol in Editor", Placeholder: "Type the name of a symbol in this file",
			Search: w.searchEditorSymbols, Accept: w.acceptLocation, Empty: "No matching symbols in this file"},
		{Prefix: projectSymPrefix, Label: "Go to Symbol in Workspace", Placeholder: "Type the name of a symbol",
			Search: w.searchProjectSymbols, Accept: w.acceptLocation, Empty: "Type to search for symbols"},
		{Prefix: linePrefix, Label: "Go to Line/Column", Placeholder: "Type a line number, then a colon and a column",
			Search: w.searchLine, Accept: w.acceptLocation, Empty: "Open a text file to go to a line"},
		{Prefix: textPrefix, Label: "Search in Files", Placeholder: "Type the text to search for", Search: w.searchText,
			Accept: func(it palette.Item) { w.findText(it.ID) }, Empty: "Type the text to search for"},
		{Prefix: helpPrefix, Label: "Help", Search: palette.Static(quickHelp)},
	}
}

var quickHelp = []palette.Item{
	{Label: commandsPrefix, Detail: "Show and Run Commands", Query: commandsPrefix},
	{Label: editorSymPrefix, Detail: "Go to Symbol in Editor", Query: editorSymPrefix},
	{Label: projectSymPrefix, Detail: "Go to Symbol in Workspace", Query: projectSymPrefix},
	{Label: linePrefix, Detail: "Go to Line/Column", Query: linePrefix},
	{Label: textPrefix, Detail: "Search for Text in Files", Query: textPrefix},
}

func (w *Workspace) QuickOpen(prefix string) {
	w.palette.Open(prefix)
}

func (w *Workspace) acceptLocation(it palette.Item) {
	w.openItem(it, searchview.OpenOptions{Focus: true})
}

func (w *Workspace) acceptLocationSide(it palette.Item) {
	w.openItem(it, searchview.OpenOptions{Focus: true, Side: true})
}

func (w *Workspace) acceptLocationBackground(it palette.Item) {
	w.openItem(it, searchview.OpenOptions{})
}

func (w *Workspace) openItem(it palette.Item, o searchview.OpenOptions) {
	loc, ok := it.Value.(quickLocation)
	if !ok {
		return
	}
	pos := editor.Pos{Line: loc.line, Col: loc.col}
	w.openLocation(searchview.Location{Path: loc.path, Start: pos, End: pos, HasRange: loc.hasPos}, o)
}

func (w *Workspace) forgetRecent(it palette.Item) {
	if loc, ok := it.Value.(quickLocation); ok {
		w.search.recent = slices.DeleteFunc(w.search.recent, func(p string) bool { return p == loc.path })
	}
}

func (w *Workspace) searchFiles(q string, done func([]palette.Item)) {
	s := w.search.engine.Session()
	s.MarkOpen(w.openPaths())
	current := ""
	if t := w.active.ActiveTab(); t != nil && q == "" {
		current = t.Path
	}
	run := func() []palette.Item {
		rs, _ := s.Files(q, quickOpenLimit+1)
		rs = slices.DeleteFunc(rs, func(r everywhere.Result) bool { return r.Path == current })
		recent := slices.DeleteFunc(slices.Clone(rs), func(r everywhere.Result) bool { return !r.Recent })
		other := slices.DeleteFunc(rs, func(r everywhere.Result) bool { return r.Recent })
		items := w.resultItems(recent)
		for i := range items {
			items[i].Group, items[i].Removable = recentlyOpenedGroup, true
		}
		others := w.resultItems(other)
		if len(items) > 0 {
			groupItems(others, fileResultsGroup)
		}
		items = append(items, others...)
		return items[:min(len(items), quickOpenLimit)]
	}
	if q == "" || w.win == nil {
		done(run())
		return
	}
	go func() {
		items := run()
		w.update(func() { done(items) })
	}()
}

func (w *Workspace) searchProjectSymbols(q string, done func([]palette.Item)) {
	if q == "" {
		done(nil)
		return
	}
	s, ctx := w.search.engine.Session(), w.ctx
	go func() {
		rs, _ := s.Symbols(ctx, q, quickOpenLimit)
		items := w.resultItems(rs)
		w.update(func() { done(items) })
	}()
}

func (w *Workspace) resultItems(rs []everywhere.Result) []palette.Item {
	items := make([]palette.Item, len(rs))
	for i, r := range rs {
		items[i] = palette.Item{Label: r.Label, Detail: r.Detail, Positions: r.Positions,
			Value: quickLocation{path: r.Path, line: r.Line, col: r.Col, hasPos: r.HasPos}}
		if r.Kind == everywhere.KindFile {
			name := r.Label
			items[i].Leading = func(c *ui.Context) { icons.FileIcon(c, name, false, false, token.IconSize.Px()) }
		} else {
			items[i].Icon = icons.ListView
		}
	}
	return items
}

func (w *Workspace) searchCommands(q string, done func([]palette.Item)) {
	recent, other := w.commandItems()
	recent, other = palette.Filter(recent, q), palette.Filter(other, q)
	if len(recent) > 0 {
		groupItems(recent, recentlyUsedGroup)
		groupItems(other, otherCommandsGroup)
	}
	done(append(recent, other...))
}

func (w *Workspace) runFromPalette(it palette.Item) {
	w.env.Commands.MarkUsed(it.ID)
	w.run(it.ID)
}

func groupItems(items []palette.Item, group string) {
	for i := range items {
		items[i].Group = group
	}
}

func (w *Workspace) searchEditorSymbols(q string, done func([]palette.Item)) {
	t := w.active.ActiveTab()
	if t == nil || t.Editor == nil || t.Editor.Buffer.Language() == nil {
		done(nil)
		return
	}
	path, lang, text := t.Path, t.Editor.Buffer.Language().Name, t.Editor.Buffer.String()
	go func() {
		items := symbolItems(path, symbols.Scan(lang, text), q)
		w.update(func() { done(items) })
	}()
}

func symbolItems(path string, syms []symbols.Symbol, q string) []palette.Item {
	rs := fuzzy.Filter(q, len(syms), func(i int) string { return syms[i].Name })
	items := make([]palette.Item, len(rs))
	for i, r := range rs {
		s := syms[r.Index]
		items[i] = palette.Item{Label: s.Name, Detail: s.Container, Positions: r.Match.Positions, Icon: icons.ListView,
			Value: quickLocation{path: path, line: s.Line, col: s.Col, hasPos: true}}
	}
	return items
}

func (w *Workspace) searchLine(q string, done func([]palette.Item)) {
	t := w.active.ActiveTab()
	if t == nil || t.Editor == nil {
		done(nil)
		return
	}
	lines := t.Editor.Buffer.LineCount()
	line, col, ok := parseLineCol(q)
	if !ok || line < 1 || line > lines {
		done([]palette.Item{{Label: "Type a line number between 1 and " + strconv.Itoa(lines), Query: linePrefix}})
		return
	}
	label := "Go to line " + strconv.Itoa(line)
	if col > 0 {
		label += ", column " + strconv.Itoa(col)
	}
	done([]palette.Item{{Label: label, Value: quickLocation{path: t.Path, line: line - 1, col: max(col-1, 0), hasPos: true}}})
}

func parseLineCol(q string) (line, col int, ok bool) {
	lineText, colText, hasCol := strings.Cut(strings.TrimSpace(q), lineColSeparator)
	line, err := strconv.Atoi(strings.TrimSpace(lineText))
	if err != nil {
		return 0, 0, false
	}
	if hasCol {
		if col, err = strconv.Atoi(strings.TrimSpace(colText)); err != nil {
			return 0, 0, false
		}
	}
	return line, col, true
}

func (w *Workspace) searchText(q string, done func([]palette.Item)) {
	if strings.TrimSpace(q) == "" {
		done(nil)
		return
	}
	done([]palette.Item{{ID: q, Label: strings.Replace(textSearchFormat, "%s", q, 1), Icon: icons.Search}})
}

func (w *Workspace) findText(q string) {
	w.ShowSideView(SearchView)
	w.search.view.Focus(searchview.ScopeText, q)
}

func (w *Workspace) ReplaceInFiles() {
	if st := w.search.view.Snapshot(); !st.ShowReplace {
		st.ShowReplace = true
		w.search.view.Restore(st)
	}
	w.FindInFiles()
}

func (w *Workspace) quickInputView(c *ui.Context) {
	open := w.palette.IsOpen()
	w.palette.View(c)
	if open && !w.palette.IsOpen() && w.focusTab == nil {
		w.focusTab = w.active.ActiveTab()
		c.Invalidate()
	}
}
