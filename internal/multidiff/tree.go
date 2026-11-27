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

func (v *View) buildTree() {
	t := &v.tree
	t.rows = t.rows[:0]
	t.fileRow = map[string]int{}
	needle := strings.ToLower(t.filter)
	var dirs []string
	for i := range v.entries {
		e := &v.entries[i]
		if !e.isFile() || needle != "" && !strings.Contains(strings.ToLower(e.Path), needle) {
			continue
		}
		parts := strings.Split(path.Dir(e.Path), "/")
		if path.Dir(e.Path) == "." {
			parts = nil
		}
		common := 0
		for common < len(dirs) && common < len(parts) && dirs[common] == parts[common] {
			common++
		}
		dirs = append(dirs[:common:common], parts[common:]...)
		hidden := false
		for d := range parts {
			p := strings.Join(parts[:d+1], "/")
			if d >= common {
				t.rows = append(t.rows, treeRow{name: parts[d], path: p, depth: d, dir: true, entry: -1})
			}
			if t.closed[p] {
				hidden = true
				break
			}
		}
		if hidden {
			continue
		}
		t.fileRow[e.Key] = len(t.rows)
		t.rows = append(t.rows, treeRow{name: path.Base(e.Path), path: e.Path, depth: len(parts), entry: i})
	}
	t.dirty, t.built = false, t.filter
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
		list.View(c, &t.list, len(t.rows), list.ViewOptions{Label: treeLabel, Empty: func(c *ui.Context) {
			feedback.Empty(c, feedback.EmptyOptions{Icon: icons.Filter, Title: "No matching files"})
		}}, func(i int) list.Item { return v.treeItem(t.rows[i]) })
	})
}

func (v *View) treeItem(r treeRow) list.Item {
	if r.dir {
		return list.Item{Label: r.name, Depth: r.depth, Branch: true, Open: !v.tree.closed[r.path], Tooltip: r.path,
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
