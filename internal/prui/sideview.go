package prui

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/ZerGo0/editor/internal/github"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const (
	sectionCurrent  = "current"
	sectionChanges  = "changes"
	sectionReview   = "review"
	sectionAssigned = "assigned"
	sectionCreated  = "created"
	sectionAll      = "all"
	sectionLocal    = "local"
	sectionSearch   = "search"

	searchPlaceholder = "Search pull requests (GitHub syntax)"
	toolbarPad        = 8
	prRowGap          = 2
	selfQualifier     = "@me"
	sideViewName      = "Pull Requests"
	doubleClickWindow = time.Second
	notificationsName = "Notifications"
)

type section struct {
	id      string
	title   string
	query   string
	opts    github.ListOptions
	local   bool
	open    bool
	items   []github.PullRequest
	page    int
	more    bool
	loading bool
	loaded  bool
	stale   bool
	err     error
	gen     int
}

type sideState struct {
	current   *section
	changes   *section
	review    *section
	assigned  *section
	created   *section
	all       *section
	local     *section
	search    *section
	custom    []*section
	field     string
	list      ui.ListState
	row       int
	toggledAt time.Time
}

func newSideState() *sideState {
	s := &sideState{
		current:  &section{id: sectionCurrent, title: "Current Branch", open: true},
		changes:  &section{id: sectionChanges, title: "Changes in Pull Request", open: true},
		review:   &section{id: sectionReview, title: "Waiting for My Review", opts: github.ListOptions{ReviewRequested: selfQualifier}, open: true},
		assigned: &section{id: sectionAssigned, title: "Assigned to Me", opts: github.ListOptions{Assignee: selfQualifier}},
		created:  &section{id: sectionCreated, title: "Created by Me", opts: github.ListOptions{Author: selfQualifier}},
		all:      &section{id: sectionAll, title: "All Open"},
		local:    &section{id: sectionLocal, title: "Local Pull Request Branches", local: true},
		row:      -1,
	}
	s.list.Selected = &s.row
	return s
}

func (s *sideState) sections() []*section {
	var list []*section
	if s.search != nil {
		list = append(list, s.search)
	}
	list = append(list, s.local, s.review, s.assigned, s.created, s.all)
	return append(list, s.custom...)
}

type rowKind int

const (
	rowHeader rowKind = iota
	rowPR
	rowCreate
	rowMore
	rowLoading
	rowNote
	rowError
	rowFile
	rowReviewBar
)

type sideRow struct {
	kind rowKind
	sec  *section
	pr   *github.PullRequest
	file *github.PRFile
	data *prData
	text string
	err  error
}

func (r sideRow) key() any {
	switch r.kind {
	case rowPR:
		return fmt.Sprintf("pr:%s:%d", r.sec.id, r.pr.Number)
	case rowFile:
		return "file:" + r.file.Path
	}
	return fmt.Sprintf("%d:%s", r.kind, r.sec.id)
}

func (r sideRow) label() string {
	switch r.kind {
	case rowHeader:
		return r.sec.title
	case rowPR:
		return fmt.Sprintf("%s, #%d by %s", r.pr.Title, r.pr.Number, r.pr.Author)
	case rowFile:
		return r.file.Path
	case rowMore:
		return "Load more"
	case rowCreate:
		return "Create Pull Request"
	case rowReviewBar:
		return "Review actions"
	}
	return r.text
}

func (p *PRs) loadSection(s *section, more bool) {
	s.gen++
	gen := s.gen
	s.loading, s.stale = true, false
	page := 1
	if more {
		page = s.page + 1
	}
	opts := s.opts
	opts.State, opts.Page, opts.PerPage = github.StateOpen, page, pageSize
	be, repo, local := p.be, p.repo, s.local
	names := append([]string(nil), p.locals...)
	go func() {
		var items []github.PullRequest
		var err error
		if local {
			items, err = be.LocalBranchPRs(p.ctx, repo, names)
		} else {
			items, err = be.List(p.ctx, repo, opts)
		}
		p.post(func() {
			if gen != s.gen {
				return
			}
			s.loading = false
			if err != nil {
				s.err = err
				p.authFailure(err)
				return
			}
			s.err, s.loaded, s.page, s.more = nil, true, page, !local && len(items) >= pageSize
			if more {
				s.items = appendUnique(s.items, items)
				return
			}
			s.items = items
		})
	}()
}

