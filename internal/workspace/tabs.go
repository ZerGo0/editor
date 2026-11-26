package workspace

import (
	"slices"

	"github.com/ZerGo0/editor/internal/document"
	"github.com/ZerGo0/editor/internal/layout"
	"github.com/ZerGo0/editor/internal/settings"
)

func (w *Workspace) OpenFile(path string, preview, focus bool) {
	w.openIn(w.active, path, preview, focus)
}

func (w *Workspace) openIn(g *layout.Group, path string, preview, focus bool) *layout.Tab {
	preview = preview && w.settings.Bool(settings.EnablePreview)
	t := g.Open(path, preview, w.newID())
	w.active = g
	w.touchRecent(path)
	if focus {
		w.focusTab = t
	}
	w.dropUnused()
	return t
}

func (w *Workspace) activate(g *layout.Group, t *layout.Tab) {
	if i := g.Index(t); i >= 0 {
		g.Active = i
	}
	w.active = g
	w.focusTab = t
	w.touchRecent(t.Path)
}

func (w *Workspace) CloseActiveTab() {
	if t := w.active.ActiveTab(); t != nil {
		w.requestClose(w.active, []*layout.Tab{t})
	}
}

func (w *Workspace) requestClose(g *layout.Group, tabs []*layout.Tab) {
	dirty := w.unsavedOnlyIn(tabs)
	if len(dirty) == 0 || w.win == nil {
		w.removeTabs(g, tabs)
		return
	}
	names := make([]string, len(dirty))
	for i, d := range dirty {
		names[i] = w.tabTitle(d.Path)
	}
	go func() {
		choice := askSave(w.win, names)
		w.update(func() {
			switch choice {
			case choiceSave:
				w.saveThen(dirty, func() { w.removeTabs(g, tabs) })
			case choiceDiscard:
				w.removeTabs(g, tabs)
			}
		})
	}()
}

func (w *Workspace) unsavedOnlyIn(closing []*layout.Tab) []*document.Document {
	staying := map[string]bool{}
	for _, t := range w.tabs() {
		if !slices.Contains(closing, t) {
			staying[t.Path] = true
		}
	}
	var out []*document.Document
	seen := map[string]bool{}
	for _, t := range closing {
		d := w.doc(t.Path)
		if d == nil || !d.Modified() || staying[t.Path] || seen[t.Path] {
			continue
		}
		seen[t.Path] = true
		out = append(out, d)
	}
	return out
}

func (w *Workspace) removeTabs(g *layout.Group, tabs []*layout.Tab) {
	w.rememberClosed(tabs)
	for _, t := range tabs {
		g.Close(g.Index(t))
	}
	if len(g.Tabs) == 0 && w.layout.Remove(g) && w.active == g {
		w.active = w.layout.Groups()[0]
	}
	if t := w.active.ActiveTab(); t != nil {
		w.focusTab = t
	}
	w.dropUnused()
}

func unpinned(tabs []*layout.Tab, keep func(t *layout.Tab) bool) []*layout.Tab {
	return slices.DeleteFunc(slices.Clone(tabs), func(t *layout.Tab) bool { return t.Pinned || keep(t) })
}

func (w *Workspace) closeOthers(g *layout.Group, keep *layout.Tab) {
	w.requestClose(g, unpinned(g.Tabs, func(t *layout.Tab) bool { return t == keep }))
}

func (w *Workspace) closeToRight(g *layout.Group, from *layout.Tab) {
	w.requestClose(g, unpinned(g.Tabs[g.Index(from)+1:], func(*layout.Tab) bool { return false }))
}

func (w *Workspace) closeSaved(g *layout.Group) {
	w.requestClose(g, unpinned(g.Tabs, func(t *layout.Tab) bool {
		d := w.doc(t.Path)
		return d != nil && d.Modified()
	}))
}

func (w *Workspace) closeAll(g *layout.Group) {
	w.requestClose(g, unpinned(g.Tabs, func(*layout.Tab) bool { return false }))
}

func (w *Workspace) UnpinActive() {
	if t := w.active.ActiveTab(); t != nil {
		w.active.SetPinned(t, false)
	}
}

func (w *Workspace) TogglePinActive() {
	if t := w.active.ActiveTab(); t != nil {
		w.active.SetPinned(t, !t.Pinned)
	}
}

func (w *Workspace) Split(vertical bool) {
	t := w.active.ActiveTab()
	if t == nil {
		return
	}
	added := w.newGroup()
	w.focusTab = added.Open(t.Path, false, w.newID())
	if w.layout.Split(w.active, added, vertical) {
		w.active = added
	}
}

func (w *Workspace) CycleTab(delta int) {
	w.active.Cycle(delta)
	w.focusTab = w.active.ActiveTab()
}

func (w *Workspace) ToggleSidebar() {
	w.sidebarHidden = !w.sidebarHidden
}
