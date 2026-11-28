package finder

import (
	"context"
	"slices"
	"strconv"
	"strings"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/design/picker"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/search/everywhere"
	"github.com/ZerGo0/editor/internal/search/textsearch"
	"github.com/egoist/mygo/ui"
)

const (
	allSectionLimit  = 5
	recentLimit      = 8
	recentFetchLimit = 50
	filesLimit       = 50
	actionsLimit     = 2000
	symbolsLimit     = 50
	textRowLimit     = 500
	allTextLimit     = 100
	minAllText       = 2
)

const (
	prefixActions       = ">"
	prefixEditorSymbols = "@"
	prefixSymbols       = "#"
	prefixLine          = ":"
	prefixText          = "%"
	prefixHelp          = "?"
)

var modePrefixes = []string{prefixActions, prefixEditorSymbols, prefixSymbols, prefixLine, prefixText, prefixHelp}

const (
	groupRecentFiles    = "Recent Files"
	groupFiles          = "Files"
	groupActions        = "Actions"
	groupText           = "Text"
	groupRecentlyOpened = "Recent"
	groupFileResults    = "File Results"
	groupRecentlyUsed   = "Recently Used"
	groupOtherCommands  = "Other Commands"
)

type Dialog struct {
	env       Env
	st        State
	open      bool
	tab       Tab
	results   picker.ResultsState
	rows      []picker.Row
	entries   []entry
	rowEntry  []int
	quick     []entry
	text      textRun
	shown     int
	gen       int
	cancel    context.CancelFunc
	loading   bool
	searched  string
	resultsOf string
	progress  delayedBusy
	showBusy  bool
	settled   string
	moved     bool
	selectAll bool
	field     ui.Handle
	preview   preview
	types     []fileType
	typesFor  int
	pal       *token.Palette
	confirm   bool
	replacing bool
	focusWith bool
	replCache replacementCache
}

func NewDialog(env Env) *Dialog {
	d := &Dialog{env: env, text: textRun{env: env}, preview: preview{env: env}}
	d.preview.clear()
	return d
}

func (d *Dialog) IsOpen() bool { return d.open }

func (d *Dialog) Tab() Tab { return d.tab }

func (d *Dialog) Query() string { return d.st.Query }

func (d *Dialog) Snapshot() State {
	st := d.st
	st.Tab = d.tab.String()
	return st
}

func (d *Dialog) Restore(st State) {
	d.st = st
	d.tab = parseTab(st.Tab)
}

func (d *Dialog) Open(tab Tab, seed string) {
	d.open, d.selectAll, d.moved = true, !slices.Contains(modePrefixes, seed), false
	d.tab = tab
	if seed != "" {
		d.st.Query = seed
	}
	d.applyPrefix()
	d.refresh()
}

func (d *Dialog) OpenReplace(seed string) {
	d.Open(TabText, seed)
	d.st.ShowReplace, d.selectAll, d.focusWith = true, false, true
}

func (d *Dialog) Close() {
	if d.open && d.tab == TabText {
		d.st.History.Add(d.st.Query)
	}
	d.open, d.confirm = false, false
	d.stopSearch()
	d.text.reset()
	d.preview.clear()
	d.quick, d.entries, d.rows, d.rowEntry = nil, nil, nil, nil
	d.resultsOf, d.settled = "", ""
	d.results.Reset(nil)
}

func (d *Dialog) stopSearch() {
	d.gen++
	d.loading = false
	if d.cancel != nil {
		d.cancel()
		d.cancel = nil
	}
	d.text.stop()
}

func (d *Dialog) setTab(t Tab) {
	if t < 0 || t >= tabCount || t == d.tab {
		return
	}
	d.tab, d.moved = t, false
	d.applyPrefix()
	d.refresh()
}

func (d *Dialog) cycleTab(delta int) {
	d.setTab((d.tab + Tab(delta) + tabCount) % tabCount)
}

func (d *Dialog) applyPrefix() {
	if d.tab != TabFiles {
		return
	}
	switch {
	case strings.HasPrefix(d.st.Query, prefixActions):
		d.tab, d.st.Query = TabActions, strings.TrimLeft(strings.TrimPrefix(d.st.Query, prefixActions), " ")
	case strings.HasPrefix(d.st.Query, prefixText):
		d.tab, d.st.Query = TabText, strings.TrimPrefix(d.st.Query, prefixText)
	}
}

func (d *Dialog) queryChanged() {
	d.moved = false
	d.applyPrefix()
	d.refresh()
}

func (d *Dialog) IndexChanged() {
	if d.open {
		d.refresh()
	}
}

