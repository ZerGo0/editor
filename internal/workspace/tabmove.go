package workspace

import (
	"slices"

	"github.com/ZerGo0/editor/internal/layout"
)

type tabDrag struct {
	group *layout.Group
	tab   *layout.Tab
}

func (w *Workspace) moveTab(d tabDrag, to *layout.Group, at int, copying bool) {
	from, tab := d.group, d.tab
	if from.Index(tab) < 0 {
		return
	}
	if from == to {
		if at != layout.AfterActive {
			to.Move(tab, at)
		}
		w.activate(to, tab)
		return
	}
	target := tab
	switch j := to.Find(tab.Path); {
	case j >= 0:
		target = to.Tabs[j]
	case copying:
		target = &layout.Tab{ID: w.newID(), Path: tab.Path}
		to.Insert(target, at)
	default:
		tab.Preview = false
		to.Insert(tab, at)
	}
	if !copying {
		w.detachTab(from, tab)
	}
	w.activate(to, target)
}

func (w *Workspace) detachTab(g *layout.Group, t *layout.Tab) {
	wasActive := g.ActiveTab() == t
	g.Close(g.Index(t))
	switch {
	case len(g.Tabs) == 0:
		w.layout.Remove(g)
	case wasActive:
		g.Active = w.mruTab(g)
	}
}

func (w *Workspace) addGroup(g *layout.Group, d layout.Direction) *layout.Group {
	added := w.newGroup()
	if !w.layout.Add(g, added, d) {
		return nil
	}
	return added
}

func (w *Workspace) SplitEditor(d layout.Direction) {
	g := w.active
	t := g.ActiveTab()
	if t == nil {
		return
	}
	if added := w.addGroup(g, d); added != nil {
		w.moveTab(tabDrag{g, t}, added, layout.AfterActive, true)
	}
}

func (w *Workspace) moveActiveTo(target func(from *layout.Group) *layout.Group) {
	from := w.active
	t := from.ActiveTab()
	if t == nil {
		return
	}
	if to := target(from); to != nil && to != from {
		w.moveTab(tabDrag{from, t}, to, layout.AfterActive, false)
	}
}

func (w *Workspace) MoveEditorToSide(d layout.Direction) {
	w.moveActiveTo(func(from *layout.Group) *layout.Group {
		if n := w.layout.Neighbor(from, d); n != nil {
			return n
		}
		return w.addGroup(from, d)
	})
}

func (w *Workspace) MoveEditorToAdjacentGroup(delta int) {
	w.moveActiveTo(func(from *layout.Group) *layout.Group {
		groups := w.layout.Groups()
		if i := slices.Index(groups, from) + delta; i >= 0 && i < len(groups) {
			return groups[i]
		}
		if delta < 0 {
			return w.addGroup(from, layout.Left)
		}
		return w.addGroup(from, layout.Right)
	})
}

func (w *Workspace) MoveEditorToEdgeGroup(last bool) {
	w.moveActiveTo(func(*layout.Group) *layout.Group {
		groups := w.layout.Groups()
		if last {
			return groups[len(groups)-1]
		}
		return groups[0]
	})
}

func (w *Workspace) MoveEditorInGroup(delta int) {
	g := w.active
	t := g.ActiveTab()
	if t == nil {
		return
	}
	i := g.Index(t)
	j := min(max(i+delta, 0), len(g.Tabs)-1)
	if j == i {
		return
	}
	if j > i {
		j++
	}
	g.Move(t, j)
}
