package issueui

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/badge"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/field"
	"github.com/ZerGo0/editor/internal/design/list"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/github"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const (
	sectionSearchID    = "search"
	sectionIDPrefix    = "query-"
	searchLabel        = "Search Results"
	searchPlaceholder  = "Search issues (GitHub syntax)"
	repoQualifier      = "repo:"
	repoQueryPrefix    = "repo:${owner}/${repository} "
	noMilestoneTitle   = "No Milestone"
	issueMetaSeparator = " · "
)

type section struct {
	id      string
	query   Query
	open    bool
	items   []github.Issue
	page    int
	more    bool
	total   int
	loading bool
	loaded  bool
	stale   bool
	err     error
	gen     int
}

type sideState struct {
	sections []*section
	search   *section
	field    string
	list     ui.ListState
	row      int
	rows     []sideRow
}

func newSideState(queries []Query) *sideState {
	s := &sideState{row: -1}
	for n, q := range queries {
		s.sections = append(s.sections, &section{id: sectionIDPrefix + strconv.Itoa(n), query: q, open: n == 0})
	}
	s.list.Selected = &s.row
	return s
}

func (s *sideState) all() []*section {
	if s.search != nil {
		return append([]*section{s.search}, s.sections...)
	}
	return s.sections
}

type rowKind int

const (
	rowHeader rowKind = iota
	rowGroup
	rowIssue
	rowMore
	rowLoading
	rowNote
	rowError
)

type sideRow struct {
	kind  rowKind
	sec   *section
	issue *github.Issue
	text  string
	err   error
}

func (r sideRow) key() any {
	if r.kind == rowIssue {
		return fmt.Sprintf("issue:%s:%d", r.sec.id, r.issue.Number)
	}
	return fmt.Sprintf("%d:%s:%s", r.kind, r.sec.id, r.text)
}

func (r sideRow) label() string {
	switch r.kind {
	case rowHeader:
		return r.sec.query.Label
	case rowIssue:
		return fmt.Sprintf("%s, #%d by %s", r.issue.Title, r.issue.Number, r.issue.Author)
	case rowMore:
		return "Load more"
	}
	return r.text
}

func (i *Issues) expandQuery(query string) string {
	return expandTemplate(query, map[string]string{varUser: i.user, varOwner: i.repo.Owner, varRepository: i.repo.Name})
}

func appendUnique(items, more []github.Issue) []github.Issue {
	for _, issue := range more {
		if !slices.ContainsFunc(items, func(x github.Issue) bool { return x.Number == issue.Number }) {
			items = append(items, issue)
		}
	}
	return items
}

func (i *Issues) loadSection(s *section, more bool) {
	s.gen++
	gen := s.gen
	s.loading, s.stale = true, false
	page := 1
	if more {
		page = s.page + 1
	}
	query := i.expandQuery(s.query.Query)
	be := i.be
	go func() {
		result, err := be.SearchIssues(i.ctx, query, page)
		i.post(func() {
			if gen != s.gen {
				return
			}
			s.loading = false
			if err != nil {
				s.err = err
				i.authFailure(err)
				return
			}
			s.err, s.loaded, s.page, s.more, s.total = nil, true, page, result.HasMore, result.Total
			if more {
				s.items = appendUnique(s.items, result.Issues)
				return
			}
			s.items = result.Issues
		})
	}()
}

func (i *Issues) runSearch(text string) {
	text = trimmed(text)
	if text == "" {
		i.side.search = nil
		return
	}
	query := text
	if !strings.Contains(text, repoQualifier) {
		query = repoQueryPrefix + text
	}
	s := &section{id: sectionSearchID, query: Query{Label: searchLabel, Query: query}, open: true}
	i.side.search = s
	i.loadSection(s, false)
}

func (i *Issues) ensureLoaded() {
	for _, s := range i.side.all() {
		if s.open && !s.loading && (s.stale || !s.loaded && s.err == nil) {
			i.loadSection(s, false)
		}
	}
}

type issueGroup struct {
	title string
	items []int
}