func (p *PRs) runSearch(query string) {
	if query == "" {
		p.side.search = nil
		return
	}
	s := &section{id: sectionSearch, title: "Search Results", query: query, opts: github.ListOptions{Query: query}, open: true}
	p.side.search = s
	p.loadSection(s, false)
}

func (p *PRs) ensureLoaded() {
	for _, s := range p.side.sections() {
		if s.open && !s.loading && (s.stale || !s.loaded && s.err == nil) {
			p.loadSection(s, false)
		}
	}
	if pr := p.current.pr; pr != nil && p.side.changes.open {
		d := p.dataFor(pr.Number)
		if !d.loading && (d.stale || !d.loaded && d.err == nil) {
			if d.pr.Title == "" {
				d.pr = *pr
			}
			p.loadDetail(d)
		}
	}
}

func (p *PRs) autoRefresh(c *ui.Context) {
	now := p.env.Now()
	if now.Sub(p.refreshed) >= autoRefreshInterval {
		p.refresh()
		return
	}
	due := p.refreshed.Add(autoRefreshInterval)
	if !p.tickAt.Equal(due) {
		p.tickAt = due
		c.After(due.Sub(now))
	}
}

func (p *PRs) sideRows() []sideRow {
	s := p.side
	rows := []sideRow{{kind: rowHeader, sec: s.current}}
	if s.current.open {
		rows = append(rows, p.currentRows()...)
	}
	if p.current.pr != nil {
		rows = append(rows, p.reviewRows()...)
	}
	for _, sec := range s.sections() {
		rows = append(rows, sideRow{kind: rowHeader, sec: sec})
		if sec.open {
			rows = append(rows, sectionRows(sec)...)
		}
	}
	return rows
}

func (p *PRs) reviewRows() []sideRow {
	sec := p.side.changes
	d := p.details[p.current.pr.Number]
	header := sideRow{kind: rowHeader, sec: sec}
	if d != nil && d.loaded {
		header.text = fmt.Sprintf("%d/%d", d.viewedCount(), len(d.files))
	}
	rows := []sideRow{header}
	if !sec.open {
		return rows
	}
	rows = append(rows, sideRow{kind: rowReviewBar, sec: sec, data: d})
	switch {
	case d == nil || !d.loaded && d.err == nil:
		return append(rows, sideRow{kind: rowLoading, sec: sec, text: "Loading changes..."})
	case !d.loaded:
		return append(rows, sideRow{kind: rowError, sec: sec, err: d.err, data: d})
	case len(d.files) == 0:
		return append(rows, sideRow{kind: rowNote, sec: sec, text: "No changed files."})
	}
	files := d.visibleFiles()
	if len(files) == 0 {
		return append(rows, sideRow{kind: rowNote, sec: sec, text: "Every file is marked as viewed."})
	}
	for i := range files {
		rows = append(rows, sideRow{kind: rowFile, sec: sec, file: &files[i], data: d})
	}
	return rows
}

func (p *PRs) currentRows() []sideRow {
	sec := p.side.current
	cur := p.current
	switch {
	case cur.pr != nil:
		return []sideRow{{kind: rowPR, sec: sec, pr: cur.pr}}
	case cur.err != nil:
		return []sideRow{{kind: rowError, sec: sec, err: cur.err}}
	case cur.loading || !cur.loaded:
		return []sideRow{{kind: rowLoading, sec: sec, text: "Looking for a pull request..."}}
	case p.branch.Name == "":
		return []sideRow{{kind: rowNote, sec: sec, text: "HEAD is detached. Check out a branch to create a pull request."}}
	}
	return []sideRow{{kind: rowCreate, sec: sec, text: p.branch.Name}}
}

