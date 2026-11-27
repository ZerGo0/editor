package finder

import (
	"slices"
	"strings"
)

func (d *Dialog) accept(row int, side bool) {
	if row < 0 || row >= len(d.rowEntry) || d.rowEntry[row] == noEntry {
		return
	}
	e := d.entries[d.rowEntry[row]]
	switch e.kind {
	case kindMore:
		d.setTab(e.tab)
	case kindHint:
		if e.query != "" {
			d.st.Query, d.selectAll = e.query, false
			d.queryChanged()
		}
	case kindAction:
		d.Close()
		if d.env.RunAction != nil {
			d.env.RunAction(e.actionID)
		}
	default:
		loc, ok := e.location()
		if !ok {
			return
		}
		d.Close()
		d.env.open(loc, OpenOptions{Focus: true, Side: side})
	}
}

func (d *Dialog) canOpenInFind() bool {
	switch d.tab {
	case TabText:
		return d.text.total > 0 && !d.text.searching && d.text.err == ""
	case TabFiles:
		for _, e := range d.entries {
			if e.kind == kindFile {
				return true
			}
		}
	}
	return false
}

func (d *Dialog) openInFindWindow() {
	var res FindResults
	switch d.tab {
	case TabText:
		files := slices.Clone(d.text.files)
		slices.SortFunc(files, func(a, b *FileNode) int { return strings.Compare(a.Rel, b.Rel) })
		res = FindResults{Title: "Text matches for " + quote(d.st.Query), Files: files, Total: d.text.total}
	case TabFiles:
		res = FindResults{Title: "Files matching " + quote(d.st.Query)}
		for _, e := range d.entries {
			if e.kind == kindFile {
				res.Files = append(res.Files, NewFileNode(e.path, relTo(d.env.Host.RootDir(), e.path), nil))
			}
		}
		res.Total = len(res.Files)
	default:
		return
	}
	d.Close()
	if d.env.ShowFindResults != nil {
		d.env.ShowFindResults(res)
	}
}
