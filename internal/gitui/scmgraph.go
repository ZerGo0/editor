package gitui

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/badge"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/list"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const (
	graphAuto      = ""
	graphNearEnd   = 40
	fileRowLoading = -1
	commitRow      = -2
)

type graphFiles struct {
	files   []git.ChangedFile
	loading bool
	err     error
}

type graphEntry struct {
	commit int
	file   int
}

type graphState struct {
	started  bool
	filter   string
	commits  []git.Commit
	rows     []git.GraphRow
	incoming map[string]bool
	outgoing map[string]bool
	loading  bool
	done     bool
	err      error
	gen      int
	list     ui.ListState
	cursor   int
	expanded map[string]*graphFiles
	upstream string
}

func (g *Git) graphOptions(skip int) git.LogOptions {
	s := &g.graph
	o := git.LogOptions{Max: g.logPageSize(), Skip: skip}
	switch s.filter {
	case allBranches:
		o.All = true
	case graphAuto:
		o.Revs = []string{headRev}
		if s.upstream != "" {
			o.Revs = append(o.Revs, s.upstream)
		}
	case currentBranch:
		o.Revs = []string{headRev}
	default:
		o.Revs = []string{s.filter}
	}
	return o
}

func (g *Git) reloadGraph() {
	s := &g.graph
	s.gen++
	s.commits, s.rows, s.done, s.err, s.loading = nil, nil, false, nil, false
	s.incoming, s.outgoing = nil, nil
	s.expanded = map[string]*graphFiles{}
	s.cursor = -1
	if st := g.snap.status; st != nil {
		s.upstream = st.Upstream
	}
	g.loadGraphPage()
}

func (g *Git) loadGraphPage() {
	s := &g.graph
	if s.loading || s.done || !g.ready() {
		return
	}
	s.loading = true
	gen, skip, r := s.gen, len(s.commits), g.repo
	opts := g.graphOptions(skip)
	if skip == 0 {
		applog.Go(func() {
			in, out := syncSets(g, r)
			g.post(func() {
				if gen == s.gen {
					s.incoming, s.outgoing = in, out
				}
			})
		})
	}
	applog.Go(func() {
		page, err := r.Log(g.ctx, opts)
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
			s.done = len(page) < opts.Max
			s.rows = git.Lanes(s.commits)
		})
	})
}

func syncSets(g *Git, r repo) (in, out map[string]bool) {
	toSet := func(commits []git.Commit, err error) map[string]bool {
		if err != nil {
			return nil
		}
		set := map[string]bool{}
		for _, c := range commits {
			set[c.OID] = true
		}
		return set
	}
	incoming, err := r.IncomingCommits(g.ctx)
	if errors.Is(err, git.ErrNoUpstream) {
		return nil, nil
	}
	return toSet(incoming, err), toSet(r.OutgoingCommits(g.ctx))
}

func (g *Git) graphEntries() []graphEntry {
	s := &g.graph
	out := make([]graphEntry, 0, len(s.commits))
	for i, cm := range s.commits {
		out = append(out, graphEntry{commit: i, file: commitRow})
		f, ok := s.expanded[cm.OID]
		switch {
		case !ok:
		case f.loading || f.err != nil:
			out = append(out, graphEntry{commit: i, file: fileRowLoading})
		default:
			for j := range f.files {
				out = append(out, graphEntry{commit: i, file: j})
			}
		}
	}
	return out
}

func (g *Git) toggleCommitFiles(cm git.Commit) {
	s := &g.graph
	if _, ok := s.expanded[cm.OID]; ok {
		delete(s.expanded, cm.OID)
		return
	}
	f := &graphFiles{loading: true}
	s.expanded[cm.OID] = f
	gen, r := s.gen, g.repo
	applog.Go(func() {
		files, err := r.CommitFiles(g.ctx, cm.OID)
		g.post(func() {
			if gen == s.gen {
				f.files, f.err, f.loading = files, err, false
			}
		})
	})
}

