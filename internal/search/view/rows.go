package searchview

import (
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/ZerGo0/editor/internal/search/everywhere"
	"github.com/ZerGo0/editor/internal/search/textsearch"
)

const (
	allSectionLimit  = 5
	quickLimit       = 200
	actionScopeLimit = 2000
	recentLimit      = 8
	recentQueries    = 5
)

type rowKind uint8

const (
	rowHeader rowKind = iota
	rowFolder
	rowFile
	rowMatch
	rowResult
	rowGroup
	rowRecentQuery
	rowGoto
	rowMessage
)

type row struct {
	kind   rowKind
	key    string
	depth  int
	label  string
	title  string
	detail string
	open   bool
	branch bool
	count  int
	total  int
	scope  Scope
	res    everywhere.Result
	file   *fileNode
	match  int
	query  string
	line   int
	col    int
}

type fileNode struct {
	path    string
	rel     string
	name    string
	dir     string
	matches []textsearch.Match
}

func newFileNode(abs, rel string, matches []textsearch.Match) *fileNode {
	dir := ""
	if i := strings.LastIndexByte(rel, '/'); i >= 0 {
		dir = rel[:i]
	}
	return &fileNode{path: abs, rel: rel, name: path.Base(rel), dir: dir, matches: matches}
}

type dirNode struct {
	name  string
	path  string
	dirs  []*dirNode
	files []*fileNode
	index map[string]*dirNode
	count int
}

func buildTree(files []*fileNode) *dirNode {
	root := &dirNode{index: map[string]*dirNode{}}
	for _, f := range files {
		node := root
		if f.dir != "" {
			for _, part := range strings.Split(f.dir, "/") {
				child := node.index[part]
				if child == nil {
					child = &dirNode{name: part, path: joinPath(node.path, part), index: map[string]*dirNode{}}
					node.index[part] = child
					node.dirs = append(node.dirs, child)
				}
				node = child
			}
		}
		node.files = append(node.files, f)
	}
	root.finish()
	return root
}

func joinPath(dir, name string) string {
	if dir == "" {
		return name
	}
	return dir + "/" + name
}

func (n *dirNode) finish() {
	slices.SortFunc(n.dirs, func(a, b *dirNode) int { return strings.Compare(a.name, b.name) })
	n.count = 0
	for i, d := range n.dirs {
		d.finish()
		for len(d.dirs) == 1 && len(d.files) == 0 {
			only := d.dirs[0]
			only.name = d.name + "/" + only.name
			d = only
		}
		n.dirs[i] = d
		n.count += d.count
	}
	for _, f := range n.files {
		n.count += len(f.matches)
	}
}

func matchKey(f *fileNode, m textsearch.Match) string {
	return "m:" + f.path + ":" + strconv.Itoa(m.Start.Line) + ":" + strconv.Itoa(m.Start.Col) + ":" + strconv.Itoa(m.End.Line) + ":" + strconv.Itoa(m.End.Col)
}

func (v *View) textRows(out []row, depth int) []row {
	files := v.text.files
	if v.st.ListView {
		for _, f := range files {
			out = v.fileRows(out, f, depth)
		}
		return out
	}
	return v.dirRows(out, buildTree(files), depth)
}

func (v *View) dirRows(out []row, n *dirNode, depth int) []row {
	for _, d := range n.dirs {
		key := "d:" + d.path
		open := !v.collapsed[key]
		out = append(out, row{kind: rowFolder, key: key, depth: depth, title: d.name, label: d.path, open: open, branch: true, count: d.count})
		if open {
			out = v.dirRows(out, d, depth+1)
		}
	}
	for _, f := range n.files {
		out = v.fileRows(out, f, depth)
	}
	return out
}

func (v *View) fileRows(out []row, f *fileNode, depth int) []row {
	key := "f:" + f.path
	open := !v.collapsed[key]
	out = append(out, row{kind: rowFile, key: key, depth: depth, title: f.name, detail: f.dir, label: f.rel, open: open, branch: true, count: len(f.matches), file: f})
	if !open {
		return out
	}
	for i, m := range f.matches {
		out = append(out, row{kind: rowMatch, key: matchKey(f, m), depth: depth + 1, label: "line " + strconv.Itoa(m.Start.Line+1) + ": " + strings.TrimSpace(m.Preview), file: f, match: i})
	}
	return out
}

func (v *View) buildRows() []row {
	switch v.scope {
	case ScopeText:
		return v.textScopeRows()
	case ScopeAll:
		return v.allRows()
	}
	return v.quickScopeRows(v.scope)
}

func (v *View) textScopeRows() []row {
	if strings.TrimSpace(v.st.Query) == "" {
		return v.historyRows(nil, recentQueries, true)
	}
	if v.text.err != "" {
		return nil
	}
	return v.textRows(nil, 0)
}

func (v *View) historyRows(out []row, limit int, header bool) []row {
	n := min(len(v.st.History), limit)
	if n == 0 {
		return out
	}
	if header {
		out = append(out, row{kind: rowHeader, key: "h:history", title: "Recent searches", scope: ScopeText})
	}
	for _, q := range v.st.History[:n] {
		out = append(out, row{kind: rowRecentQuery, key: "r:" + q, label: q, query: q})
	}
	return out
}

func (v *View) section(s Scope) *section { return &v.sections[s] }

