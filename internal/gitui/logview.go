package gitui

import (
	"fmt"
	"strings"
	"time"

	"github.com/ZerGo0/editor/internal/design"
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/field"
	"github.com/ZerGo0/editor/internal/design/list"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const (
	logNearEnd      = 60
	defaultLogSplit = 64
	filterDelay     = 300 * time.Millisecond
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
	toolbarRow(c, "Log Filters", func() {
		changed := false
		filterField := func(label, placeholder, id string, value *string) {
			ui.Row(c).Width(token.FieldWidthSmall.Px()).Shrink(0).Children(func() {
				if field.TextInput(c, label, value, field.InputOptions{Placeholder: placeholder, ID: id}).Changed() {
					changed = true
				}
			})
		}
		filterField("Filter by Text", "Text or hash", "text", &s.filter.text)
		branch := s.filter.branch
		ui.Row(c).Width(token.FieldWidthSmall.Px()).Shrink(0).Children(func() {
			action.Select(c, "Filter by Branch", &s.filter.branch, s.g.branchOptions(), action.SelectOptions{ID: "branch"})
		})
		if s.filter.branch != branch {
			s.applyFilter()
		}
		filterField("Filter by User", "User", "author", &s.filter.author)
		if s.fixedPath == "" {
			filterField("Filter by Path", "Path", "path", &s.filter.path)
		}
		if changed {
			s.scheduleApply()
		}
		if s.filter != (logFilter{branch: s.defaultBranch()}) && iconButton(c, "Reset Filters", icons.Close, false) {
			s.resetFilters()
		}
		design.Spacer(c)
		if s.loading {
			feedback.Spinner(c, "Loading commits")
		}
		secondary(c, commitCount(len(s.commits), s.done)).SingleLine().Shrink(0)
		if iconButton(c, "Refresh Log", icons.Refresh, false) {
			s.reload()
		}
	})
}

func (g *Git) branchOptions() []action.Option[string] {
	opts := []action.Option[string]{{Value: allBranches, Label: allBranches}, {Value: currentBranch, Label: currentBranch}}
	for _, b := range g.branchNames() {
		opts = append(opts, action.Option[string]{Value: b, Label: b})
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
	s.list.RowHeight = token.RowHeight.Px()
	s.list.NearEnd = logNearEnd
	s.list.EndReached = func(int) { s.loadPage() }
	var gw float32
	if s.graph != nil {
		gw = graphWidth(s.graph)
	}
	remotes := s.g.remoteSet()
	ui.Column(c).Fill().Children(func() {
		l := ui.List(c.Key("log"), &s.list, len(commits)).Grow(1).FillWidth().MinHeight(0).Label("Commits").Background(ui.Transparent)
		l.Rows(func(row ui.ListRow) {
			rc := row.Context
			st := list.State{Selected: row.Selected(), Focused: row.ListFocused(), Hovered: row.Hovered()}
			list.Row(rc, st, s.item(rc, row.Index, commits[row.Index], gw, remotes))
		})
		if l.Submitted() {
			if cm, ok := s.selectedCommit(); ok {
				s.open(cm)
			}
		}
		if s.err != nil {
			ui.Row(c).FillWidth().Padding(token.GapStack.Px(), token.PadView.Px()).Gap(token.GapControl.Px()).AlignItems(ui.Center).Children(func() {
				errorText(c, s.err.Error()).Grow(1).Shrink(1).MinWidth(0)
				if smallButton(c, "Retry", false) {
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

func (s *logState) item(c *ui.Context, i int, cm git.Commit, gw float32, remotes map[string]bool) list.Item {
	merge := len(cm.Parents) > 1
	p := token.Of(c)
	return list.Item{
		Label: cm.Subject,
		Muted: merge,
		Leading: func(c *ui.Context) {
			if gw == 0 || i >= len(s.graph) {
				return
			}
			gr := s.graph[i]
			ui.Box(c).Width(gw).Height(token.RowHeight.Px()).Shrink(0).AccessHidden(true).Draw(func(pt *ui.Painter, r ui.Rect) {
				drawGraphRow(pt, r, p, p.Layer.Panel, gr, merge)
			})
		},
		Trailing: func(c *ui.Context) {
			refTags(c, cm.Refs, remotes, maxBadges)
			secondary(c, cm.Author).SingleLine().Width(token.ColumnMedium.Px()).Shrink(0)
			secondary(c, formatTime(cm.AuthorTime, c.Now())).SingleLine().Width(token.ColumnMedium.Px()).Shrink(0)
			secondary(c, shortHash(cm.OID)).SingleLine().Width(token.ColumnSmall.Px()).Shrink(0)
		},
		Menu: func(m *ui.Menu) { s.commitMenu(c, m, cm) },
	}
}
