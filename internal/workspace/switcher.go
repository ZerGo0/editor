package workspace

import (
	"path/filepath"
	"slices"

	"github.com/ZerGo0/editor/internal/layout"
	"github.com/ZerGo0/editor/internal/palette"
	"github.com/egoist/mygo/ui"
)

const switcherTitle = "Open Editors"

type switcherPick struct {
	group *layout.Group
	tab   *layout.Tab
}

func (w *Workspace) observeInput(ev ui.InputEvent) {
	w.observeShift(ev)
	w.observeSwitcher(ev)
	w.observeCodeKey(ev)
}

func (w *Workspace) switcherOpen() bool {
	return w.palette.IsOpen() && w.palette.Label() == switcherTitle
}

func (w *Workspace) observeSwitcher(ev ui.InputEvent) {
	switch {
	case ev.Kind == ui.InputKeyDown && ev.Key == ui.KeyTab && ev.Mods&^ui.Shift == ui.Ctrl:
		d := 1
		if ev.Mods&ui.Shift != 0 {
			d = -1
		}
		if !w.switcherOpen() {
			w.showSwitcher()
		}
		w.palette.Move(d)
	case ev.Kind == ui.InputKeyUp && (ev.Code == ui.CodeControlLeft || ev.Code == ui.CodeControlRight) && w.switcherOpen():
		w.palette.Accept()
	}
}

func (w *Workspace) showSwitcher() {
	var picks []switcherPick
	for _, g := range w.layout.Groups() {
		for _, t := range g.Tabs {
			picks = append(picks, switcherPick{group: g, tab: t})
		}
	}
	rank := func(p switcherPick) int {
		if p.tab == w.active.ActiveTab() {
			return -1
		}
		if i := slices.Index(w.search.recent, p.tab.Path); i >= 0 {
			return i
		}
		return len(w.search.recent)
	}
	slices.SortStableFunc(picks, func(a, b switcherPick) int { return rank(a) - rank(b) })
	labels := w.tabLabels()
	items := make([]palette.Item, len(picks))
	for i, p := range picks {
		path := p.tab.Path
		items[i] = palette.Item{Label: w.tabTitle(path), Value: p, Leading: func(c *ui.Context) { w.tabIcon(c, path) }}
		switch {
		case labels[path] != "":
			items[i].Detail = labels[path]
		case !isVirtual(path) && w.rel(filepath.Dir(path)) != ".":
			items[i].Detail = w.rel(filepath.Dir(path))
		}
	}
	w.palette.Show(palette.Mode{
		Label:       switcherTitle,
		Placeholder: "Select an open editor",
		Search:      palette.Static(items),
		Accept:      func(it palette.Item) { w.openPick(it.Value.(switcherPick)) },
	})
}

func (w *Workspace) openPick(p switcherPick) {
	if p.group.Index(p.tab) < 0 {
		return
	}
	w.activate(p.group, p.tab)
}
