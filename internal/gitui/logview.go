package gitui

import (
	"fmt"
	"strings"
	"time"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const (
	logNearEnd      = 60
	defaultLogSplit = 64
	filterDelay     = 300 * time.Millisecond
	filterWidth     = 150
	branchWidth     = 150
	authorWidth     = 120
	dateWidth       = 110
	hashWidth       = 64
	maxBadges       = 3
	focusPageLimit  = 20
	allBranches     = "All Branches"
	currentBranch   = "Current Branch"
	minHashFilter   = 4
	maxHashFilter   = 40
)

type logFilter struct {
	text   string
	branch string
	author string
	path   string
}

type logState struct {
	g          *Git
	fixedPath  string
	started    bool
	split      float32
	selected   int
	list       ui.ListState
	filter     logFilter
	applied    logFilter
	timer      *time.Timer
	commits    []git.Commit
	graph      []git.GraphRow
	loading    bool
	done       bool
	err        error
	gen        int
	focus      string
	focusPages int
	details    commitDetails
}

func newLogState(g *Git, fixedPath string) *logState {
	s := &logState{g: g, fixedPath: fixedPath, split: defaultLogSplit, selected: -1}
	s.filter.branch = s.defaultBranch()
	s.applied = s.filter
	s.details.selected = -1
	return s
}

func (s *logState) defaultBranch() string {
	if s.fixedPath != "" {
		return currentBranch
	}
	return allBranches
}

func (s *logState) stopTimer() {
	if s.timer != nil {
		s.timer.Stop()
	}
}

func (s *logState) filtered() bool {
	f := s.applied
	return f.text != "" || f.author != "" || f.path != "" || f.branch != s.defaultBranch()
}

func (s *logState) showsGraph() bool {
	f := s.applied
	return s.fixedPath == "" && f.text == "" && f.author == "" && f.path == ""
}

func (s *logState) options(skip int) git.LogOptions {
	f := s.applied
	o := git.LogOptions{Max: s.g.logPageSize(), Skip: skip, Author: f.author, Grep: f.text}
	switch f.branch {
	case allBranches:
		o.All = true
	case currentBranch, "":
	default:
		o.Revs = []string{f.branch}
	}
	if s.fixedPath != "" {
		o.Paths, o.Follow = []string{s.fixedPath}, true
	} else if f.path != "" {
		o.Paths = []string{f.path}
	}
	return o
}

func isHashLike(s string) bool {
	if len(s) < minHashFilter || len(s) > maxHashFilter {
		return false
	}
	for _, r := range s {
		if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
			return false
		}
	}
	return true
}

func (s *logState) reload() {
	s.gen++
	s.commits, s.graph, s.done, s.err, s.loading = nil, nil, false, nil, false
	s.selected = -1
	s.details = commitDetails{selected: -1}
	s.loadPage()
}

func (s *logState) loadPage() {
	g := s.g
	if s.loading || s.done || !g.ready() {
		return
	}
	s.loading = true
	gen, skip, r, text := s.gen, len(s.commits), g.repo, s.applied.text
	opts := s.options(skip)
	go func() {
		page, err := r.Log(g.ctx, opts)
		full := len(page) == opts.Max
		if err == nil && skip == 0 && isHashLike(text) {
			if hit, herr := r.Log(g.ctx, git.LogOptions{Revs: []string{text}, Max: 1}); herr == nil {
				page = prependMissing(hit, page)
			}
		}
		g.post(func() {
			if gen != s.gen {
				return
			}
			s.loading = false
			if err != nil {
				s.err = err
				return
			}
			s.err = nil
			s.commits = append(s.commits, page...)
			s.done = !full
			s.graph = nil
			if s.showsGraph() {
				s.graph = git.Lanes(s.commits)
			}
			s.applyFocus()
		})
	}()
}

func prependMissing(hits, page []git.Commit) []git.Commit {
	for _, h := range hits {
		found := false
		for _, c := range page {
			if c.OID == h.OID {
				found = true
				break
			}
		}
		if !found {
			page = append([]git.Commit{h}, page...)
		}
	}
	return page
}

