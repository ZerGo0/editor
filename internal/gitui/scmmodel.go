package gitui

import (
	"path"
	"slices"
	"strings"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/git"
)

type group int

const (
	groupMerge group = iota
	groupStaged
	groupChanges
	groupCount
)

var (
	groupTitles = [groupCount]string{"Merge Changes", "Staged Changes", "Changes"}
	groupIDs    = [groupCount]string{"merge", "staged", "changes"}
)

const (
	codeUntracked = '?'
	codeConflict  = 'U'
	codeDeleted   = 'D'
	codeAdded     = 'A'
	codeModified  = 'M'
	keyGroup      = "g:"
	keyDir        = ":d:"
	keyFile       = ":f:"
	rootDir       = "."
	noRow         = -1
)

type changeRow struct {
	group group
	file  git.FileStatus
}

func (r changeRow) untracked() bool {
	return r.group == groupChanges && r.file.Index == codeUntracked
}

func (r changeRow) code() byte {
	switch {
	case r.group == groupMerge:
		return codeConflict
	case r.group == groupStaged:
		return r.file.Index
	case r.untracked():
		return codeUntracked
	}
	return r.file.WorkTree
}

func (r changeRow) key() string {
	return groupIDs[r.group] + keyFile + r.file.Path
}

func gitStatus(code byte) token.GitStatus {
	switch code {
	case codeAdded:
		return token.GitAdded
	case 'C':
		return token.GitCopied
	case codeDeleted:
		return token.GitDeleted
	case 'R':
		return token.GitRenamed
	case codeUntracked:
		return token.GitUntracked
	case codeConflict:
		return token.GitConflict
	case statusIgnored:
		return token.GitIgnored
	case 0, ' ', '.':
		return token.GitNone
	}
	return token.GitModified
}

func changeGroups(st *git.Status) [groupCount][]changeRow {
	var out [groupCount][]changeRow
	if st == nil {
		return out
	}
	add := func(gr group, files []git.FileStatus) {
		for _, f := range files {
			out[gr] = append(out[gr], changeRow{group: gr, file: f})
		}
	}
	add(groupMerge, st.Conflicted())
	add(groupStaged, st.Staged())
	add(groupChanges, st.Unstaged())
	add(groupChanges, st.Untracked())
	for gr := range out {
		slices.SortFunc(out[gr], func(a, b changeRow) int { return strings.Compare(a.file.Path, b.file.Path) })
	}
	return out
}

func rowPaths(rows []changeRow) []string {
	paths := make([]string, 0, len(rows))
	for _, r := range rows {
		paths = append(paths, r.file.Path)
	}
	return paths
}

func rowsIn(rows []changeRow, gr group) []changeRow {
	var out []changeRow
	for _, r := range rows {
		if r.group == gr {
			out = append(out, r)
		}
	}
	return out
}

type changeNode struct {
	label    string
	path     string
	row      int
	children []string
}

type changeTree struct {
	roots []string
	nodes map[string]*changeNode
}

func buildChangeTree(gr group, rows []changeRow) changeTree {
	t := changeTree{nodes: map[string]*changeNode{}}
	prefix := groupIDs[gr] + keyDir
	var dirOf func(dir string) string
	attach := func(parent, key string) {
		if parent == rootDir {
			t.roots = append(t.roots, key)
			return
		}
		n := t.nodes[dirOf(parent)]
		n.children = append(n.children, key)
	}
	dirOf = func(dir string) string {
		key := prefix + dir
		if _, ok := t.nodes[key]; ok {
			return key
		}
		t.nodes[key] = &changeNode{label: path.Base(dir), path: dir, row: noRow}
		attach(path.Dir(dir), key)
		return key
	}
	for i, r := range rows {
		key := r.key()
		t.nodes[key] = &changeNode{label: path.Base(r.file.Path), path: r.file.Path, row: i}
		attach(path.Dir(r.file.Path), key)
	}
	t.roots = t.sorted(t.roots)
	for _, k := range t.roots {
		t.compact(k)
	}
	return t
}

