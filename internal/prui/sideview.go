package prui

import (
	"context"
	"fmt"
	"path"
	"strconv"
	"time"

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
	sectionCurrent  = "current"
	sectionChanges  = "changes"
	sectionReview   = "review"
	sectionAssigned = "assigned"
	sectionCreated  = "created"
	sectionAll      = "all"
	sectionLocal    = "local"
	sectionSearch   = "search"

	searchPlaceholder = "Search pull requests (GitHub syntax)"
	prMetaSeparator   = " · "
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
	rowReviewOpen
	rowReviewExit
	rowReviewHide
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
	case rowReviewOpen:
		return "Open Pull Request"
	case rowReviewExit:
		return "Exit Review Mode"
	case rowReviewHide:
		return "Hide viewed files"
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
	rows = append(rows, sideRow{kind: rowReviewOpen, sec: sec, data: d}, sideRow{kind: rowReviewExit, sec: sec, data: d})
	if d != nil && d.loaded && d.viewedCount() > 0 {
		rows = append(rows, sideRow{kind: rowReviewHide, sec: sec, data: d})
	}
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
	return []sideRow{{kind: rowNote, sec: sec, text: "No pull request for " + p.branch.Name + "."}, {kind: rowCreate, sec: sec, text: p.branch.Name}}
}