func (g *Git) graphPane(c *ui.Context) {
	s := &g.graph
	if g.snap.status == nil {
		g.graphSkeleton(c)
		return
	}
	if !s.started {
		s.started = true
		g.reloadGraph()
	}
	switch {
	case len(s.commits) == 0 && s.err != nil:
		errorState(c, "Cannot load the graph", s.err, g.reloadGraph)
		return
	case len(s.commits) == 0 && !s.done:
		g.graphSkeleton(c)
		return
	case len(s.commits) == 0:
		emptyState(c, "No commits yet", "Commit your changes to start the history.")
		return
	}
	entries := g.graphEntries()
	commits := s.commits
	gw := graphWidth(s.rows)
	remotes := g.remoteSet()
	s.list.Key = func(i int) any {
		e := entries[i]
		if e.file == commitRow {
			return commits[e.commit].OID
		}
		return fmt.Sprintf("%s:%d", commits[e.commit].OID, e.file)
	}
	s.list.Label = func(i int) string { return g.graphLabel(entries[i]) }
	s.list.Selected = &s.cursor
	s.list.RowHeight = token.RowHeight.Px()
	s.list.NearEnd = graphNearEnd
	s.list.EndReached = func(int) { g.loadGraphPage() }
	l := ui.List(c.Key("graph"), &s.list, len(entries)).Grow(1).FillWidth().MinHeight(0).Label("Graph").Background(ui.Transparent)
	width := list.ObserveWidth(c, l)
	l.Rows(func(row ui.ListRow) {
		rc := row.Context
		st := list.State{Selected: row.Selected(), Focused: row.ListFocused(), Hovered: row.Hovered(), Width: width}
		e := entries[row.Index]
		cm := commits[e.commit]
		if e.file == commitRow {
			el := list.Row(rc, st, g.graphCommitItem(rc, commits, s.rows, e.commit, gw, remotes))
			feedback.HoverCard(rc, el, func() { g.commitCard(rc, cm, remotes) })
			return
		}
		list.Row(rc, st, g.graphFileItem(rc, e, gw))
	})
	if s.cursor >= 0 && s.cursor < len(entries) && l.Submitted() {
		e := entries[s.cursor]
		cm := commits[e.commit]
		if f := s.expanded[cm.OID]; e.file >= 0 && f != nil && e.file < len(f.files) {
			g.openDiffWith(commitSource(cm, f.files[e.file]), false, true)
		} else if e.file == commitRow {
			g.toggleCommitFiles(cm)
		}
	}
}

func (g *Git) graphLabel(e graphEntry) string {
	s := &g.graph
	cm := s.commits[e.commit]
	if e.file == commitRow {
		return cm.Subject
	}
	if f := s.expanded[cm.OID]; f != nil && e.file >= 0 && e.file < len(f.files) {
		return f.files[e.file].Path
	}
	return "Loading files"
}

func graphLanes(c *ui.Context, commits []git.Commit, rows []git.GraphRow, i int, gw float32, through bool) {
	p := token.Of(c)
	bg := p.Layer.SideBar
	merge := len(commits[i].Parents) > 1
	ui.Box(c).Width(gw).Height(token.RowHeight.Px()).Shrink(0).AccessHidden(true).Draw(func(pt *ui.Painter, r ui.Rect) {
		if i >= len(rows) {
			return
		}
		if through {
			drawGraphContinue(pt, r, p, rows[i])
			return
		}
		drawGraphRow(pt, r, p, bg, rows[i], merge)
	})
}

func (g *Git) graphCommitItem(c *ui.Context, commits []git.Commit, rows []git.GraphRow, i int, gw float32, remotes map[string]bool) list.Item {
	cm := commits[i]
	_, open := g.graph.expanded[cm.OID]
	return list.Item{
		Label:        cm.Subject,
		Description:  cm.Author,
		Muted:        len(cm.Parents) > 1,
		Open:         open,
		Pills:        g.graphPills(cm, remotes),
		LeadingWidth: gw,
		Leading:      func(c *ui.Context) { graphLanes(c, commits, rows, i, gw, false) },
		Menu:         func(m *ui.Menu) { g.graphCommitMenu(c, m, cm) },
		OnClick:      func() { g.toggleCommitFiles(cm) },
		OnDoubleClick: func() {
			g.openCommitTab(cm)
		},
	}
}

func (g *Git) graphPills(cm git.Commit, remotes map[string]bool) []list.Pill {
	var pills []list.Pill
	switch {
	case g.graph.incoming[cm.OID]:
		pills = append(pills, list.Pill{Icon: icons.Pull, Tone: token.ToneInfo, Tooltip: "Incoming commit"})
	case g.graph.outgoing[cm.OID]:
		pills = append(pills, list.Pill{Icon: icons.Push, Tone: token.ToneSuccess, Tooltip: "Outgoing commit"})
	}
	named := false
	byKind := map[refKind][]string{}
	var kinds []refKind
	for _, b := range refBadges(cm.Refs, remotes) {
		if b.kind != refHead && (b.kind != refRemote || b.name != g.graph.upstream) {
			continue
		}
		tone, icon := refTone(b.kind)
		if !named {
			named = true
			pills = append(pills, list.Pill{Text: b.name, Icon: icon, Tone: tone, Tooltip: b.name})
			continue
		}
		if len(byKind[b.kind]) == 0 {
			kinds = append(kinds, b.kind)
		}
		byKind[b.kind] = append(byKind[b.kind], b.name)
	}
	for _, k := range kinds {
		tone, icon := refTone(k)
		names := byKind[k]
		count := ""
		if len(names) > 1 {
			count = strconv.Itoa(len(names))
		}
		pills = append(pills, list.Pill{Text: count, Short: count, Icon: icon, Tone: tone, Tooltip: strings.Join(names, ", ")})
	}
	return pills
}

func refTone(k refKind) (token.Tone, *ui.SVG) {
	switch k {
	case refHead:
		return token.ToneInfo, icons.Check
	case refLocal:
		return token.ToneSuccess, icons.GitBranch
	case refRemote:
		return token.ToneNeutral, icons.Github
	}
	return token.ToneWarning, icons.Tag
}

