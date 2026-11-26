package workspace

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"slices"
	"time"
	"unicode"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/glob"
	"github.com/ZerGo0/editor/internal/history"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/ZerGo0/editor/internal/layout"
	"github.com/ZerGo0/editor/internal/palette"
	"github.com/ZerGo0/editor/internal/search/everywhere"
	"github.com/ZerGo0/editor/internal/search/files"
	"github.com/ZerGo0/editor/internal/search/symbols"
	searchview "github.com/ZerGo0/editor/internal/search/view"
	"github.com/ZerGo0/editor/internal/session"
	"github.com/ZerGo0/editor/internal/settings"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	SearchView         = "search"
	reindexDelay       = time.Second
	maxRecentFiles     = 50
	doubleShiftWindow  = 400 * time.Millisecond
	paletteIconSize    = 16
	beforeReplaceLabel = "Before Replace"
)

type searchState struct {
	files   *files.Index
	scanner *files.Scanner
	view    *searchview.View
	recent  []string
	timer   *time.Timer
	cancel  context.CancelFunc
	shiftAt time.Time
}

func (w *Workspace) initSearch(saved session.Project) {
	w.search.files = &files.Index{Root: w.Root}
	syms := symbols.NewIndex()
	w.search.scanner = &files.Scanner{Root: w.Root, Visitors: []files.Visitor{everywhere.SymbolVisitor{Index: syms}, w.insights.todos}}
	engine := everywhere.New(&everywhere.Env{
		Root:     w.Root,
		Files:    w.search.files,
		Symbols:  syms,
		Actions:  w.commandItems,
		Recent:   func() []string { return slices.Clone(w.search.recent) },
		Modified: w.modifiedTexts,
		Repo:     w.openRepo,
	})
	w.search.view = searchview.New(searchview.Env{
		Host:        w,
		Providers:   engine,
		Files:       w.search.files,
		Settings:    w.env.Settings,
		OpenFiles:   w.openPaths,
		Open:        w.openLocation,
		Reveal:      func(p string) { w.revealPath(p, false) },
		RunAction:   w.run,
		GitRef:      w.checkoutRef,
		GitCommit:   w.showCommit,
		BeforeWrite: w.beforeReplace,
		FocusEditor: w.focusEditor,
	})
	if len(saved.Search) > 0 {
		var st searchview.State
		if err := json.Unmarshal(saved.Search, &st); err == nil {
			w.search.view.Restore(st)
		}
	}
	w.addSideView(toolView{id: SearchView, title: "Search", command: "workbench.view.search", icon: icons.Search, view: w.search.view.View, badge: w.search.view.Count})
	w.palette.Icon = paletteIcon
}

func (w *Workspace) snapshotSearch(p *session.Project) {
	data, err := json.Marshal(w.search.view.Snapshot())
	if err != nil {
		log.Printf("save search state: %v", err)
		return
	}
	p.Search = data
}

func (w *Workspace) openRepo() *git.Repo {
	repo, err := git.Open(w.ctx, w.Root)
	if err != nil {
		return nil
	}
	return repo
}

func (w *Workspace) reindex() {
	if w.search.cancel != nil {
		w.search.cancel()
	}
	ctx, cancel := context.WithCancel(w.ctx)
	w.search.cancel = cancel
	patterns := slices.Clone(w.env.Settings.Strings(settings.Exclude))
	excluded := func(rel string) bool { return glob.MatchAny(patterns, rel) }
	go func() {
		if err := w.search.files.Refresh(ctx, excluded); err != nil {
			return
		}
		w.update(w.search.view.IndexChanged)
		paths, _ := w.search.files.Paths()
		w.search.scanner.Scan(ctx, paths)
		w.update(w.search.view.IndexChanged)
	}()
}

func (w *Workspace) scheduleReindex() {
	if w.search.timer != nil {
		w.search.timer.Stop()
	}
	w.search.timer = time.AfterFunc(reindexDelay, func() { w.update(w.reindex) })
}

func (w *Workspace) touchRecent(p string) {
	if isVirtual(p) {
		return
	}
	r := append([]string{p}, slices.DeleteFunc(w.search.recent, func(q string) bool { return q == p })...)
	w.search.recent = r[:min(len(r), maxRecentFiles)]
}

func (w *Workspace) modifiedTexts() map[string]string {
	out := map[string]string{}
	for _, d := range w.modified() {
		out[d.Path] = d.Buffer.String()
	}
	return out
}

func (w *Workspace) openPaths() []string {
	var out []string
	for _, t := range w.tabs() {
		if !isVirtual(t.Path) && !slices.Contains(out, t.Path) {
			out = append(out, t.Path)
		}
	}
	return out
}

