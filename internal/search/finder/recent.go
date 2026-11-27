package finder

import (
	"strings"

	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/list"
	"github.com/ZerGo0/editor/internal/design/picker"
	"github.com/ZerGo0/editor/internal/fuzzy"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/ZerGo0/editor/internal/search/everywhere"
	"github.com/egoist/mygo/ui"
)

const (
	recentLabel       = "Recent Files"
	toolWindowsGroup  = "Tool Windows"
	recentPlaceholder = "Type to filter recent files and tool windows"
	recentID          = "recent"
	recentListLabel   = "Recent files and tool windows"
)

type recentRow struct {
	file everywhere.Result
	tool *ToolWindow
}

type RecentFiles struct {
	env     Env
	open    bool
	query   string
	results picker.ResultsState
	items   []recentRow
	rows    []picker.Row
	field   ui.Handle
	focus   bool
	listed  string
}

func NewRecentFiles(env Env) *RecentFiles {
	return &RecentFiles{env: env}
}

func (r *RecentFiles) IsOpen() bool { return r.open }

func (r *RecentFiles) Open() {
	r.open, r.focus, r.query = true, true, ""
	r.refresh()
}

func (r *RecentFiles) Close() {
	r.open = false
}

func (r *RecentFiles) refresh() {
	r.listed = r.query
	q := strings.TrimSpace(r.query)
	var files []everywhere.Result
	if r.env.Engine != nil {
		sess := r.env.Engine.Session()
		sess.MarkOpen(r.env.openFiles())
		files = sess.Recent(recentFetchLimit)
	}
	active := r.env.Host.ActivePath()
	var tools []ToolWindow
	if r.env.ToolWindows != nil {
		tools = r.env.ToolWindows()
	}
	r.items, r.rows = nil, nil
	var fileRows []picker.Row
	for _, f := range filterFiles(files, q, active) {
		item := list.Item{Label: f.Label, Matches: f.Positions, Description: f.Detail, Leading: fileIcon(f.Label)}
		r.items = append(r.items, recentRow{file: f})
		fileRows = append(fileRows, picker.Row{Item: item})
	}
	if len(fileRows) > 0 {
		r.rows = append(r.rows, picker.Row{Kind: picker.RowHeader, Title: recentLabel})
		r.rows = append(r.rows, fileRows...)
		r.items = append([]recentRow{{}}, r.items...)
	}
	rs := fuzzy.Filter(q, len(tools), func(i int) string { return tools[i].Title })
	if len(rs) > 0 {
		r.rows = append(r.rows, picker.Row{Kind: picker.RowHeader, Title: toolWindowsGroup})
		r.items = append(r.items, recentRow{})
	}
	for _, m := range rs {
		t := tools[m.Index]
		r.items = append(r.items, recentRow{tool: &t})
		r.rows = append(r.rows, picker.Row{Item: list.Item{Label: t.Title, Matches: m.Match.Positions, Icon: t.Icon}, Keys: t.Keys})
	}
	r.results.Reset(r.rows)
}

func filterFiles(files []everywhere.Result, q, active string) []everywhere.Result {
	var out []everywhere.Result
	for _, f := range files {
		if f.Path == active {
			continue
		}
		if q != "" {
			m, ok := fuzzy.Find(q, f.Label)
			if !ok {
				continue
			}
			f.Positions = m.Positions
		}
		out = append(out, f)
	}
	return out
}

func (r *RecentFiles) View(c *ui.Context) {
	if !r.open {
		return
	}
	if r.query != r.listed {
		r.refresh()
	}
	if picker.Dialog(c, picker.DialogOptions{Label: recentLabel, ID: recentID, WindowKeys: r.env.WindowKeys}, func() { r.body(c) }) {
		r.Close()
	}
}

func (r *RecentFiles) body(c *ui.Context) {
	keys := picker.Query(c, recentLabel, &r.query, picker.QueryOptions{Placeholder: recentPlaceholder, Handle: &r.field})
	if r.focus {
		r.field.SelectAll()
		r.focus = false
	}
	switch {
	case keys.Down:
		r.results.Move(1)
	case keys.Up:
		r.results.Move(-1)
	case keys.Submitted:
		r.accept(r.results.Selected)
	}
	if i := picker.Results(c, &r.results, r.rows, picker.ResultsOptions{Label: recentListLabel, Empty: r.empty}); i >= 0 && r.open {
		r.accept(i)
	}
}

func (r *RecentFiles) empty(c *ui.Context) {
	feedback.Empty(c, feedback.EmptyOptions{Icon: icons.History, Title: "No recent files or tool windows"})
}

func (r *RecentFiles) accept(row int) {
	if row < 0 || row >= len(r.items) {
		return
	}
	it := r.items[row]
	switch {
	case it.tool != nil:
		r.Close()
		it.tool.Show()
	case it.file.Path != "":
		r.Close()
		r.env.open(Location{Path: it.file.Path}, OpenOptions{Focus: true})
	}
}
