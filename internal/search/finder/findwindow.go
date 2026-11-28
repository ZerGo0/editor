package finder

import (
	"strconv"
	"strings"

	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/list"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	pl "github.com/ZerGo0/editor/internal/plural"
	"github.com/ZerGo0/editor/internal/search/textsearch"
	"github.com/egoist/mygo/ui"
)

const (
	findListLabel  = "Find results"
	expandAllLabel = "Expand All"
	collapseLabel  = "Collapse All"
	clearLabel     = "Clear Results"
	searchingLabel = "Searching..."
)

type findRow struct {
	file  *FileNode
	match int
}

type FindWindow struct {
	env       Env
	res       FindResults
	collapsed map[string]bool
	rows      []findRow
	dirty     bool
	list      ui.ListState
	selected  int
	cursor    findRow
	hasCursor bool
}

func NewFindWindow(env Env) *FindWindow {
	f := &FindWindow{env: env, collapsed: map[string]bool{}, selected: -1, dirty: true}
	f.list.Selected = &f.selected
	return f
}

func (f *FindWindow) Show(res FindResults) {
	f.res = res
	f.collapsed = map[string]bool{}
	f.selected, f.hasCursor, f.dirty = -1, false, true
}

func (f *FindWindow) Clear() {
	f.Show(FindResults{})
}

func (f *FindWindow) Count() int {
	return f.res.Total
}

func (f *FindWindow) build() {
	if !f.dirty {
		return
	}
	f.dirty = false
	f.rows = f.rows[:0]
	for _, file := range f.res.Files {
		f.rows = append(f.rows, findRow{file: file, match: -1})
		if f.collapsed[file.Path] {
			continue
		}
		for i := range file.Matches {
			f.rows = append(f.rows, findRow{file: file, match: i})
		}
	}
}

func (f *FindWindow) setAllCollapsed(collapsed bool) {
	f.collapsed = map[string]bool{}
	if collapsed {
		for _, file := range f.res.Files {
			f.collapsed[file.Path] = true
		}
	}
	f.dirty = true
}

func (f *FindWindow) toggle(file *FileNode) {
	f.collapsed[file.Path] = !f.collapsed[file.Path]
	f.dirty = true
}

func (f *FindWindow) openRow(r findRow) {
	f.openRowWith(r, OpenOptions{Preview: true, Focus: true})
}

func (f *FindWindow) openRowWith(r findRow, o OpenOptions) {
	f.cursor, f.hasCursor = r, true
	f.env.open(r.file.location(r.match), o)
}

func (f *FindWindow) Actions(c *ui.Context) {
	empty := len(f.res.Files) == 0
	if action.IconButton(c, icons.ExpandAll, expandAllLabel, action.IconOptions{Disabled: empty}).Clicked() {
		f.setAllCollapsed(false)
	}
	if action.IconButton(c, icons.CollapseAll, collapseLabel, action.IconOptions{Disabled: empty}).Clicked() {
		f.setAllCollapsed(true)
	}
	if action.IconButton(c, icons.ClearAll, clearLabel, action.IconOptions{Disabled: empty}).Clicked() {
		f.Clear()
	}
}

func (f *FindWindow) View(c *ui.Context) {
	f.build()
	p := token.Of(c)
	ui.Column(c).Grow(1).FillWidth().MinHeight(0).Children(func() {
		if f.res.Title != "" {
			ui.Row(c).FillWidth().Padding(token.PadInputY.Px(), token.PadRow.Px()).Children(func() {
				token.TextLabel.Apply(ui.Text(c, f.summary())).TextColor(p.Text.Secondary).SingleLine().Role(ui.RoleStatus)
			})
		}
		list.View(c, &f.list, len(f.rows), list.ViewOptions{Label: findListLabel, Tree: true, Empty: f.empty}, func(i int) list.Item { return f.item(c, p, i) })
	})
}

func (f *FindWindow) summary() string {
	if len(f.res.Files) == 0 {
		return f.res.Title
	}
	if f.res.Total == len(f.res.Files) {
		return f.res.Title + " · " + plural(f.res.Total, "file")
	}
	return f.res.Title + " · " + plural(f.res.Total, "match") + " in " + plural(len(f.res.Files), "file")
}

