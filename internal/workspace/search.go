package workspace

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"slices"
	"time"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/command"
	"github.com/ZerGo0/editor/internal/fsutil"
	"github.com/ZerGo0/editor/internal/glob"
	"github.com/ZerGo0/editor/internal/history"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/ZerGo0/editor/internal/layout"
	"github.com/ZerGo0/editor/internal/palette"
	"github.com/ZerGo0/editor/internal/search/everywhere"
	"github.com/ZerGo0/editor/internal/search/files"
	"github.com/ZerGo0/editor/internal/search/finder"
	"github.com/ZerGo0/editor/internal/search/references"
	"github.com/ZerGo0/editor/internal/search/symbols"
	"github.com/ZerGo0/editor/internal/session"
	"github.com/ZerGo0/editor/internal/settings"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	FindPanel          = "find"
	showFindWindowID   = "search.showFindWindow"
	reindexDelay       = time.Second
	maxRecentFiles     = 50
	maxScannedFiles    = 10000
	doubleShiftWindow  = 400 * time.Millisecond
	beforeReplaceLabel = "Before Replace"
)

type searchState struct {
	files       *files.Index
	scanner     *files.Scanner
	dialog      *finder.Dialog
	recentFiles *finder.RecentFiles
	find        *finder.FindWindow
	recent      []string
	timer       *time.Timer
	cancel      context.CancelFunc
	shiftAt     time.Time
	engine      *everywhere.Search
	symbols     *symbols.Index
}

func (w *Workspace) initSearch(saved session.Project) {
	w.palette.WindowKeys = w.modalKeys()
	w.search.files = &files.Index{Root: w.Root}
	syms := symbols.NewIndex()
	w.search.symbols = syms
	w.search.scanner = &files.Scanner{Root: w.Root, MaxFiles: maxScannedFiles, Visitors: []files.Visitor{everywhere.SymbolVisitor{Index: syms}, w.insights.todos}}
	engine := everywhere.New(&everywhere.Env{
		Root:    w.Root,
		Files:   w.search.files,
		Symbols: syms,
		Actions: w.commandList,
		Recent:  func() []string { return slices.Clone(w.search.recent) },
	})
	env := finder.Env{
		Host:            w,
		Engine:          engine,
		Files:           w.search.files,
		Settings:        w.settings,
		OpenFiles:       w.openPaths,
		Open:            w.openLocation,
		Commands:        w.commandItems,
		RunAction:       w.runCommand,
		ForgetRecent:    w.forgetRecent,
		ToolWindows:     w.toolWindows,
		BeforeWrite:     w.beforeReplace,
		ShowFindResults: w.showFindResults,
		WindowKeys:      w.modalKeys(),
	}
	w.search.engine = engine
	w.search.dialog = finder.NewDialog(env)
	w.search.recentFiles = finder.NewRecentFiles(env)
	w.search.find = finder.NewFindWindow(env)
	if len(saved.Search) > 0 {
		var st finder.State
		if err := json.Unmarshal(saved.Search, &st); err == nil {
			w.search.dialog.Restore(st)
		}
	}
	w.addPanel(toolView{id: FindPanel, title: "Find", command: showFindWindowID, icon: icons.Search, view: w.search.find.View, badge: w.search.find.Count, actions: w.search.find.Actions})
}

func (w *Workspace) snapshotSearch(p *session.Project) {
	st := w.search.dialog.Snapshot()
	data, err := json.Marshal(st)
	if err != nil {
		log.Printf("save search state: %v", err)
		return
	}
	p.Search = data
}

func (w *Workspace) reindex() {
	if w.search.cancel != nil {
		w.search.cancel()
	}
	ctx, cancel := context.WithCancel(w.ctx)
	w.search.cancel = cancel
	patterns := slices.Clone(w.settings.Strings(settings.Exclude))
	excluded := func(rel string) bool { return glob.MatchAny(patterns, rel) }
	applog.Go(func() {
		if err := w.search.files.Refresh(ctx, excluded); err != nil {
			return
		}
		w.updateQuietly(w.indexChanged)
		paths, _ := w.search.files.Paths()
		w.search.scanner.Scan(ctx, paths)
		w.updateQuietly(w.indexChanged)
	})
}

func (w *Workspace) indexChanged() bool {
	w.search.dialog.IndexChanged()
	return w.search.dialog.IsOpen()
}

func (w *Workspace) scheduleReindex() {
	if w.search.timer != nil {
		w.search.timer.Stop()
	}
	w.search.timer = time.AfterFunc(reindexDelay, func() {
		w.updateQuietly(func() bool {
			w.reindex()
			return false
		})
	})
}

func (w *Workspace) touchRecent(p string) {
	if !onDisk(p) {
		return
	}
	r := append([]string{p}, slices.DeleteFunc(w.search.recent, func(q string) bool { return q == p })...)
	w.search.recent = r[:min(len(r), maxRecentFiles)]
	if w.env.TouchFile != nil {
		w.env.TouchFile(p)
	}
}