func (s *logState) scheduleApply() {
	if s.timer != nil {
		s.timer.Stop()
	}
	s.timer = time.AfterFunc(filterDelay, func() { s.g.post(s.applyFilter) })
}

func (s *logState) applyFilter() {
	if s.filter == s.applied {
		return
	}
	s.applied = s.filter
	s.reload()
}

func (s *logState) resetFilters() {
	s.filter = logFilter{branch: s.defaultBranch()}
	s.applyFilter()
}

func (s *logState) focusCommit(oid string) {
	s.focus, s.focusPages = oid, 0
	if s.filtered() {
		s.resetFilters()
	}
	if !s.started {
		s.started = true
		s.reload()
		return
	}
	s.applyFocus()
}

func (s *logState) applyFocus() {
	if s.focus == "" {
		return
	}
	for i, c := range s.commits {
		if strings.HasPrefix(c.OID, s.focus) {
			s.selected, s.focus = i, ""
			s.list.ScrollTo(i, ui.Center)
			return
		}
	}
	if s.done || s.focusPages >= focusPageLimit {
		s.g.h.Notify("Commit " + shortHash(s.focus) + " is not in the loaded log.")
		s.focus = ""
		return
	}
	s.focusPages++
	s.loadPage()
}

func (s *logState) selectedCommit() (git.Commit, bool) {
	if s.selected < 0 || s.selected >= len(s.commits) {
		return git.Commit{}, false
	}
	return s.commits[s.selected], true
}

func commitCount(n int, complete bool) string {
	more := ""
	if !complete {
		more = "+"
	}
	noun := "commits"
	if n == 1 && complete {
		noun = "commit"
	}
	return fmt.Sprintf("%d%s %s", n, more, noun)
}

func (g *Git) LogPanel(c *ui.Context) {
	ui.Column(c).Fill().Children(func() {
		if g.gate(c) {
			g.log.view(c)
		}
	})
}

func (s *logState) view(c *ui.Context) {
	if !s.started {
		s.started = true
		s.reload()
	}
	s.filterBar(c)
	ui.SplitPercent(c, &s.split, func() { s.listView(c) }, func() { s.detailsView(c) }).Grow(1).FillWidth().MinHeight(0)
}

func (s *logState) filterBar(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).FillWidth().Height(toolbarHeight+4).Shrink(0).Padding(0, rowPadding).Gap(6).AlignItems(ui.Center).
		BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
		changed := ui.TextInput(c.Key("text"), &s.filter.text).Placeholder("Text or hash").Label("Filter by Text").
			Width(filterWidth).Changed()
		if ui.Select(c.Key("branch"), &s.filter.branch, s.g.branchOptions()).Label("Filter by Branch").Width(branchWidth).Changed() {
			s.applyFilter()
		}
		changed = ui.TextInput(c.Key("author"), &s.filter.author).Placeholder("User").Label("Filter by User").
			Width(filterWidth).Changed() || changed
		if s.fixedPath == "" {
			changed = ui.TextInput(c.Key("path"), &s.filter.path).Placeholder("Path").Label("Filter by Path").
				Width(filterWidth).Changed() || changed
		}
		if changed {
			s.scheduleApply()
		}
		if s.filter != (logFilter{branch: s.defaultBranch()}) && iconButton(c, "Reset Filters", icons.Close, false).Clicked() {
			s.resetFilters()
		}
		ui.Box(c).Grow(1)
		if s.loading {
			ui.Spinner(c).Size(iconSize, iconSize).Label("Loading commits")
		}
		muted(c, commitCount(len(s.commits), s.done)).SingleLine()
		if iconButton(c, "Refresh Log", icons.Refresh, false).Clicked() {
			s.reload()
		}
	})
}

func (g *Git) branchOptions() []string {
	opts := []string{allBranches, currentBranch}
	for _, b := range g.snap.branches {
		if !strings.HasSuffix(b.Name, "/HEAD") {
			opts = append(opts, b.Name)
		}
	}
	return opts
}

