package layout

import (
	"slices"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

type Position struct {
	Line, Col int
	ScrollY   float64
	Reveal    bool
	End       *editor.Pos
}

type Tab struct {
	ID      int
	Path    string
	Preview bool
	Editor  *editor.State
	Restore *Position
	Handle  ui.Handle
	Pinned  bool
}

const AfterActive = -1

type Group struct {
	ID     int
	Tabs   []*Tab
	Active int
}

func (g *Group) ActiveTab() *Tab {
	if g.Active < 0 || g.Active >= len(g.Tabs) {
		return nil
	}
	return g.Tabs[g.Active]
}

func (g *Group) Index(t *Tab) int {
	return slices.Index(g.Tabs, t)
}

func (g *Group) Find(path string) int {
	return slices.IndexFunc(g.Tabs, func(t *Tab) bool { return t.Path == path })
}

func (g *Group) Open(path string, preview bool, id int) *Tab {
	if i := g.Find(path); i >= 0 {
		g.Active = i
		if !preview {
			g.Tabs[i].Preview = false
		}
		return g.Tabs[i]
	}
	t := &Tab{ID: id, Path: path, Preview: preview}
	if preview {
		if i := slices.IndexFunc(g.Tabs, func(t *Tab) bool { return t.Preview }); i >= 0 {
			g.Tabs[i] = t
			g.Active = i
			return t
		}
	}
	at := g.afterActive()
	g.Tabs = slices.Insert(g.Tabs, at, t)
	g.Active = at
	return t
}

func (g *Group) afterActive() int {
	return max(min(g.Active+1, len(g.Tabs)), g.PinnedCount())
}

func (g *Group) Close(i int) {
	if i < 0 || i >= len(g.Tabs) {
		return
	}
	g.Tabs = slices.Delete(g.Tabs, i, i+1)
	switch {
	case i < g.Active:
		g.Active--
	case g.Active >= len(g.Tabs):
		g.Active = len(g.Tabs) - 1
	}
	g.Active = max(g.Active, 0)
}

func (g *Group) Cycle(delta int) {
	if n := len(g.Tabs); n > 0 {
		g.Active = ((g.Active+delta)%n + n) % n
	}
}

func (g *Group) PinnedCount() int {
	n := 0
	for _, t := range g.Tabs {
		if t.Pinned {
			n++
		}
	}
	return n
}

func (g *Group) SetPinned(t *Tab, pinned bool) {
	i := g.Index(t)
	if i < 0 || t.Pinned == pinned {
		return
	}
	active := g.ActiveTab()
	g.Tabs = slices.Delete(g.Tabs, i, i+1)
	at := g.PinnedCount()
	t.Pinned = pinned
	if pinned {
		t.Preview = false
	}
	g.Tabs = slices.Insert(g.Tabs, at, t)
	g.Active = g.Index(active)
}

func (g *Group) Insert(t *Tab, at int) {
	active := g.ActiveTab()
	keepPin := at == AfterActive && t.Pinned
	switch {
	case keepPin:
		at = g.PinnedCount()
	case at == AfterActive:
		at = g.afterActive()
	}
	at = min(max(at, 0), len(g.Tabs))
	t.Pinned = keepPin || at < g.PinnedCount()
	if t.Pinned {
		t.Preview = false
	}
	g.Tabs = slices.Insert(g.Tabs, at, t)
	g.Active = max(g.Index(active), 0)
}

func (g *Group) Move(t *Tab, to int) {
	i := g.Index(t)
	if i < 0 {
		return
	}
	g.Close(i)
	if to > i {
		to--
	}
	g.Insert(t, to)
	g.Active = g.Index(t)
}
