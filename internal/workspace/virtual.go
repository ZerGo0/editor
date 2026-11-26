package workspace

import (
	"path/filepath"
	"strings"

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
	close      func()
}

func isVirtual(p string) bool {
	return strings.HasPrefix(p, virtualPrefix)
}

func (w *Workspace) initVirtual() {
	w.virtual = map[string]*virtualTab{
		SettingsTab:    {title: "Settings", view: func(c *ui.Context) { w.settingsView.View(c) }, persistent: true},
		KeybindingsTab: {title: "Keyboard Shortcuts", view: func(c *ui.Context) { w.keysView.View(c) }, persistent: true},
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
	v, ok := w.virtual[p]
	return !isVirtual(p) || ok && v.persistent
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
