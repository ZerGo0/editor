package issueui

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/ZerGo0/editor/internal/github"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const (
	sectionSearchID   = "search"
	sectionIDPrefix   = "query-"
	searchLabel       = "Search Results"
	searchPlaceholder = "Search issues (GitHub syntax)"
	repoQualifier     = "repo:"
	repoQueryPrefix   = "repo:${owner}/${repository} "
	noMilestoneTitle  = "No Milestone"
	toolbarPad        = 8
	groupIndent       = 10
	maxSideLabels     = 2
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
		i.sideToolbar(c)
		rows := i.sideRows()
		s.rows = rows
		s.list.Key = func(n int) any { return rows[n].key() }
		s.list.Label = func(n int) string { return rows[n].label() }
		ui.List(c, &s.list, len(rows), func(n int) { i.sideRowView(c, rows[n]) }).Grow(1).FillWidth().MinHeight(0).Label("Issues")
		if s.list.Shortcut(c, 0, ui.KeyEnter) && s.row >= 0 && s.row < len(rows) {
			i.activate(rows[s.row])
		}
	})
}

func (i *Issues) sideToolbar(c *ui.Context) {
	s := i.side
	ui.Row(c).Gap(spacingSmall).AlignItems(ui.Center).Padding(0, toolbarPad, toolbarPad, toolbarPad).Shrink(0).FillWidth().Children(func() {
		field := ui.SearchField(c.Key("search"), &s.field).Placeholder(searchPlaceholder).Label("Search issues").Grow(1).MinWidth(0)
		switch {
		case field.Submitted():
			i.runSearch(s.field)
		case field.Changed() && s.field == "" && s.search != nil:
			i.runSearch("")
		}
		if iconButton(c, "create", "Create Issue", icons.Plus).Clicked() {
			i.openCreate(createDraft{})
		}
		if iconButton(c, "refresh", "Refresh Issues", icons.Refresh).Clicked() {
			i.refresh()
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

func (i *Issues) sideRowView(c *ui.Context, r sideRow) {
	t := c.Theme()
	switch r.kind {
	case rowHeader:
		row := ui.Row(c).Gap(spacingSmall).AlignItems(ui.Center).Padding(rowPadY, rowPadX).FillWidth()
		row.Children(func() {
			svg := icons.ChevronRight
			if r.sec.open {
				svg = icons.ChevronDown
			}
			ui.Icon(c, svg).Size(iconSmall, iconSmall).TextColor(t.TextMuted).Shrink(0)
			ui.Text(c, r.sec.query.Label).FontSize(fontBody).Bold().SingleLine().MinWidth(0)
			if n := len(r.sec.items); n > 0 {
				label := strconv.Itoa(n)
				if r.sec.more {
					label += "+"
				}
				mutedPill(c, label)
			}
		})
		if row.RowClicked() {
			r.sec.open = !r.sec.open
		}
	case rowGroup:
		mutedText(c, r.text).Bold().SingleLine().Padding(rowPadY, rowPadX, 0, rowPadX+groupIndent)
	case rowIssue:
		row := i.issueRow(c, *r.issue)
		if row.RowClicked() {
			i.openIssue(r.issue.Number, r.issue)
		}
	case rowMore:
		row := ui.Row(c).Padding(rowPadY, rowPadX, rowPadY, rowPadX+iconMedium).FillWidth()
		row.Children(func() {
			ui.Button(c, "Load more").OnClick(func() { i.loadSection(r.sec, true) })
		})
		if row.RowClicked() {
			i.loadSection(r.sec, true)
		}
	case rowLoading:
		loadingRow(c, r.text)
	case rowNote:
		mutedText(c, r.text).Padding(rowPadY, rowPadX, rowPadY, rowPadX+iconMedium)
	case rowError:
		errorBlock(c, r.err, func() { i.retrySection(r.sec) })
	}
}

func (i *Issues) issueRow(c *ui.Context, issue github.Issue) ui.Element {
	t := c.Theme()
	now := i.env.Now()
	row := ui.Row(c).Gap(spacingMedium).AlignItems(ui.Start).Padding(rowPadY, rowPadX).FillWidth()
	row.ContextMenu(func(m *ui.Menu) { i.issueMenu(c, m, issue) })
	row.Children(func() {
		svg := icons.Dot
		if !isOpen(issue) {
			svg = icons.Check
		}
		ui.Icon(c, svg).Size(iconMedium, iconMedium).TextColor(stateColor(t, issue)).Shrink(0).Margin(rowIconGap, 0, 0, 0).Label(stateLabel(issue))
		ui.Column(c).Grow(1).MinWidth(0).Gap(rowIconGap).Children(func() {
			ui.Text(c, issue.Title).FontSize(fontBody).SingleLine()
			ui.Row(c).Gap(spacingSmall + rowIconGap).AlignItems(ui.Center).Children(func() {
				mutedText(c, "#"+strconv.Itoa(issue.Number)).SingleLine().Shrink(0)
				if issue.Author != "" {
					mutedText(c, issue.Author).SingleLine().Shrink(1).MinWidth(0)
				}
				if when := relativeTime(now, issue.Updated); when != "" {
					mutedText(c, when).SingleLine().Shrink(0)
				}
				for n, l := range issue.Labels {
					if n == maxSideLabels {
						mutedPill(c, "+"+strconv.Itoa(len(issue.Labels)-maxSideLabels))
						break
					}
					labelPill(c, l)
				}
				if issue.Comments > 0 {
					ui.Icon(c, icons.Comment).Size(iconSmall, iconSmall).TextColor(t.TextMuted).Shrink(0).Label(fmt.Sprintf("%d comments", issue.Comments))
					mutedText(c, strconv.Itoa(issue.Comments)).Shrink(0)
				}
			})
		})
	})
	return row
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