func (t changeTree) sorted(keys []string) []string {
	slices.SortFunc(keys, func(a, b string) int {
		na, nb := t.nodes[a], t.nodes[b]
		if da, db := na.row == noRow, nb.row == noRow; da != db {
			if da {
				return -1
			}
			return 1
		}
		return strings.Compare(strings.ToLower(na.label), strings.ToLower(nb.label))
	})
	return keys
}

func (t changeTree) compact(key string) {
	n := t.nodes[key]
	if n.row != noRow {
		return
	}
	for len(n.children) == 1 {
		child := t.nodes[n.children[0]]
		if child.row != noRow {
			break
		}
		n.label += "/" + child.label
		n.path = child.path
		n.children = child.children
	}
	n.children = t.sorted(n.children)
	for _, k := range n.children {
		t.compact(k)
	}
}

func (t changeTree) rowsUnder(key string, out []int) []int {
	n := t.nodes[key]
	if n.row != noRow {
		return append(out, n.row)
	}
	for _, k := range n.children {
		out = t.rowsUnder(k, out)
	}
	return out
}

type entryKind int

const (
	entryGroup entryKind = iota
	entryDir
	entryFile
)

type changeEntry struct {
	kind  entryKind
	key   string
	group group
	depth int
	label string
	path  string
	rows  []changeRow
	open  bool
}

func groupKey(gr group) string {
	return keyGroup + groupIDs[gr]
}

func fileEntry(r changeRow, depth int) changeEntry {
	name, dir := splitPath(r.file.Path)
	return changeEntry{kind: entryFile, key: r.key(), group: r.group, depth: depth, label: name, path: dir, rows: []changeRow{r}}
}

func flattenChanges(groups [groupCount][]changeRow, tree bool, closed map[string]bool) []changeEntry {
	var out []changeEntry
	for gi, rows := range groups {
		gr := group(gi)
		if len(rows) == 0 && gr != groupChanges {
			continue
		}
		key := groupKey(gr)
		open := !closed[key]
		out = append(out, changeEntry{kind: entryGroup, key: key, group: gr, label: groupTitles[gr], rows: rows, open: open})
		if !open {
			continue
		}
		if !tree {
			for _, r := range rows {
				out = append(out, fileEntry(r, 1))
			}
			continue
		}
		t := buildChangeTree(gr, rows)
		var walk func(keys []string, depth int)
		walk = func(keys []string, depth int) {
			for _, k := range keys {
				n := t.nodes[k]
				if n.row != noRow {
					e := fileEntry(rows[n.row], depth)
					e.path = ""
					out = append(out, e)
					continue
				}
				var under []changeRow
				for _, i := range t.rowsUnder(k, nil) {
					under = append(under, rows[i])
				}
				open := !closed[k]
				out = append(out, changeEntry{kind: entryDir, key: k, group: gr, depth: depth, label: n.label, path: n.path, rows: under, open: open})
				if open {
					walk(n.children, depth+1)
				}
			}
		}
		walk(t.roots, 1)
	}
	return out
}

func selectedRows(entries []changeEntry, has func(string) bool) []changeRow {
	seen := map[string]bool{}
	var out []changeRow
	for _, e := range entries {
		if !has(e.key) {
			continue
		}
		for _, r := range e.rows {
			if k := r.key(); !seen[k] {
				seen[k] = true
				out = append(out, r)
			}
		}
	}
	return out
}

func planRollback(rows []changeRow) rollbackPlan {
	var p rollbackPlan
	for _, r := range rows {
		f := r.file
		switch {
		case r.untracked():
			p.untracked = append(p.untracked, f.Path)
		case r.group == groupMerge:
		case r.group == groupChanges:
			p.discard = append(p.discard, f.Path)
		case f.Index == codeAdded:
			p.unstage = append(p.unstage, f.Path)
		case f.OrigPath != "":
			p.unstage = append(p.unstage, f.Path, f.OrigPath)
			p.discard = append(p.discard, f.OrigPath)
		default:
			p.unstage = append(p.unstage, f.Path)
			p.discard = append(p.discard, f.Path)
		}
	}
	return p
}

type rollbackPlan struct {
	unstage   []string
	discard   []string
	untracked []string
}