func (d *Dialog) Changed() {
	if d.open && d.tab != TabActions {
		d.refresh()
	}
}

func (d *Dialog) searchKey() string {
	parts := []string{strconv.Itoa(int(d.tab)), d.st.Query, strconv.FormatBool(d.st.MatchCase), strconv.FormatBool(d.st.WholeWord),
		strconv.FormatBool(d.st.Regex), strconv.FormatBool(d.st.NonProject), strings.Join(d.st.FileTypes, ","), d.st.Mask, d.st.Scope, d.st.Directory}
	return strings.Join(parts, "\x00")
}

func (d *Dialog) filter() everywhere.Filter {
	return everywhere.Filter{Globs: d.activeGlobs(), ProjectOnly: !d.st.NonProject}
}

func (d *Dialog) session() *everywhere.Session {
	if d.env.Engine == nil {
		return nil
	}
	s := d.env.Engine.Session()
	s.MarkOpen(d.env.openFiles())
	s.SetFilter(d.filter())
	return s
}

func (d *Dialog) wantsText() bool {
	switch d.tab {
	case TabText:
		return true
	case TabAll:
		return len([]rune(strings.TrimSpace(d.st.Query))) >= minAllText
	}
	return false
}

func (d *Dialog) refresh() {
	d.stopSearch()
	d.searched = d.searchKey()
	ctx, cancel := context.WithCancel(context.Background())
	d.cancel = cancel
	gen := d.gen
	if d.wantsText() {
		limit := resultLimit
		if d.tab == TabAll {
			limit = allTextLimit
		}
		d.startText(limit)
	} else {
		d.text.reset()
	}
	if d.tab == TabText {
		d.compose(d.text.takeRenewed())
		return
	}
	in := d.quickInput()
	sess := d.session()
	d.loading = true
	applog.Go(func() {
		entries := in.compute(ctx, sess)
		d.env.Host.Update(func() {
			if gen != d.gen {
				return
			}
			d.loading, d.quick = false, entries
			d.compose(true)
		})
	})
}

func (d *Dialog) startText(limit int) {
	req := textRequest{Pattern: d.st.Query, MatchCase: d.st.MatchCase, WholeWord: d.st.WholeWord, Regex: d.st.Regex,
		Include: d.activeGlobs(), NonProject: d.st.NonProject, Limit: limit}
	if d.tab == TabText {
		req.Include = append(req.Include, textsearch.ParseGlobs(d.st.Mask)...)
		req.Directory = d.directory()
		if d.st.Scope == ScopeDirectory && req.Directory == "" {
			d.text.reset()
			return
		}
	}
	d.text.start(req)
}

func (d *Dialog) directory() string {
	if d.st.Scope == ScopeDirectory {
		return strings.TrimSpace(d.st.Directory)
	}
	return ""
}

func (d *Dialog) awaitingText() bool {
	return d.tab == TabText && d.text.fresh && d.text.searching && len(d.entries) > 0
}

func (d *Dialog) compose(renewed bool) {
	d.shown = d.text.version
	if d.awaitingText() {
		return
	}
	entries := d.quick
	switch d.tab {
	case TabText:
		entries = d.textEntries(textRowLimit, "")
	case TabAll:
		entries = append(slices.Clone(d.quick), d.textEntries(allSectionLimit, groupText)...)
	}
	d.labelMatches(entries)
	d.entries = entries
	d.rows, d.rowEntry = d.rowsOf(entries)
	fresh := renewed && !d.loading && d.resultsOf != d.searched
	switch {
	case d.moved:
		d.results.Clamp(d.rows)
	case fresh:
		d.results.Reset(d.rows)
	default:
		d.results.SelectFirst(d.rows)
	}
	if fresh {
		d.resultsOf = d.searched
	}
}

func (d *Dialog) textEntries(limit int, group string) []entry {
	var out []entry
fill:
	for _, f := range d.text.files {
		for i, m := range f.Matches {
			if len(out) == limit {
				break fill
			}
			out = append(out, entry{kind: kindMatch, group: group, file: f, match: i, path: f.Path, label: lineLabel(m)})
		}
	}
	extra := d.text.total - len(out)
	switch {
	case extra <= 0:
	case group == "":
		out = append(out, entry{kind: kindHint, label: "Showing the first " + strconv.Itoa(len(out)) + " of " + strconv.Itoa(d.text.total) + " matches"})
	default:
		out = append(out, entry{kind: kindMore, group: group, tab: TabText, label: moreLabel(extra, "match", "matches")})
	}
	return out
}

func moreLabel(n int, one, many string) string {
	noun := many
	if n == 1 {
		noun = one
	}
	return "Show " + strconv.Itoa(n) + " more " + noun
}