func (v *View) sectionRows(out []row, s Scope, limit int, showAll bool) []row {
	sec := v.section(s)
	if len(sec.items) == 0 {
		return out
	}
	if limit > 0 {
		out = append(out, row{kind: rowHeader, key: "h:" + s.String(), title: s.String(), count: sec.total, total: sec.total, scope: s})
	}
	items := sec.items
	if limit > 0 {
		items = items[:min(len(items), limit)]
	}
	if s == ScopeSymbols && v.st.SymbolsTree && limit == 0 {
		return v.groupedRows(out, items)
	}
	for _, r := range items {
		out = append(out, row{kind: rowResult, key: resultKey(s, r), res: r, label: resultLabel(r), line: r.Line, col: r.Col})
	}
	return out
}

func (v *View) groupedRows(out []row, items []everywhere.Result) []row {
	var order []string
	groups := map[string][]everywhere.Result{}
	for _, r := range items {
		if _, ok := groups[r.Path]; !ok {
			order = append(order, r.Path)
		}
		groups[r.Path] = append(groups[r.Path], r)
	}
	for _, p := range order {
		key := "g:" + p
		open := !v.collapsed[key]
		first := groups[p][0]
		out = append(out, row{kind: rowGroup, key: key, title: path.Base(first.Detail), detail: path.Dir(first.Detail), label: first.Detail, open: open, branch: true, count: len(groups[p]), res: first})
		if !open {
			continue
		}
		for _, r := range groups[p] {
			out = append(out, row{kind: rowResult, key: resultKey(ScopeSymbols, r), depth: 1, res: r, label: resultLabel(r), line: r.Line, col: r.Col})
		}
	}
	return out
}

func resultKey(s Scope, r everywhere.Result) string {
	return "q:" + s.String() + ":" + strconv.Itoa(int(r.Kind)) + ":" + r.Path + ":" + r.ActionID + ":" + r.Ref.Name + ":" + r.OID + ":" + r.Label + ":" + strconv.Itoa(r.Line)
}

func resultLabel(r everywhere.Result) string {
	switch r.Kind {
	case everywhere.KindFile:
		return path.Join(r.Detail, r.Label)
	case everywhere.KindSymbol:
		return r.Label + " in " + r.Detail
	}
	return r.Label
}

func (v *View) allRows() []row {
	var out []row
	if strings.TrimSpace(v.st.Query) == "" {
		if rs := v.section(ScopeFiles).items; len(rs) > 0 {
			out = append(out, row{kind: rowHeader, key: "h:recent", title: "Recent files", scope: ScopeFiles})
			for _, r := range rs[:min(len(rs), recentLimit)] {
				out = append(out, row{kind: rowResult, key: resultKey(ScopeFiles, r), res: r, label: resultLabel(r)})
			}
		}
		return v.historyRows(out, recentQueries, true)
	}
	out = v.sectionRows(out, ScopeFiles, allSectionLimit, true)
	out = v.sectionRows(out, ScopeSymbols, allSectionLimit, true)
	out = v.textSectionRows(out)
	out = v.sectionRows(out, ScopeActions, allSectionLimit, true)
	out = v.sectionRows(out, ScopeGit, allSectionLimit, true)
	if len(out) == 0 && !v.busy() {
		out = append(out, v.emptyRow())
	}
	return out
}

func (v *View) textSectionRows(out []row) []row {
	if v.text.err != "" || v.text.total == 0 {
		return out
	}
	out = append(out, row{kind: rowHeader, key: "h:Text", title: "Text", count: v.text.total, total: v.text.total, scope: ScopeText})
	shown := 0
	for _, f := range v.text.files {
		for i, m := range f.matches {
			if shown == allSectionLimit {
				return out
			}
			out = append(out, row{kind: rowMatch, key: matchKey(f, m), label: "line " + strconv.Itoa(m.Start.Line+1) + ": " + strings.TrimSpace(m.Preview), file: f, match: i, detail: f.rel})
			shown++
		}
	}
	return out
}

func (v *View) emptyRow() row {
	return row{kind: rowMessage, key: "msg:empty", title: "No results for " + strconv.Quote(firstLine(strings.TrimSpace(v.st.Query)))}
}

func (v *View) quickScopeRows(s Scope) []row {
	q := strings.TrimSpace(v.st.Query)
	var out []row
	if s == ScopeFiles {
		if fq := parseFileQuery(q); fq.hasLine && fq.path == "" {
			out = append(out, row{kind: rowGoto, key: "goto:" + strconv.Itoa(fq.line), line: fq.line, col: fq.col, label: "Go to line " + strconv.Itoa(fq.line+1)})
		}
	}
	out = v.sectionRows(out, s, 0, false)
	if len(out) == 0 && !v.busy() {
		out = append(out, v.quickHint(s, q))
	}
	return out
}

func (v *View) quickHint(s Scope, q string) row {
	hint := map[Scope]string{
		ScopeFiles:   "Type a file name. Add :line to jump to a line.",
		ScopeSymbols: "Type the name of a function, type or heading.",
		ScopeActions: "No matching actions.",
		ScopeGit:     "Type a branch, tag, commit hash or message.",
	}[s]
	if q != "" && s != ScopeGit {
		hint = "No results for " + strconv.Quote(firstLine(q))
	}
	return row{kind: rowMessage, key: "msg:hint", title: hint}
}
