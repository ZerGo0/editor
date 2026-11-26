package searchview

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/ZerGo0/editor/internal/search/everywhere"
	"github.com/ZerGo0/editor/internal/search/textsearch"
	"github.com/egoist/mygo/ui/editor"
)

func matchLocation(f *fileNode, m textsearch.Match) Location {
	return Location{
		Path:     f.path,
		Start:    editor.Pos{Line: m.Start.Line, Col: m.Start.Col},
		End:      editor.Pos{Line: m.End.Line, Col: m.End.Col},
		HasRange: true,
	}
}

func (r row) location() (Location, bool) {
	switch r.kind {
	case rowMatch:
		return matchLocation(r.file, r.file.matches[r.match]), true
	case rowFile:
		if len(r.file.matches) > 0 {
			return matchLocation(r.file, r.file.matches[0]), true
		}
		return Location{Path: r.file.path}, true
	case rowGroup:
		return Location{Path: r.res.Path, Start: editor.Pos{Line: r.res.Line, Col: r.res.Col}, End: editor.Pos{Line: r.res.Line, Col: r.res.Col}, HasRange: true}, true
	case rowResult:
		if r.res.Kind != everywhere.KindFile && r.res.Kind != everywhere.KindSymbol {
			return Location{}, false
		}
		pos := editor.Pos{Line: r.res.Line, Col: r.res.Col}
		return Location{Path: r.res.Path, Start: pos, End: pos, HasRange: r.res.HasPos}, true
	}
	return Location{}, false
}

func (v *View) rowAt(i int) (row, bool) {
	if i < 0 || i >= len(v.rows) {
		return row{}, false
	}
	return v.rows[i], true
}

func (v *View) previewRow(i int) {
	r, ok := v.rowAt(i)
	if !ok || r.key == v.lastPreview {
		return
	}
	if r.kind == rowGoto {
		v.openGoto(r, OpenOptions{Preview: true})
		v.lastPreview = r.key
		return
	}
	if loc, ok := r.location(); ok && r.kind != rowGroup {
		v.lastPreview = r.key
		v.env.open(loc, OpenOptions{Preview: true})
	}
}

func (v *View) activateRow(i int, o OpenOptions) {
	r, ok := v.rowAt(i)
	if !ok {
		return
	}
	switch r.kind {
	case rowHeader:
		v.switchScope(r.scope)
	case rowFolder, rowGroup:
		v.toggle(r.key)
	case rowRecentQuery:
		v.useQuery(r.query)
	case rowGoto:
		v.openGoto(r, o)
	case rowFile, rowMatch:
		v.openLocation(r, o)
	case rowResult:
		v.activateResult(r, o)
	}
}

func (v *View) openLocation(r row, o OpenOptions) {
	if loc, ok := r.location(); ok {
		v.lastPreview = r.key
		o.Focus = o.Focus || !o.Preview
		v.env.open(loc, o)
	}
}

func (v *View) openGoto(r row, o OpenOptions) {
	path := v.h.ActivePath()
	if path == "" {
		return
	}
	pos := editor.Pos{Line: r.line, Col: r.col}
	o.Focus = o.Focus || !o.Preview
	v.env.open(Location{Path: path, Start: pos, End: pos, HasRange: true}, o)
}

func (v *View) activateResult(r row, o OpenOptions) {
	switch r.res.Kind {
	case everywhere.KindFile, everywhere.KindSymbol:
		v.openLocation(r, o)
	case everywhere.KindAction:
		if v.env.RunAction != nil {
			v.env.RunAction(r.res.ActionID)
		}
	case everywhere.KindGitRef:
		if v.env.GitRef != nil {
			v.env.GitRef(r.res.Ref)
		}
	case everywhere.KindGitCommit:
		if v.env.GitCommit != nil {
			v.env.GitCommit(r.res.OID)
		}
	}
}

func (v *View) switchScope(s Scope) {
	if s == v.scope {
		return
	}
	v.setScope(s)
	v.selected, v.selectedKey = -1, ""
	v.searchSoon(0)
}

func (v *View) useQuery(q string) {
	v.setScope(ScopeText)
	v.st.Query = q
	v.selected, v.selectedKey = -1, ""
	v.field.Focus()
	v.searchSoon(0)
}

