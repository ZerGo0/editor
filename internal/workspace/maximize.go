package workspace

import (
	"slices"

	"github.com/ZerGo0/editor/internal/layout"
)

type hiddenParts struct {
	sidebar, panel bool
}

func (w *Workspace) maximizedGroup() *layout.Group {
	if w.maxGroup != nil && (w.maxGroup != w.active || !slices.Contains(w.layout.Groups(), w.maxGroup)) {
		w.maxGroup = nil
	}
	return w.maxGroup
}

func (w *Workspace) ToggleMaximizeEditorGroup() {
	switch {
	case w.maximizedGroup() != nil:
		w.maxGroup = nil
	case len(w.layout.Groups()) > 1:
		w.maxGroup = w.active
	}
	w.focusTab = w.active.ActiveTab()
}

func (w *Workspace) ToggleHideToolWindows() {
	if !w.sidebarHidden || w.tools.panelOpen {
		w.hidden = &hiddenParts{sidebar: !w.sidebarHidden, panel: w.tools.panelOpen}
		w.sidebarHidden, w.tools.panelOpen = true, false
		if len(w.layout.Groups()) > 1 {
			w.maxGroup = w.active
		}
	} else if h := w.hidden; h != nil {
		w.sidebarHidden, w.tools.panelOpen = !h.sidebar, h.panel
		w.maxGroup = nil
		w.hidden = nil
	}
	w.focusTab = w.active.ActiveTab()
}