func (w *Workspace) commandItems() []palette.Item {
	reg := w.env.Commands
	items := make([]palette.Item, 0, len(reg.All()))
	seen := map[string]bool{}
	add := func(id string) {
		cmd := reg.Get(id)
		if cmd == nil || seen[id] {
			return
		}
		seen[id] = true
		items = append(items, palette.Item{ID: id, Label: cmd.Label(), Keys: w.keyLabel(id)})
	}
	for _, id := range reg.Recent() {
		add(id)
	}
	for _, cmd := range reg.All() {
		add(cmd.ID)
	}
	return items
}

func (w *Workspace) ShowCommands() {
	w.palette.Show("Command Palette", palette.Tab{
		Placeholder: "Type the name of a command to run",
		Search:      palette.Static(w.commandItems()),
		Hint:        "Enter to run",
	})
}

func (w *Workspace) ShowSearch() {
	w.ShowSideView(SearchView)
}

func (w *Workspace) FocusSearch(scope searchview.Scope) {
	w.ShowSideView(SearchView)
	w.search.view.Focus(scope, "")
}

func (w *Workspace) FindInFiles() {
	w.ShowSideView(SearchView)
	w.search.view.Focus(searchview.ScopeText, w.searchSeed())
}

func (w *Workspace) GoToSymbolInEditor() {
	w.ShowSideView(SearchView)
	w.search.view.FocusEditorSymbols()
}

func (w *Workspace) NextSearchResult() {
	w.search.view.Next()
}

func (w *Workspace) PreviousSearchResult() {
	w.search.view.Prev()
}

func (w *Workspace) searchSeed() string {
	t := w.active.ActiveTab()
	if t == nil || t.Editor == nil {
		return ""
	}
	if s := t.Editor.SelectedText(); s != "" {
		return s
	}
	return wordAt(t.Editor.Buffer.Line(t.Editor.CursorPos().Line), t.Editor.CursorPos().Col)
}

func wordAt(line string, col int) string {
	runes := []rune(line)
	isWord := func(r rune) bool { return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) }
	at := len([]rune(line[:min(col, len(line))]))
	start, end := at, at
	for start > 0 && isWord(runes[start-1]) {
		start--
	}
	for end < len(runes) && isWord(runes[end]) {
		end++
	}
	return string(runes[start:end])
}

func (w *Workspace) acceptPalette(item palette.Item) {
	switch v := item.Value.(type) {
	case languagePick:
		w.setLanguage(v)
	case switcherPick:
		w.openPick(v)
	default:
		w.run(item.ID)
	}
}

func paletteIcon(c *ui.Context, item palette.Item) {
	if v, ok := item.Value.(switcherPick); ok && !isVirtual(v.tab.Path) {
		icons.FileIcon(c, filepath.Base(v.tab.Path), false, false, paletteIconSize)
	}
}

func (w *Workspace) openLocation(loc searchview.Location, o searchview.OpenOptions) {
	g := w.active
	if o.Side {
		g = w.sideGroup()
	}
	t := w.openIn(g, loc.Path, o.Preview, o.Focus)
	if loc.HasRange {
		revealRange(t, loc.Start, loc.End)
	}
}

func (w *Workspace) sideGroup() *layout.Group {
	groups := w.layout.Groups()
	if i := slices.Index(groups, w.active); i >= 0 && i+1 < len(groups) {
		return groups[i+1]
	}
	added := w.newGroup()
	if w.layout.Split(w.active, added, false) {
		return added
	}
	return w.active
}

func revealRange(t *layout.Tab, start, end editor.Pos) {
	if t.Editor == nil {
		t.Restore = &layout.Position{Line: start.Line, Col: start.Col, Reveal: true, End: &end}
		return
	}
	buf := t.Editor.Buffer
	t.Editor.GoTo(start)
	t.Editor.Select(buf.Offset(start), buf.Offset(end))
}

func (w *Workspace) beforeReplace(path string) {
	info, err := os.Stat(path)
	if err != nil || info.Size() > history.DefaultMaxFileBytes {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	w.update(func() {
		if h := w.historyUI(); h != nil {
			h.Record(path, data, beforeReplaceLabel)
		}
	})
}

func (w *Workspace) focusEditor() {
	w.focusTab = w.active.ActiveTab()
}

func (w *Workspace) observeShift(ev ui.InputEvent) {
	if ev.Kind != ui.InputKeyDown || ev.Repeat {
		return
	}
	if ev.Code != ui.CodeShiftLeft && ev.Code != ui.CodeShiftRight || ev.Mods != ui.Shift {
		w.search.shiftAt = time.Time{}
		return
	}
	now := time.Now()
	if !w.search.shiftAt.IsZero() && now.Sub(w.search.shiftAt) < doubleShiftWindow {
		w.search.shiftAt = time.Time{}
		w.FocusSearch(searchview.ScopeAll)
		return
	}
	w.search.shiftAt = now
}
