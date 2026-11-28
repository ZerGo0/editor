package workspace

import (
	"path/filepath"
	"strings"

	"github.com/ZerGo0/editor/internal/layout"

	"github.com/egoist/mygo/ui"
)

const (
	SettingsTab    = "editor://settings"
	KeybindingsTab = "editor://keybindings"
	virtualPrefix  = "editor://"
)

type virtualTab struct {
	title      string
	view       func(c *ui.Context)
	persistent bool
	icon       *ui.SVG
	close      func()
	focus      func()
}

func isVirtual(p string) bool {
	return strings.HasPrefix(p, virtualPrefix)
}

func (w *Workspace) initVirtual() {
	w.virtual = map[string]*virtualTab{
		SettingsTab:    {title: "Settings", view: func(c *ui.Context) { w.settingsView.View(c) }, persistent: true, focus: func() { w.settingsView.Focus() }},
		KeybindingsTab: {title: "Keyboard Shortcuts", view: func(c *ui.Context) { w.keysView.View(c) }, persistent: true, focus: func() { w.keysView.Focus() }},
	}
}

func (w *Workspace) OpenView(id, title string, view func(c *ui.Context)) {
	p := virtualPrefix + id
	w.virtual[p] = &virtualTab{title: title, view: view}
	w.OpenFile(p, false, true)
}

func (w *Workspace) tabTitle(p string) string {
	if v, ok := w.virtual[p]; ok {
		return v.title
	}
	if isUntitled(p) {
		return strings.TrimPrefix(p, untitledPrefix)
	}
	return filepath.Base(p)
}

func (w *Workspace) restorable(p string) bool {
	if isUntitled(p) {
		return w.hasBackup(p)
	}
	if !isVirtual(p) {
		return true
	}
	if _, pending := w.pendingViews[p]; pending {
		return true
	}
	v, ok := w.virtual[p]
	return ok && (v.persistent || w.restorerFor(p) != nil)
}

type viewRestorer func(id, title string) (func(c *ui.Context), bool)

func (w *Workspace) restorerFor(p string) viewRestorer {
	id := strings.TrimPrefix(p, virtualPrefix)
	for prefix, r := range w.restorers {
		if strings.HasPrefix(id, prefix) {
			return r
		}
	}
	return nil
}

func (w *Workspace) RestoreViews(prefix string, restore func(id, title string) (func(c *ui.Context), bool)) {
	w.restorers[prefix] = restore
	for p, title := range w.pendingViews {
		id := strings.TrimPrefix(p, virtualPrefix)
		if !strings.HasPrefix(id, prefix) {
			continue
		}
		delete(w.pendingViews, p)
		if view, ok := restore(id, title); ok {
			w.virtual[p] = &virtualTab{title: title, view: view}
		}
	}
}

func (w *Workspace) dropUnrestoredViews() {
	for _, g := range w.layout.Groups() {
		var gone []*layout.Tab
		for _, t := range g.Tabs {
			if isVirtual(t.Path) && w.virtual[t.Path] == nil {
				gone = append(gone, t)
			}
		}
		if len(gone) > 0 {
			w.dropTabs(g, gone)
		}
	}
	clear(w.pendingViews)
	w.focusTab = w.active.ActiveTab()
}

func (w *Workspace) dropUnusedViews(open map[string]bool) {
	for p, v := range w.virtual {
		if !v.persistent && !open[p] {
			if v.close != nil {
				v.close()
			}
			delete(w.virtual, p)
		}
	}
}
func (w *Workspace) dropTabs(g *layout.Group, gone []*layout.Tab) {
	for _, t := range gone {
		g.Close(g.Index(t))
	}
	if len(g.Tabs) == 0 && w.layout.Remove(g) && w.active == g {
		w.active = w.layout.Groups()[0]
	}
}