func sectionRows(sec *section) []sideRow {
	var rows []sideRow
	switch {
	case sec.err != nil && len(sec.items) == 0:
		return []sideRow{{kind: rowError, sec: sec, err: sec.err}}
	case sec.loading && len(sec.items) == 0:
		return []sideRow{{kind: rowLoading, sec: sec, text: "Loading..."}}
	case sec.loaded && len(sec.items) == 0:
		return []sideRow{{kind: rowNote, sec: sec, text: "No pull requests. Press refresh to check again."}}
	}
	for i := range sec.items {
		rows = append(rows, sideRow{kind: rowPR, sec: sec, pr: &sec.items[i]})
	}
	switch {
	case sec.err != nil:
		rows = append(rows, sideRow{kind: rowError, sec: sec, err: sec.err})
	case sec.loading:
		rows = append(rows, sideRow{kind: rowLoading, sec: sec, text: "Loading..."})
	case sec.more:
		rows = append(rows, sideRow{kind: rowMore, sec: sec})
	}
	return rows
}

func (p *PRs) retrySection(sec *section, d *prData) {
	sec.err = nil
	switch {
	case sec == p.side.current:
		p.current.err = nil
		p.refreshCurrent()
	case sec == p.side.changes && d != nil:
		d.err = nil
		p.loadDetail(d)
	default:
		p.loadSection(sec, sec.loaded && len(sec.items) > 0)
	}
}

func (p *PRs) Title() string {
	return sideViewName
}

func (p *PRs) Badge() int {
	if p.closed || p.phase != phaseReady || p.side == nil {
		return 0
	}
	return len(p.side.review.items)
}

func (p *PRs) SideView(c *ui.Context) {
	if p.closed {
		return
	}
	if p.phase != phaseReady {
		p.stateView(c)
		return
	}
	p.autoRefresh(c)
	p.ensureLoaded()
	s := p.side
	ui.Column(c).Grow(1).FillWidth().MinHeight(0).Children(func() {
		p.sideToolbar(c)
		rows := p.sideRows()
		s.list.Key = func(i int) any { return rows[i].key() }
		s.list.Label = func(i int) string { return rows[i].label() }
		list := ui.List(c, &s.list, len(rows), func(i int) { p.sideRowView(c, rows[i]) }).Grow(1).FillWidth().MinHeight(0).Label("Pull requests")
		if list.Submitted() && s.row >= 0 && s.row < len(rows) {
			r := rows[s.row]
			if r.kind != rowHeader || c.Now().Sub(s.toggledAt) > doubleClickWindow {
				p.activate(r)
			}
		}
	})
}

func (p *PRs) sideToolbar(c *ui.Context) {
	s := p.side
	ui.Row(c).Gap(spacingSmall).AlignItems(ui.Center).Padding(0, toolbarPad, toolbarPad, toolbarPad).Shrink(0).FillWidth().Children(func() {
		field := ui.SearchField(c.Key("search"), &s.field).Placeholder(searchPlaceholder).Label("Search pull requests").Grow(1).MinWidth(0)
		switch {
		case field.Submitted():
			p.runSearch(s.field)
		case field.Changed() && s.field == "" && s.search != nil:
			p.runSearch("")
		}
		if iconButton(c, "refresh", "Refresh", icons.Refresh).Clicked() {
			p.refresh()
		}
	})
}

func (p *PRs) activate(r sideRow) {
	switch r.kind {
	case rowHeader:
		r.sec.open = !r.sec.open
	case rowPR:
		p.openPR(r.pr.Number, r.pr)
	case rowFile:
		p.openDiff(r.data, *r.file, diffSpec{mode: modePR})
	case rowCreate:
		p.openCreate()
	case rowMore:
		p.loadSection(r.sec, true)
	case rowError:
		p.retrySection(r.sec, r.data)
	}
}