func refTags(c *ui.Context, refs []string, remotes map[string]bool, limit int) {
	badges := refBadges(refs, remotes)
	for j, b := range badges {
		if limit > 0 && j == limit {
			wholeTag(c, fmt.Sprintf("+%d", len(badges)-limit), badge.TagOptions{Tooltip: refNames(badges[limit:])})
			return
		}
		tone, icon := refTone(b.kind)
		wholeTag(c, refLabel(b.name), badge.TagOptions{Tone: tone, Icon: icon, Tooltip: b.name})
	}
}

func refNames(badges []refBadge) string {
	names := make([]string, len(badges))
	for i, b := range badges {
		names[i] = b.name
	}
	return strings.Join(names, ", ")
}

func (g *Git) commitCard(c *ui.Context, cm git.Commit, remotes map[string]bool) {
	p := token.Of(c)
	token.TextStrong.Apply(ui.Text(c, cm.Subject)).TextColor(p.Text.Primary).Wrap()
	if body := strings.TrimSpace(cm.Body); body != "" {
		paragraph(c, body)
	}
	secondary(c, fmt.Sprintf("%s <%s>", cm.Author, cm.AuthorEmail)).Wrap()
	secondary(c, formatTime(cm.AuthorTime, c.Now())+" \u2022 "+shortHash(cm.OID))
	if len(cm.Refs) > 0 {
		ui.Row(c).Gap(token.GapInline.Px()).Wrap().Children(func() { refTags(c, cm.Refs, remotes, 0) })
	}
	switch {
	case g.graph.incoming[cm.OID]:
		secondary(c, "Incoming: on the upstream, not in your branch yet.")
	case g.graph.outgoing[cm.OID]:
		secondary(c, "Outgoing: in your branch, not pushed yet.")
	}
}

func (g *Git) graphFileItem(c *ui.Context, e graphEntry, gw float32) list.Item {
	s := &g.graph
	cm := s.commits[e.commit]
	f := s.expanded[cm.OID]
	lanes := func(c *ui.Context) { graphLanes(c, g.graph.commits, g.graph.rows, e.commit, gw, true) }
	if f == nil || e.file < 0 || e.file >= len(f.files) {
		it := list.Item{Label: "Loading files", Muted: true, Leading: lanes}
		if f != nil && f.err != nil {
			it.Label, it.Tone = f.err.Error(), token.ToneError
		}
		return it
	}
	file := f.files[e.file]
	name, dir := splitPath(file.Path)
	src := commitSource(cm, file)
	return list.Item{
		Label:       name,
		Description: dir,
		Git:         gitStatus(file.Status),
		Strike:      file.Status == codeDeleted,
		Tooltip:     file.Path + " \u2022 " + statusName(file.Status),
		Leading: func(c *ui.Context) {
			lanes(c)
			ui.Box(c).Width(token.IndentStep.Px()).Shrink(0)
			icons.FileIcon(c, name, false, false, token.IconSize.Px())
		},
		Menu:          func(m *ui.Menu) { g.commitFileMenu(c, m, cm, file) },
		OnClick:       func() { g.openDiffWith(src, true, false) },
		OnDoubleClick: func() { g.openDiffWith(src, false, true) },
	}
}

func (g *Git) graphCommitMenu(c *ui.Context, m *ui.Menu, cm git.Commit) {
	if m.Item("Open Changes").Chosen() {
		g.openCommitTab(cm)
	}
	m.Separator()
	g.commitActions(c, m, cm)
	m.Separator()
	if m.Item("Show in Git Log").Chosen() {
		g.ShowCommit(cm.OID)
	}
}

func (g *Git) graphActions(c *ui.Context) {
	busy := g.busy != ""
	if iconButton(c, "Fetch", icons.Fetch, busy) {
		g.fetch()
	}
	if iconButton(c, "Pull", icons.Pull, busy) {
		g.pull()
	}
	if iconButton(c, "Push", icons.Push, busy) {
		g.pushOrOpen()
	}
	if iconButton(c, "Refresh Graph", icons.Refresh, g.graph.loading) {
		g.reloadGraph()
	}
	action.MenuIconButton(c, icons.More, "Graph Actions", action.IconOptions{}, g.graphMenu)
}

func (g *Git) graphMenu(m *ui.Menu) {
	m.Submenu("Show Branches", func(m *ui.Menu) {
		choices := append([]string{graphAuto, allBranches, currentBranch}, g.branchNames()...)
		for _, b := range choices {
			label := b
			if b == graphAuto {
				label = "Auto (Current and Upstream)"
			}
			if m.Item(label).Checked(g.graph.filter == b).Chosen() && g.graph.filter != b {
				g.graph.filter = b
				g.reloadGraph()
			}
		}
	})
	m.Separator()
	if m.Item("Show Git Log").Chosen() {
		g.showLog()
	}
}

func (g *Git) branchNames() []string {
	var out []string
	for _, b := range g.snap.branches {
		if !strings.HasSuffix(b.Name, "/HEAD") {
			out = append(out, b.Name)
		}
	}
	return out
}
