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
	name  string
	path  string
	depth int
	dir   bool
	entry int
}

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
	t.rows = t.rows[:0]
	t.fileRow = map[string]int{}
	needle := strings.ToLower(t.filter)
	root := &treeNode{}
	for i := range v.entries {
		e := &v.entries[i]
		if !e.isFile() || needle != "" && !strings.Contains(strings.ToLower(e.Path), needle) {
			continue
		}
		n := root
		if dir := path.Dir(e.Path); dir != "." {
			for _, part := range strings.Split(dir, "/") {
				n = n.dir(part)
			}
		}
		n.files = append(n.files, i)
	}
	v.flattenTree(root, 0)
	t.dirty, t.built = false, t.filter
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

func (v *View) sidebar(c *ui.Context) {
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
	p := token.Of(c)
	ui.Column(c.Key("multidiff-tree")).Fill().Background(p.Layer.SideBar).Children(func() {
		ui.Row(c).FillWidth().Padding(token.GapInline.Px(), token.PadRow.Px()).Shrink(0).Children(func() {
			field.SearchInput(c, filterLabel, &t.filter, field.SearchOptions{Placeholder: "Filter files", ID: "tree-filter"}).Grow(1).MinWidth(0)
		})
		list.View(c, &t.list, len(t.rows), list.ViewOptions{Label: treeLabel, Tree: true, Guides: true, Empty: func(c *ui.Context) {
			feedback.Empty(c, feedback.EmptyOptions{Icon: icons.Filter, Title: "No matching files"})
		}}, func(i int) list.Item { return v.treeItem(t.rows[i]) })
	})
}

func (v *View) treeItem(r treeRow) list.Item {
	if r.dir {
		open := !v.tree.closed[r.path]
		return list.Item{Label: r.name, Depth: r.depth, Branch: true, Open: open, Tooltip: r.path,
			Leading: func(c *ui.Context) { icons.FileIcon(c, path.Base(r.path), true, open, token.IconSize.Px()) },
			OnClick: func() { v.tree.closed[r.path] = !v.tree.closed[r.path]; v.tree.dirty = true }}
	}
	e := &v.entries[r.entry]
	it := list.Item{
		Label: r.name, Depth: r.depth, Git: e.Status, Tooltip: e.Path, Muted: v.viewed(e),
		Leading: func(c *ui.Context) { icons.FileIcon(c, r.name, false, false, token.IconSize.Px()) },
		OnClick: func() { v.Reveal(e.Key) },
	}
	if e.Adds != CountUnknown {
		it.Description = fmt.Sprintf("+%d -%d", e.Adds, e.Dels)
	}
	return it
}
