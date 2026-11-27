package finder

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
		return d.text.total > 0 && !d.text.searching
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
		res = FindResults{Title: "Text matches for " + quote(d.st.Query), Files: d.text.files, Total: d.text.total}
	case TabFiles:
		res = FindResults{Title: "Files matching " + quote(d.st.Query)}
		for _, e := range d.entries {
			if e.kind == kindFile {
				res.Files = append(res.Files, newFileNode(e.path, relTo(d.env.Host.RootDir(), e.path), nil))
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
