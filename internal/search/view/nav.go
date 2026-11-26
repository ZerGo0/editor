package searchview

import "strings"

func (v *View) Next() { v.step(1) }

func (v *View) Prev() { v.step(-1) }

func (v *View) navigable() []row {
	var out []row
	if v.scope == ScopeText {
		for _, f := range v.text.files {
			for i, m := range f.matches {
				out = append(out, row{kind: rowMatch, key: matchKey(f, m), file: f, match: i})
			}
		}
		return out
	}
	for _, r := range v.rows {
		if _, ok := r.location(); ok && r.kind == rowResult {
			out = append(out, r)
		}
	}
	return out
}

func (v *View) step(dir int) {
	if v.rowsDirty {
		v.rebuild()
	}
	list := v.navigable()
	if len(list) == 0 {
		return
	}
	at := -1
	for i, r := range list {
		if r.key == v.cursor || r.key == v.selectedKey {
			at = i
			break
		}
	}
	switch {
	case at < 0 && dir > 0:
		at = 0
	case at < 0:
		at = len(list) - 1
	default:
		at = (at + dir + len(list)) % len(list)
	}
	target := list[at]
	v.cursor, v.selectedKey = target.key, target.key
	v.userMoved = true
	if target.file != nil {
		delete(v.collapsed, "f:"+target.file.path)
		v.expandFolders(target.file)
	}
	v.rebuild()
	v.scrollTo = true
	v.openLocation(target, OpenOptions{Preview: true, Focus: true})
}

func (v *View) expandFolders(f *fileNode) {
	for dir := f.dir; dir != ""; {
		delete(v.collapsed, "d:"+dir)
		i := strings.LastIndexByte(dir, '/')
		if i < 0 {
			break
		}
		dir = dir[:i]
	}
}