func (p *PRs) sideRowView(c *ui.Context, r sideRow) {
	switch r.kind {
	case rowHeader:
		p.headerRow(c, r)
	case rowPR:
		row := p.prRow(c, *r.pr)
		if row.RowClicked() {
			p.openPR(r.pr.Number, r.pr)
		}
		row.ContextMenu(func(m *ui.Menu) { p.prMenu(c, m, r.pr) })
	case rowFile:
		p.fileRow(c, r)
	case rowReviewBar:
		p.reviewBar(c, r)
	case rowCreate:
		row := ui.Column(c).Gap(messageGap).Padding(rowPadY, rowPadX, rowPadY, rowPadX+iconMedium).FillWidth()
		row.Children(func() {
			mutedText(c, "No pull request for "+r.text+".")
			ui.PrimaryButton(c, "Create Pull Request").OnClick(p.openCreate)
		})
		if row.RowClicked() {
			p.openCreate()
		}
	case rowMore:
		row := ui.Row(c).Padding(rowPadY, rowPadX, rowPadY, rowPadX+iconMedium).FillWidth()
		row.Children(func() {
			ui.Button(c, "Load more").OnClick(func() { p.loadSection(r.sec, true) })
		})
		if row.RowClicked() {
			p.loadSection(r.sec, true)
		}
	case rowLoading:
		loadingRow(c, r.text)
	case rowNote:
		mutedText(c, r.text).Padding(rowPadY, rowPadX, rowPadY, rowPadX+iconMedium)
	case rowError:
		errorBlock(c, r.err, func() { p.retrySection(r.sec, r.data) })
	}
}

func (p *PRs) headerRow(c *ui.Context, r sideRow) {
	t := c.Theme()
	row := ui.Row(c).Gap(spacingSmall).AlignItems(ui.Center).Padding(rowPadY, rowPadX).FillWidth()
	row.Children(func() {
		svg := icons.ChevronRight
		if r.sec.open {
			svg = icons.ChevronDown
		}
		ui.Icon(c, svg).Size(iconSmall, iconSmall).TextColor(t.TextMuted).Shrink(0)
		ui.Text(c, r.sec.title).FontSize(fontBody).Bold().SingleLine()
		if r.sec == p.side.current && p.branch.Name != "" {
			mutedText(c, p.branch.Name).SingleLine().MinWidth(0)
		}
		switch {
		case r.text != "":
			mutedPill(c, r.text)
		case len(r.sec.items) > 0:
			label := strconv.Itoa(len(r.sec.items))
			if r.sec.more {
				label += "+"
			}
			mutedPill(c, label)
		}
	})
	if row.RowClicked() {
		r.sec.open = !r.sec.open
		p.side.toggledAt = c.Now()
	}
	row.ContextMenu(func(m *ui.Menu) { p.sectionMenu(m, r.sec) })
}

func (p *PRs) sectionMenu(m *ui.Menu, sec *section) {
	if m.Item("Refresh").Chosen() {
		p.refresh()
	}
	switch {
	case sec == p.side.search:
		m.Separator()
		if m.Item("Save Query...").Chosen() {
			p.askSaveQuery()
		}
		if m.Item("Clear Search").Chosen() {
			p.runSearch("")
			p.side.field = ""
		}
	case slicesContains(p.side.custom, sec):
		m.Separator()
		if m.Item("Edit Query...").Chosen() {
			p.h.Ask("Edit Query", "Save", sec.query, func(v string) error { return p.editQuery(sec, v) })
		}
		if m.Item("Remove Query").Chosen() {
			p.removeQuery(sec)
		}
	}
}

func slicesContains(list []*section, sec *section) bool {
	for _, s := range list {
		if s == sec {
			return true
		}
	}
	return false
}

func (p *PRs) prMenu(c *ui.Context, m *ui.Menu, pr *github.PullRequest) {
	if m.Item("Open Pull Request").Chosen() {
		p.openPR(pr.Number, pr)
	}
	if m.Item("Open on GitHub").Disabled(pr.URL == "").Chosen() {
		openExternal(c, pr.URL)
	}
	if m.Item("Copy Link").Disabled(pr.URL == "").Chosen() {
		c.WriteClipboard(pr.URL)
	}
	m.Separator()
	if m.Item("Checkout").Chosen() {
		p.checkoutPR(pr.Number, &p.checkoutBusy)
	}
}

