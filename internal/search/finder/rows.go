package finder

import (
	"strconv"

	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/list"
	"github.com/ZerGo0/editor/internal/design/picker"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/ZerGo0/editor/internal/workspace/tablabel"
	"github.com/egoist/mygo/ui"
)

const (
	removeRecentLabel = "Remove from Recent"
	noEntry           = -1
)

func (d *Dialog) rowsOf(entries []entry) ([]picker.Row, []int) {
	rows := make([]picker.Row, 0, len(entries))
	index := make([]int, 0, len(entries))
	prev := ""
	for i, e := range entries {
		if e.group != "" && e.group != prev {
			rows = append(rows, picker.Row{Kind: picker.RowHeader, Title: e.group})
			index = append(index, noEntry)
		}
		prev = e.group
		rows = append(rows, d.row(e))
		index = append(index, i)
	}
	return rows, index
}

func fileIcon(name string) func(c *ui.Context) {
	return func(c *ui.Context) { icons.FileIcon(c, name, false, false, token.IconSize.Px()) }
}

func (d *Dialog) row(e entry) picker.Row {
	switch e.kind {
	case kindMore:
		return picker.Row{Kind: picker.RowMore, Title: e.label}
	case kindFile:
		it := list.Item{Label: e.label, Matches: e.positions, Aside: e.detail, Tooltip: relTo(d.env.Host.RootDir(), e.path), Leading: fileIcon(e.label)}
		if e.recent && d.tab == TabFiles {
			path := e.path
			it.Actions = func(c *ui.Context) {
				if action.IconButton(c, icons.Close, removeRecentLabel, action.IconOptions{}).Clicked() {
					d.forget(path)
				}
			}
		}
		return picker.Row{Item: it}
	case kindSymbol:
		return picker.Row{Item: list.Item{Label: e.label, Matches: e.positions, Description: e.detail, Icon: icons.ListView}}
	case kindAction:
		return picker.Row{Item: list.Item{Label: e.label, Matches: e.positions}, Keys: e.keys}
	case kindMatch:
		return picker.Row{Item: d.matchItem(e)}
	case kindLine:
		return picker.Row{Item: list.Item{Label: e.label, Icon: icons.ArrowRight}}
	}
	return picker.Row{Item: list.Item{Label: e.label, Description: e.detail}}
}

func (d *Dialog) labelMatches(entries []entry) {
	var paths []string
	for _, e := range entries {
		if e.kind == kindMatch {
			paths = append(paths, e.path)
		}
	}
	qualified := tablabel.Qualify(paths)
	for i, e := range entries {
		if e.kind != kindMatch {
			continue
		}
		name := e.file.Name
		if q, ok := qualified[e.path]; ok {
			name = q
		}
		entries[i].detail = name + " " + strconv.Itoa(e.file.Matches[e.match].Start.Line+1)
	}
}

func (d *Dialog) matchItem(e entry) list.Item {
	file, i := e.file, e.match
	m := file.Matches[i]
	it := list.Item{Label: e.label, Aside: e.detail, Tooltip: file.Rel, Rich: func(s list.Style) []ui.Span {
		if d.replaceVisible() && d.text.matcher != nil {
			return matchSpans(d.pal, s, m, d.replacement().Expand(m.Matched, m.Groups), true)
		}
		return matchSpans(d.pal, s, m, "", false)
	}}
	it.Actions = func(c *ui.Context) {
		if !d.replaceVisible() {
			return
		}
		if action.IconButton(c, icons.Replace, replaceLabel, action.IconOptions{Keys: replaceKeys, Disabled: !d.canReplace()}).Clicked() {
			d.replaceMatch(file, i)
		}
		if action.IconButton(c, icons.ReplaceAll, replaceFileLabel, action.IconOptions{Disabled: !d.canReplace()}).Clicked() {
			d.replaceFile(file)
		}
	}
	return it
}

func (d *Dialog) forget(path string) {
	if d.env.ForgetRecent != nil {
		d.env.ForgetRecent(path)
	}
	d.refresh()
}

func (d *Dialog) current() (entry, bool) {
	row := d.results.Selected
	if row < 0 || row >= len(d.rowEntry) || d.rowEntry[row] == noEntry {
		return entry{}, false
	}
	return d.entries[d.rowEntry[row]], true
}