func groupByMilestoneTitle(items []github.Issue) []issueGroup {
	var groups []issueGroup
	var none issueGroup
	for n, issue := range items {
		title := milestoneTitle(issue)
		if title == "" {
			none.items = append(none.items, n)
			continue
		}
		at := slices.IndexFunc(groups, func(g issueGroup) bool { return g.title == title })
		if at < 0 {
			groups = append(groups, issueGroup{title: title})
			at = len(groups) - 1
		}
		groups[at].items = append(groups[at].items, n)
	}
	if len(none.items) > 0 {
		none.title = noMilestoneTitle
		groups = append(groups, none)
	}
	return groups
}

func (i *Issues) sideRows() []sideRow {
	var rows []sideRow
	for _, s := range i.side.all() {
		rows = append(rows, sideRow{kind: rowHeader, sec: s})
		if s.open {
			rows = append(rows, sectionRows(s)...)
		}
	}
	return rows
}

func sectionRows(s *section) []sideRow {
	switch {
	case s.err != nil && len(s.items) == 0:
		return []sideRow{{kind: rowError, sec: s, err: s.err}}
	case s.loading && len(s.items) == 0:
		return []sideRow{{kind: rowLoading, sec: s, text: "Loading..."}}
	case s.loaded && len(s.items) == 0:
		return []sideRow{{kind: rowNote, sec: s, text: "No issues found. Refresh to check again."}}
	}
	var rows []sideRow
	if s.query.groupsByMilestone() {
		for _, g := range groupByMilestoneTitle(s.items) {
			rows = append(rows, sideRow{kind: rowGroup, sec: s, text: g.title})
			for _, n := range g.items {
				rows = append(rows, sideRow{kind: rowIssue, sec: s, issue: &s.items[n]})
			}
		}
	} else {
		for n := range s.items {
			rows = append(rows, sideRow{kind: rowIssue, sec: s, issue: &s.items[n]})
		}
	}
	switch {
	case s.err != nil:
		rows = append(rows, sideRow{kind: rowError, sec: s, err: s.err})
	case s.loading:
		rows = append(rows, sideRow{kind: rowLoading, sec: s, text: "Loading..."})
	case s.more:
		rows = append(rows, sideRow{kind: rowMore, sec: s})
	}
	return rows
}

func (i *Issues) retrySection(s *section) {
	s.err = nil
	i.loadSection(s, s.loaded && len(s.items) > 0)
}

func (i *Issues) selectedIssue() *github.Issue {
	s := i.side
	if s == nil || s.row < 0 || s.row >= len(s.rows) || s.rows[s.row].kind != rowIssue {
		return nil
	}
	return s.rows[s.row].issue
}

func (i *Issues) SideView(c *ui.Context) {
	if i.closed {
		return
	}
	if !i.ready() {
		i.stateView(c)
		return
	}
	i.ensureLoaded()
	s := i.side
	ui.Column(c).Grow(1).FillWidth().MinHeight(0).Children(func() {
		i.sideSearch(c)
		rows := i.sideRows()
		s.rows = rows
		s.list.Key = func(n int) any { return rows[n].key() }
		s.list.Label = func(n int) string { return rows[n].label() }
		list.View(c, &s.list, len(rows), list.ViewOptions{Label: "Issues", Tree: true}, func(n int) list.Item { return i.sideItem(c, rows[n]) })
		if s.list.Shortcut(c, 0, ui.KeyEnter) && s.row >= 0 && s.row < len(rows) {
			i.activate(rows[s.row])
		}
		if s.row >= 0 && s.row < len(rows) && rows[s.row].kind == rowHeader && s.list.Handle.Focused(c) {
			sec := rows[s.row].sec
			switch {
			case !sec.open && c.Shortcut(0, ui.KeyRight):
				sec.open = true
			case sec.open && c.Shortcut(0, ui.KeyLeft):
				sec.open = false
			}
		}
	})
}

func (i *Issues) SideActions(c *ui.Context) {
	if !i.ready() {
		return
	}
	if action.IconButton(c, icons.Plus, "Create Issue", action.IconOptions{ID: "create"}).Clicked() {
		i.openCreate(createDraft{})
	}
	if action.IconButton(c, icons.Refresh, "Refresh Issues", action.IconOptions{ID: "refresh"}).Clicked() {
		i.refresh()
	}
}