func (p *PRs) fileRow(c *ui.Context, r sideRow) {
	t := c.Theme()
	d, f := r.data, *r.file
	row := ui.Row(c).Gap(spacingMedium).AlignItems(ui.Center).Padding(rowPadY/2, rowPadX, rowPadY/2, rowPadX+iconMedium).FillWidth()
	row.Children(func() {
		if d.pr.NodeID != "" {
			viewed := d.viewed[f.Path]
			if ui.Checkbox(c.Key("viewed"), &viewed, "").Label("Viewed " + f.Path).Changed() {
				p.setViewed(d, f.Path, viewed)
			}
		}
		ui.Text(c, fileStatusLetter(f.Status)).FontSize(fontSmall).Bold().Width(statLetterWidth).TextColor(fileStatusColor(t, f.Status)).Shrink(0)
		ui.Text(c, f.Path).FontSize(fontBody).SingleLine().Grow(1).MinWidth(0)
		if n := d.commentCount(f.Path); n > 0 {
			ui.Icon(c, icons.Comment).Size(iconSmall, iconSmall).TextColor(t.TextMuted).Shrink(0).Label(fmt.Sprintf("%d comments", n))
			mutedText(c, strconv.Itoa(n)).Shrink(0)
		}
	})
	if row.RowClicked() {
		p.openDiff(d, f, diffSpec{mode: modePR})
	}
}

func (p *PRs) reviewBar(c *ui.Context, r sideRow) {
	d := r.data
	number := p.current.pr.Number
	ui.Row(c).Gap(spacingSmall).AlignItems(ui.Center).Wrap().Padding(rowPadY/2, rowPadX, rowPadY/2, rowPadX+iconMedium).FillWidth().Children(func() {
		ui.Button(c, "Open Pull Request").OnClick(func() { p.openPR(number, p.current.pr) })
		ui.Button(c, "Exit Review Mode").Disabled(p.checkoutBusy != "").OnClick(p.exitReview)
		if d != nil && d.loaded && d.viewedCount() > 0 {
			ui.Toggle(c.Key("hide-viewed"), &d.hideViewed, "Hide viewed")
		}
	})
}

func (p *PRs) exitReview() {
	pr := p.current.pr
	if pr == nil || pr.BaseRef == "" {
		p.h.Notify("The current branch has no pull request.")
		return
	}
	local, base := p.local, pr.BaseRef
	p.act(&p.checkoutBusy, "Checking out "+base, "Cannot check out "+base, func(ctx context.Context) (func(), error) {
		if err := local.Checkout(ctx, base, false); err != nil {
			return nil, err
		}
		return func() {
			p.closeReviewTabs()
			p.h.Notify("Checked out " + base)
			p.BranchChanged()
		}, nil
	})
}

func (p *PRs) askSaveQuery() {
	if p.side.search == nil {
		p.h.Notify("Search for pull requests first, then save the query.")
		return
	}
	query := p.side.search.query
	p.h.Ask("Save Query", "Save", query, func(label string) error { return p.addQuery(label, query) })
}

func (p *PRs) prRow(c *ui.Context, pr github.PullRequest) ui.Element {
	t := c.Theme()
	now := p.env.Now()
	row := ui.Row(c).Gap(spacingMedium).AlignItems(ui.Start).Padding(rowPadY, rowPadX).FillWidth()
	row.Children(func() {
		ui.Icon(c, icons.PullRequest).Size(iconMedium, iconMedium).TextColor(stateColor(t, pr)).Shrink(0).Margin(prRowGap, 0, 0, 0)
		ui.Column(c).Grow(1).MinWidth(0).Gap(prRowGap).Children(func() {
			ui.Text(c, pr.Title).FontSize(fontBody).SingleLine()
			ui.Row(c).Gap(spacingSmall + prRowGap).AlignItems(ui.Center).Children(func() {
				mutedText(c, "#"+strconv.Itoa(pr.Number)).SingleLine().Shrink(0)
				if pr.Author != "" {
					mutedText(c, pr.Author).SingleLine().Shrink(1).MinWidth(0)
				}
				if when := relativeTime(now, pr.Updated); when != "" {
					mutedText(c, when).SingleLine().Shrink(0)
				}
				if pr.Draft {
					mutedPill(c, prStateDraft)
				}
				checkIcon(c, p.checks[pr.Number])
			})
		})
	})
	return row
}
