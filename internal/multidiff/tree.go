package multidiff

import (
	"fmt"
	"path"
	"strings"

	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/field"
	"github.com/ZerGo0/editor/internal/design/list"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const (
	treeLabel   = "Changed Files"
	filterLabel = "Filter Files"
)

type treeRow struct {
	name    string
	path    string
	depth   int
	dir     bool
	section bool
	entry   int
}

const sectionPathPrefix = "\x00"

type treeState struct {
	shown   bool
	forced  bool
	width   float32
	filter  string
	built   string
	closed  map[string]bool
	list    ui.ListState
	sel     int
	rows    []treeRow
	fileRow map[string]int
	dirty   bool
}

func newTree() treeState {
	return treeState{shown: true, width: token.SidebarMinWidth.Px(), closed: map[string]bool{}, sel: -1, dirty: true}
}

type treeNode struct {
	name  string
	path  string
	dirs  []*treeNode
	files []int
}

func (n *treeNode) dir(name string) *treeNode {
	for _, d := range n.dirs {
		if d.name == name {
			return d
		}
	}
	d := &treeNode{name: name, path: path.Join(n.path, name)}
	n.dirs = append(n.dirs, d)
	return d
}

func (v *View) buildTree() {
	t := &v.tree
	if t.built != t.filter {
		t.list.ScrollTo(0, ui.Start)
	}
	t.rows = t.rows[:0]
	t.fileRow = map[string]int{}
	groups := v.treeGroups(strings.ToLower(t.filter))
	for _, g := range groups {
		depth := 0
		if len(groups) > 1 && g.section >= 0 {
			s := &v.entries[g.section]
			p := sectionPathPrefix + s.Key
			t.rows = append(t.rows, treeRow{name: s.Title, path: p, dir: true, section: true, entry: g.section})
			if t.closed[p] {
				continue
			}
			depth = 1
		}
		v.flattenTree(g.root, depth)
	}
	t.dirty, t.built = false, t.filter
}

type treeGroup struct {
	section int
	root    *treeNode
}

func (v *View) treeGroups(needle string) []treeGroup {
	var groups []treeGroup
	cur := treeGroup{section: -1, root: &treeNode{}}
	flush := func() {
		if len(cur.root.dirs) > 0 || len(cur.root.files) > 0 {
			groups = append(groups, cur)
		}
	}
	for i := range v.entries {
		e := &v.entries[i]
		if !e.isFile() {
			flush()
			cur = treeGroup{section: i, root: &treeNode{}}
			continue
		}
		if needle != "" && !strings.Contains(strings.ToLower(e.Path), needle) {
			continue
		}
		n := cur.root
		if dir := path.Dir(e.Path); dir != "." {
			for _, part := range strings.Split(dir, "/") {
				n = n.dir(part)
			}
		}
		n.files = append(n.files, i)
	}
	flush()
	return groups
}

func (v *View) flattenTree(n *treeNode, depth int) {
	t := &v.tree
	for _, d := range n.dirs {
		name := d.name
		for len(d.dirs) == 1 && len(d.files) == 0 {
			d = d.dirs[0]
			name += "/" + d.name
		}
		t.rows = append(t.rows, treeRow{name: name, path: d.path, depth: depth, dir: true, entry: -1})
		if !t.closed[d.path] {
			v.flattenTree(d, depth+1)
		}
	}
	for _, i := range n.files {
		e := &v.entries[i]
		t.fileRow[e.Key] = len(t.rows)
		t.rows = append(t.rows, treeRow{name: path.Base(e.Path), path: e.Path, depth: depth, entry: i})
	}
}

func (v *View) syncTree() {
	t := &v.tree
	if t.dirty || t.built != t.filter {
		v.buildTree()
	}
	t.sel = -1
	if v.top >= 0 && v.top < len(v.entries) {
		if r, ok := t.fileRow[v.entries[v.top].Key]; ok {
			t.sel = r
		}
	}
	t.list.Selected = &t.sel
}

func (v *View) sidebar(c *ui.Context) {
	t := &v.tree
	p := token.Of(c)
	ui.Column(c.Key("multidiff-tree")).Fill().Background(p.Layer.SideBar).Children(func() {
		ui.Row(c).FillWidth().Padding(token.GapInline.Px(), token.PadRow.Px()).Shrink(0).Children(func() {
			field.SearchInput(c, filterLabel, &t.filter, field.SearchOptions{Placeholder: "Filter files", ID: "tree-filter"}).Grow(1).MinWidth(0)
		})
		v.syncTree()
		list.View(c, &t.list, len(t.rows), list.ViewOptions{Label: treeLabel, Tree: true, Guides: true, Empty: func(c *ui.Context) {
			feedback.Empty(c, feedback.EmptyOptions{Icon: icons.Filter, Title: "No matching files"})
		}}, func(i int) list.Item { return v.treeItem(t.rows[i]) })
	})
}

func (v *View) toggleTreeRow(p string) {
	v.tree.closed[p] = !v.tree.closed[p]
	v.tree.dirty = true
}

func (v *View) treeItem(r treeRow) list.Item {
	if r.dir {
		open := !v.tree.closed[r.path]
		if r.section {
			return list.Item{Label: r.name, Depth: r.depth, Branch: true, Open: open, Strong: true, Tooltip: r.name, OnClick: func() { v.toggleTreeRow(r.path) }}
		}
		return list.Item{Label: r.name, Depth: r.depth, Branch: true, Open: open, Tooltip: r.path,
			Leading: func(c *ui.Context) { icons.FileIcon(c, path.Base(r.path), true, open, token.IconSize.Px()) },
			OnClick: func() { v.toggleTreeRow(r.path) }}
	}
	e := &v.entries[r.entry]
	it := list.Item{
		Label: r.name, Depth: r.depth, Git: e.Status, Tooltip: e.Path, Muted: v.viewed(e),
		Leading: func(c *ui.Context) { icons.FileIcon(c, r.name, false, false, token.IconSize.Px()) }, LeadingWidth: token.IconSize.Px(),
		OnClick: func() { v.Reveal(e.Key) },
	}
	if e.Adds != CountUnknown {
		it.Description = fmt.Sprintf("+%d -%d", e.Adds, e.Dels)
	}
	return it
}