func (v *View) toggle(key string) {
	if v.collapsed[key] {
		delete(v.collapsed, key)
	} else {
		v.collapsed[key] = true
	}
	v.rowsDirty = true
}

func (v *View) branchKeys() []string {
	var keys []string
	if v.scope == ScopeText {
		var walk func(n *dirNode)
		walk = func(n *dirNode) {
			for _, d := range n.dirs {
				keys = append(keys, "d:"+d.path)
				walk(d)
			}
		}
		if !v.st.ListView {
			walk(buildTree(v.text.files))
		}
		for _, f := range v.text.files {
			keys = append(keys, "f:"+f.path)
		}
		return keys
	}
	for _, r := range v.sections[ScopeSymbols].items {
		keys = append(keys, "g:"+r.Path)
	}
	return keys
}

func (v *View) anyExpanded() bool {
	for _, k := range v.branchKeys() {
		if !v.collapsed[k] {
			return true
		}
	}
	return false
}

func (v *View) toggleAll() {
	collapse := v.anyExpanded()
	for _, k := range v.branchKeys() {
		if collapse {
			v.collapsed[k] = true
		} else {
			delete(v.collapsed, k)
		}
	}
	v.rowsDirty = true
}

func (v *View) hasBranches() bool {
	switch v.scope {
	case ScopeText:
		return len(v.text.files) > 0
	case ScopeSymbols:
		return v.st.SymbolsTree && len(v.sections[ScopeSymbols].items) > 0
	}
	return false
}

func (v *View) dismiss(r row) {
	switch r.kind {
	case rowMatch:
		f := r.file
		if r.match < len(f.matches) {
			f.matches = slices.Delete(slices.Clone(f.matches), r.match, r.match+1)
		}
		if len(f.matches) == 0 {
			v.removeFiles(func(x *fileNode) bool { return x == f })
		}
	case rowFile:
		v.removeFiles(func(x *fileNode) bool { return x == r.file })
	case rowFolder:
		prefix := strings.TrimPrefix(r.key, "d:") + "/"
		v.removeFiles(func(x *fileNode) bool { return strings.HasPrefix(x.rel, prefix) })
	}
	v.recount()
	v.rowsDirty = true
}

func (v *View) removeFiles(match func(*fileNode) bool) {
	v.text.files = slices.DeleteFunc(v.text.files, match)
}

func (v *View) ClearResults() {
	v.cancelAll()
	v.st.Query = ""
	v.text = textResults{}
	v.sections = [scopeCount]section{}
	v.selected, v.selectedKey = -1, ""
	v.rowsDirty = true
	v.field.Focus()
}

func (v *View) Refresh() {
	v.sess = nil
	v.searchSoon(0)
}

func (v *View) IndexChanged() {
	if v.scope == ScopeText || strings.TrimSpace(v.st.Query) == "" {
		return
	}
	v.Refresh()
}

func (r row) canDismiss() bool {
	return r.kind == rowMatch || r.kind == rowFile || r.kind == rowFolder
}

func (v *View) rowText(r row) string {
	switch r.kind {
	case rowMatch:
		return strings.TrimSpace(r.file.matches[r.match].Preview)
	case rowFile:
		return v.fileText(r.file)
	case rowFolder:
		return strings.TrimPrefix(r.key, "d:")
	case rowResult:
		return r.res.Label
	}
	return r.label
}

func (v *View) fileText(f *fileNode) string {
	var b strings.Builder
	b.WriteString(f.rel)
	for _, m := range f.matches {
		b.WriteString("\n  " + strconv.Itoa(m.Start.Line+1) + ": " + strings.TrimSpace(m.Preview))
	}
	return b.String()
}

func (v *View) allText() string {
	var parts []string
	for _, f := range v.text.files {
		parts = append(parts, v.fileText(f))
	}
	return strings.Join(parts, "\n")
}

func (r row) path() string {
	switch {
	case r.file != nil:
		return r.file.path
	case r.res.Path != "":
		return r.res.Path
	}
	return ""
}

func (v *View) summary() string {
	n, m := v.text.total, len(v.text.files)
	return fmt.Sprintf("%s in %s", plural(n, "result"), plural(m, "file"))
}

func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return strconv.Itoa(n) + " " + noun + "s"
}