func plural(n int, noun string) string {
	if noun == "match" {
		return pl.Count(n, noun, "matches")
	}
	return pl.Count(n, noun, noun+"s")
}

func (f *FindWindow) empty(c *ui.Context) {
	if f.res.Busy {
		f.skeleton(c)
		return
	}
	feedback.Empty(c, feedback.EmptyOptions{Icon: icons.Search, Title: "No results to show", Message: "Choose Open in Find Window in Search Everywhere to keep its results here."})
}

func (f *FindWindow) rowMenu(c *ui.Context, r findRow) func(m *ui.Menu) {
	return func(m *ui.Menu) {
		if m.Item("Open").Chosen() {
			f.openRow(r)
		}
		if m.Item("Open to the Side").Chosen() {
			f.openRowWith(r, OpenOptions{Focus: true, Side: true})
		}
		m.Separator()
		if m.Item("Copy").Disabled(r.match < 0).Chosen() {
			c.WriteClipboard(strings.TrimSpace(r.file.Matches[r.match].Preview))
		}
		if m.Item("Copy Path").Chosen() {
			c.WriteClipboard(r.file.Path)
		}
	}
}

func (f *FindWindow) item(c *ui.Context, p *token.Palette, i int) list.Item {
	r := f.rows[i]
	if r.match < 0 {
		file := r.file
		it := FileItem(file)
		it.Menu = f.rowMenu(c, r)
		if len(file.Matches) > 0 {
			it.Branch, it.Open = true, !f.collapsed[file.Path]
			it.OnClick = func() { f.toggle(file) }
			it.OnDoubleClick = func() { f.openRow(r) }
			return it
		}
		it.OnClick = func() { f.openRow(r) }
		return it
	}
	it := MatchItem(p, r.file.Matches[r.match])
	it.OnClick = func() { f.openRow(r) }
	it.Menu = f.rowMenu(c, r)
	return it
}

func (f *FindWindow) skeleton(c *ui.Context) {
	p := token.Of(c)
	file := &FileNode{Name: feedback.Placeholder(0, 1), Dir: feedback.Placeholder(1, 1), Matches: make([]textsearch.Match, list.SkeletonRows-1)}
	list.Skeleton(c, searchingLabel, list.SkeletonRows, func(i int) list.Item {
		if i == 0 {
			it := FileItem(file)
			it.Branch, it.Open = true, true
			return it
		}
		return MatchItem(p, textsearch.Match{Preview: feedback.Placeholder(i, placeholderLineWords+i%placeholderLineWords), Start: textsearch.Pos{Line: i}})
	})
}

func FileItem(file *FileNode) list.Item {
	return list.Item{Label: file.Name, Description: file.Dir, Leading: fileIcon(file.Name), Count: len(file.Matches), Strong: true}
}

func MatchItem(p *token.Palette, m textsearch.Match) list.Item {
	return list.Item{Label: lineLabel(m), Depth: 1, Description: "line " + strconv.Itoa(m.Start.Line+1),
		Rich: func(s list.Style) []ui.Span { return matchSpans(p, s, m, "", false) }}
}

func (f *FindWindow) targets() []findRow {
	var out []findRow
	for _, file := range f.res.Files {
		if len(file.Matches) == 0 {
			out = append(out, findRow{file: file, match: -1})
			continue
		}
		for i := range file.Matches {
			out = append(out, findRow{file: file, match: i})
		}
	}
	return out
}

func (f *FindWindow) Next() { f.step(1) }

func (f *FindWindow) Prev() { f.step(-1) }

func (f *FindWindow) step(dir int) {
	targets := f.targets()
	if len(targets) == 0 {
		return
	}
	at := -1
	if f.hasCursor {
		for i, t := range targets {
			if t == f.cursor {
				at = i
				break
			}
		}
	}
	switch {
	case at < 0 && dir > 0:
		at = 0
	case at < 0:
		at = len(targets) - 1
	default:
		at = (at + dir + len(targets)) % len(targets)
	}
	target := targets[at]
	delete(f.collapsed, target.file.Path)
	f.dirty = true
	f.openRow(target)
}