func (g *Git) remoteSet() map[string]bool {
	set := map[string]bool{}
	for _, r := range g.snap.remotes {
		set[r.Name] = true
	}
	return set
}

func (s *logState) listView(c *ui.Context) {
	switch {
	case len(s.commits) == 0 && s.err != nil:
		errorState(c, "Cannot load the log", s.err, s.reload)
		return
	case len(s.commits) == 0 && (s.loading || !s.done):
		loadingState(c, "Loading commits")
		return
	case len(s.commits) == 0 && s.filtered():
		emptyState(c, "No commits match the filters", "Change or reset the filters.")
		return
	case len(s.commits) == 0:
		emptyState(c, "No commits yet", "Commit your changes to start the history.")
		return
	}
	commits := s.commits
	s.list.Key = func(i int) any { return commits[i].OID }
	s.list.Label = func(i int) string { return commits[i].Subject }
	s.list.Selected = &s.selected
	s.list.RowHeight = rowHeight
	s.list.NearEnd = logNearEnd
	s.list.EndReached = func(int) { s.loadPage() }
	var gw float32
	if s.graph != nil {
		gw = graphWidth(s.graph)
	}
	now, remotes := c.Now(), s.g.remoteSet()
	ui.Column(c).Fill().Children(func() {
		list := ui.List(c.Key("log"), &s.list, len(commits)).Grow(1).FillWidth().MinHeight(0).Label("Commits")
		list.Rows(func(row ui.ListRow) { s.row(row, commits[row.Index], gw, now, remotes) })
		if list.Submitted() {
			if cm, ok := s.selectedCommit(); ok {
				s.open(cm)
			}
		}
		if s.err != nil {
			ui.Row(c).FillWidth().Padding(4, rowPadding).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, s.err.Error()).FontSize(smallFont).TextColor(c.Theme().Danger).Grow(1).Shrink(1).MinWidth(0)
				if ui.Button(c, "Retry").Clicked() {
					s.err = nil
					s.loadPage()
				}
			})
		}
	})
	if cm, ok := s.selectedCommit(); ok && s.details.oid != cm.OID {
		s.loadDetails(cm)
	}
}

func refColor(t *ui.Theme, k refKind) ui.Color {
	switch k {
	case refHead:
		return t.Warning
	case refLocal:
		return t.Success
	case refRemote:
		return t.Accent
	}
	return t.TextMuted
}

func (s *logState) row(row ui.ListRow, cm git.Commit, gw float32, now time.Time, remotes map[string]bool) {
	c := row.Context
	t := c.Theme()
	i := row.Index
	merge := len(cm.Parents) > 1
	el := ui.Row(c).Height(rowHeight).FillWidth().Gap(6).PaddingEnd(rowPadding).AlignItems(ui.Center)
	el.Children(func() {
		if gw > 0 && i < len(s.graph) {
			gr := s.graph[i]
			ui.Box(c).Width(gw).Height(rowHeight).Shrink(0).Draw(func(p *ui.Painter, r ui.Rect) {
				drawGraphRow(p, r, t, gr, merge)
			})
		} else {
			ui.Box(c).Width(rowPadding).Shrink(0)
		}
		badges := refBadges(cm.Refs, remotes)
		for j, b := range badges {
			if j == maxBadges {
				badge(c, fmt.Sprintf("+%d", len(badges)-maxBadges), t.TextMuted)
				break
			}
			badge(c, b.name, refColor(t, b.kind))
		}
		subject := ui.Text(c, cm.Subject).FontSize(bodyFont).SingleLine().Grow(1).Shrink(1).MinWidth(0)
		if merge {
			subject.TextColor(t.TextMuted)
		}
		muted(c, cm.Author).SingleLine().Width(authorWidth).Shrink(0)
		muted(c, formatTime(cm.AuthorTime, now)).SingleLine().Width(dateWidth).Shrink(0)
		muted(c, shortHash(cm.OID)).SingleLine().Width(hashWidth).Shrink(0)
	})
	el.ContextMenu(func(m *ui.Menu) { s.commitMenu(c, m, cm) })
}