func (w *Workspace) loadRecentFiles(saved []string) {
	for _, p := range saved {
		p = w.abs(p)
		if _, err := os.Stat(p); err == nil && !slices.Contains(w.search.recent, p) {
			w.search.recent = append(w.search.recent, p)
		}
	}
	if w.env.RecentFiles != nil {
		for _, p := range w.env.RecentFiles() {
			if fsutil.Within(w.Root, p) && !slices.Contains(w.search.recent, p) {
				w.search.recent = append(w.search.recent, p)
			}
		}
	}
	w.search.recent = w.search.recent[:min(len(w.search.recent), maxRecentFiles)]
}

func (w *Workspace) snapshotRecentFiles() []string {
	out := make([]string, len(w.search.recent))
	for i, p := range w.search.recent {
		out[i] = w.rel(p)
	}
	return out
}

func (w *Workspace) openPaths() []string {
	var out []string
	for _, t := range w.tabs() {
		if onDisk(t.Path) && !slices.Contains(out, t.Path) {
			out = append(out, t.Path)
		}
	}
	return out
}

func (w *Workspace) commandList() []palette.Item {
	recent, other := w.commandItems()
	return append(recent, other...)
}

func (w *Workspace) commandItems() (recent, other []palette.Item) {
	reg := w.env.Commands
	seen := map[string]bool{}
	item := func(cmd *command.Command) palette.Item {
		seen[cmd.ID] = true
		return palette.Item{ID: cmd.ID, Label: cmd.Label(), Keys: w.keyLabel(cmd.ID)}
	}
	for _, id := range reg.Recent() {
		if cmd := reg.Get(id); cmd != nil && !seen[id] {
			recent = append(recent, item(cmd))
		}
	}
	for _, cmd := range reg.All() {
		if !seen[cmd.ID] {
			other = append(other, item(cmd))
		}
	}
	return recent, other
}

func (w *Workspace) OpenSearch(tab finder.Tab) {
	w.openSearch(tab, "")
}

func (w *Workspace) openSearch(tab finder.Tab, seed string) {
	w.closeSearchPopups()
	w.search.dialog.Open(tab, seed)
}

func (w *Workspace) closeSearchPopups() {
	w.palette.Close()
	w.search.dialog.Close()
	w.search.recentFiles.Close()
}

func (w *Workspace) searchTextShown() bool {
	return w.search.dialog.IsOpen() && w.search.dialog.Tab() == finder.TabText
}

func (w *Workspace) searchOpen() bool {
	return w.search.dialog.IsOpen() || w.search.recentFiles.IsOpen()
}

func (w *Workspace) ShowCommands() {
	w.openSearch(finder.TabActions, "")
}

func (w *Workspace) ShowSearch() {
	w.openSearch(finder.TabText, "")
}

func (w *Workspace) QuickOpen() {
	w.openSearch(finder.TabFiles, "")
}

func (w *Workspace) FindInFiles() {
	w.openSearch(finder.TabText, w.searchSeed())
}

func (w *Workspace) ReplaceInFiles() {
	w.closeSearchPopups()
	w.search.dialog.OpenReplace(w.searchSeed())
}

func (w *Workspace) ShowRecentFiles() {
	w.closeSearchPopups()
	w.search.recentFiles.Open()
}

func (w *Workspace) GoToSymbolInEditor() {
	w.openSearch(finder.TabFiles, "@")
}

func (w *Workspace) GoToSymbolInWorkspace() {
	w.openSearch(finder.TabFiles, "#")
}

func (w *Workspace) NextSearchResult() {
	w.search.find.Next()
}

func (w *Workspace) PreviousSearchResult() {
	w.search.find.Prev()
}

func (w *Workspace) showFindResults(res finder.FindResults) {
	w.search.find.Show(res)
	w.ShowPanel(FindPanel)
}

func (w *Workspace) runCommand(id string) {
	w.env.Commands.MarkUsed(id)
	w.run(id)
}

func (w *Workspace) forgetRecent(path string) {
	w.search.recent = slices.DeleteFunc(w.search.recent, func(p string) bool { return p == path })
	if w.env.ForgetFile != nil {
		w.env.ForgetFile(path)
	}
}

func (w *Workspace) toolWindows() []finder.ToolWindow {
	var out []finder.ToolWindow
	for _, v := range w.tools.sideViews {
		out = append(out, finder.ToolWindow{Title: v.title, Keys: w.keyLabel(v.command), Icon: v.icon, Show: func() { w.FocusSideView(v.id) }})
	}
	for _, v := range w.tools.panels {
		out = append(out, finder.ToolWindow{Title: v.title, Keys: w.keyLabel(v.command), Icon: v.icon, Show: func() { w.ShowPanel(v.id) }})
	}
	return out
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
	at := len([]rune(line[:min(col, len(line))]))
	start, end := at, at
	for start > 0 && references.IsWordRune(runes[start-1]) {
		start--
	}
	for end < len(runes) && references.IsWordRune(runes[end]) {
		end++
	}
	return string(runes[start:end])
}

func (w *Workspace) openLocation(loc finder.Location, o finder.OpenOptions) {
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
		w.openSearch(finder.TabAll, "")
		return
	}
	w.search.shiftAt = now
}