func sectionRows(sec *section) []sideRow {
	var rows []sideRow
	switch {
	case sec.err != nil && len(sec.items) == 0:
		return []sideRow{{kind: rowError, sec: sec, err: sec.err}}
	case sec.loading && len(sec.items) == 0:
		return []sideRow{{kind: rowLoading, sec: sec, text: "Loading..."}}
	case sec.loaded && len(sec.items) == 0:
		return []sideRow{{kind: rowNote, sec: sec, text: "No pull requests"}}
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
		p.sideSearch(c)
		rows := p.sideRows()
		s.list.Key = func(i int) any { return rows[i].key() }
		s.list.Label = func(i int) string { return p.rowLabel(rows[i]) }
		el := list.View(c, &s.list, len(rows), list.ViewOptions{Label: "Pull requests", Tree: true}, func(i int) list.Item { return p.sideItem(c, rows[i]) })
		if el.Submitted() && s.row >= 0 && s.row < len(rows) {
			r := rows[s.row]
			if r.kind != rowHeader || c.Now().Sub(s.toggledAt) > doubleClickWindow {
				p.activate(r)
			}
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

func (p *PRs) rowLabel(r sideRow) string {
	label := r.label()
	if r.kind == rowPR {
		if text := checkStateLabel(p.checks[r.pr.Number]); text != "" {
			label += ", " + text
		}
	}
	return label
}

func (p *PRs) SideActions(c *ui.Context) {
	if p.Ready() {
		action.IconButton(c, icons.Refresh, "Refresh", action.IconOptions{ID: "refresh"}).OnClick(p.refresh)
	}
}

func (p *PRs) sideSearch(c *ui.Context) {
	s := p.side
	ui.Column(c).FillWidth().Padding(token.GapInline.Px(), token.PadRow.Px(), token.GapControl.Px(), token.PadRow.Px()).Shrink(0).Children(func() {
		field := field.SearchInput(c, "Search pull requests", &s.field, field.SearchOptions{Placeholder: searchPlaceholder, ID: "search"})
		switch {
		case field.Submitted():
			p.runSearch(s.field)
		case field.Changed() && s.field == "" && s.search != nil:
			p.runSearch("")
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
	case rowReviewOpen:
		p.openPR(p.current.pr.Number, p.current.pr)
	case rowReviewExit:
		if p.checkoutBusy == "" {
			p.exitReview()
		}
	case rowReviewHide:
		r.data.hideViewed = !r.data.hideViewed
	case rowMore:
		p.loadSection(r.sec, true)
	case rowError:
		p.retrySection(r.sec, r.data)
	}
}

func (p *PRs) sideItem(c *ui.Context, r sideRow) list.Item {
	switch r.kind {
	case rowHeader:
		return p.headerItem(c, r)
	case rowPR:
		return p.prItem(c, r.pr)
	case rowFile:
		return p.fileItem(r)
	case rowReviewOpen:
		return list.Item{Depth: 1, Icon: icons.PullRequest, Label: r.label(), OnClick: func() { p.activate(r) }}
	case rowReviewExit:
		return list.Item{Depth: 1, Icon: icons.Close, Label: r.label(), Muted: p.checkoutBusy != "", OnClick: func() { p.activate(r) }}
	case rowReviewHide:
		return list.Item{Depth: 1, Content: func(c *ui.Context) {
			action.Checkbox(c, "Hide viewed", &r.data.hideViewed, action.ChoiceOptions{ID: "hide-viewed"})
		}}
	case rowCreate:
		return list.Item{Depth: 1, Icon: icons.Plus, Label: r.label(), Strong: true, OnClick: func() { p.activate(r) }}
	case rowMore:
		return list.Item{Depth: 1, Label: r.label(), OnClick: func() { p.activate(r) }}
	case rowLoading:
		return list.Item{Depth: 1, Muted: true, Label: r.text, Leading: func(c *ui.Context) { feedback.Spinner(c, r.text) }}
	case rowNote:
		return list.Item{Depth: 1, Muted: true, Label: r.text, Tooltip: r.text}
	case rowError:
		return list.Item{Depth: 1, Tone: token.ToneError, Label: errorText(r.err), Tooltip: errorText(r.err), Trailing: func(c *ui.Context) {
			ghostButton(c, retryLabel, "retry").OnClick(func() { p.retrySection(r.sec, r.data) })
		}}
	}
	return list.Item{}
}

func (p *PRs) headerItem(c *ui.Context, r sideRow) list.Item {
	it := list.Item{Label: r.sec.title, Strong: true, Branch: true, Open: r.sec.open, Menu: func(m *ui.Menu) { p.sectionMenu(m, r.sec) }}
	switch {
	case r.sec == p.side.current && p.branch.Name != "":
		it.Description = p.branch.Name
	case r.text != "":
		it.Description = r.text
	default:
		it.Count = len(r.sec.items)
	}
	it.OnClick = func() {
		r.sec.open = !r.sec.open
		p.side.toggledAt = c.Now()
	}
	return it
}

func (p *PRs) prItem(c *ui.Context, pr *github.PullRequest) list.Item {
	pal := token.Of(c)
	description := "#" + strconv.Itoa(pr.Number)
	if when := relativeTime(p.env.Now(), pr.Updated); when != "" {
		description += prMetaSeparator + when
	}
	return list.Item{
		Depth:       1,
		Label:       pr.Title,
		Description: description,
		Tooltip:     fmt.Sprintf("%s\n#%d by %s", pr.Title, pr.Number, pr.Author),
		Leading: func(c *ui.Context) {
			ui.Icon(c, icons.PullRequest).Size(token.IconSize.Px(), token.IconSize.Px()).TextColor(pal.Tone(stateTone(*pr))).Shrink(0).AccessHidden(true)
		},
		Trailing: func(c *ui.Context) {
			if pr.Draft {
				mutedTag(c, prStateDraft)
			}
			checkIcon(c, p.checks[pr.Number])
			if pr.Author != "" {
				badge.Avatar(c, pr.Author, token.AvatarSize)
			}
		},
		OnClick: func() { p.openPR(pr.Number, pr) },
		Menu:    func(m *ui.Menu) { p.prMenu(c, m, pr) },
	}
}

func (p *PRs) fileItem(r sideRow) list.Item {
	d, f := r.data, *r.file
	it := list.Item{
		Depth:       1,
		Label:       path.Base(f.Path),
		Description: pathDir(f.Path),
		Tooltip:     f.Path,
		Git:         fileGitStatus(f.Status),
		Count:       d.commentCount(f.Path),
		OnClick:     func() { p.openDiff(d, f, diffSpec{mode: modePR}) },
	}
	if d.pr.NodeID != "" {
		it.Leading = func(c *ui.Context) { p.viewedBox(c, d, f.Path) }
	}
	return it
}

func (p *PRs) viewedBox(c *ui.Context, d *prData, filePath string) {
	viewed := d.viewed[filePath]
	if action.Checkbox(c, "", &viewed, action.ChoiceOptions{ID: "viewed"}).Label("Viewed " + filePath).Changed() {
		p.setViewed(d, filePath, viewed)
	}
}

func pathDir(p string) string {
	if dir := path.Dir(p); dir != "." {
		return dir
	}
	return ""
}

func fileGitStatus(status string) token.GitStatus {
	switch status {
	case statusAdded:
		return token.GitAdded
	case statusRemoved:
		return token.GitDeleted
	case statusRenamed:
		return token.GitRenamed
	}
	return token.GitModified
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