func (i *Issues) sideSearch(c *ui.Context) {
	s := i.side
	ui.Column(c).FillWidth().Padding(token.GapInline.Px(), token.PadRow.Px(), token.GapControl.Px(), token.PadRow.Px()).Shrink(0).Children(func() {
		field := field.SearchInput(c, "Search issues", &s.field, field.SearchOptions{Placeholder: searchPlaceholder, ID: "search"})
		switch {
		case field.Submitted():
			i.runSearch(s.field)
		case field.Changed() && s.field == "" && s.search != nil:
			i.runSearch("")
		}
	})
}

func (i *Issues) activate(r sideRow) {
	switch r.kind {
	case rowHeader:
		r.sec.open = !r.sec.open
	case rowIssue:
		i.openIssue(r.issue.Number, r.issue)
	case rowMore:
		i.loadSection(r.sec, true)
	case rowError:
		i.retrySection(r.sec)
	}
}

func (i *Issues) sideItem(c *ui.Context, r sideRow) list.Item {
	switch r.kind {
	case rowHeader:
		it := list.Item{Label: r.sec.query.Label, Strong: true, Branch: true, Open: r.sec.open, Count: len(r.sec.items), OnClick: func() { r.sec.open = !r.sec.open }}
		return it
	case rowGroup:
		return list.Item{Depth: 1, Label: r.text, Strong: true, Muted: true}
	case rowIssue:
		return i.issueItem(c, r)
	case rowMore:
		return list.Item{Depth: 1, Label: r.label(), OnClick: func() { i.loadSection(r.sec, true) }}
	case rowLoading:
		return list.Item{Depth: 1, Muted: true, Label: r.text, Leading: func(c *ui.Context) { feedback.Spinner(c, r.text) }}
	case rowNote:
		return list.Item{Depth: 1, Muted: true, Label: r.text, Tooltip: r.text}
	case rowError:
		return list.Item{Depth: 1, Tone: token.ToneError, Label: errorText(r.err), Tooltip: errorText(r.err), Trailing: func(c *ui.Context) {
			ghostButton(c, retryLabel, "retry").OnClick(func() { i.retrySection(r.sec) })
		}}
	}
	return list.Item{}
}

func (i *Issues) issueItem(c *ui.Context, r sideRow) list.Item {
	pal := token.Of(c)
	issue := *r.issue
	depth := 1
	if r.sec.query.groupsByMilestone() {
		depth++
	}
	description := "#" + strconv.Itoa(issue.Number)
	if when := relativeTime(i.env.Now(), issue.Updated); when != "" {
		description += issueMetaSeparator + when
	}
	svg := icons.Dot
	if !isOpen(issue) {
		svg = icons.Check
	}
	return list.Item{
		Depth:       depth,
		Label:       issue.Title,
		Description: description,
		Tooltip:     issueTooltip(issue),
		Count:       issue.Comments,
		Leading: func(c *ui.Context) {
			ui.Icon(c, svg).Size(token.IconSize.Px(), token.IconSize.Px()).TextColor(pal.Tone(stateTone(issue))).Shrink(0).Label(stateLabel(issue))
		},
		Trailing: func(c *ui.Context) {
			if issue.Author != "" {
				badge.Avatar(c, issue.Author, token.AvatarSize)
			}
		},
		OnClick: func() { i.openIssue(issue.Number, r.issue) },
		Menu:    func(m *ui.Menu) { i.issueMenu(c, m, issue) },
	}
}

func issueTooltip(issue github.Issue) string {
	tip := fmt.Sprintf("%s\n#%d by %s", issue.Title, issue.Number, issue.Author)
	if len(issue.Labels) == 0 {
		return tip
	}
	names := make([]string, len(issue.Labels))
	for i, l := range issue.Labels {
		names[i] = l.Name
	}
	return tip + "\n" + strings.Join(names, ", ")
}

func (i *Issues) issueMenu(c *ui.Context, m *ui.Menu, issue github.Issue) {
	if m.Item("Open").Chosen() {
		i.openIssue(issue.Number, &issue)
	}
	if m.Item("Start Working").Disabled(i.working.busy != "").Chosen() {
		i.startWorking(issue.Number, &issue)
	}
	m.Separator()
	if m.Item("Open on GitHub").Disabled(issue.URL == "").Chosen() {
		openExternal(c, issue.URL)
	}
	if m.Item("Copy Link").Disabled(issue.URL == "").Chosen() {
		c.WriteClipboard(issue.URL)
	}
}
