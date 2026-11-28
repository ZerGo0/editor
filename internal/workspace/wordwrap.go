package workspace

import (
	"cmp"

	"github.com/ZerGo0/editor/internal/settings"
	"github.com/egoist/mygo/ui/editor"
)

func (w *Workspace) wrapping(diff bool, path string) (on bool, column int) {
	if override, ok := w.wrapOverrides[w.drawingTab]; ok && diff && w.drawingTab != "" {
		return override, 0
	}
	s := w.settings
	mode := s.String(settings.WordWrap)
	if diff {
		if m := s.String(settings.DiffWordWrap); m != settings.WrapInherit {
			mode = m
		}
	}
	if override, ok := w.wrapOverrides[path]; ok {
		return override, 0
	}
	if mode == settings.WrapColumn || mode == settings.WrapBounded {
		column = s.Int(settings.WordWrapColumn)
	}
	return mode != settings.WrapOff, column
}

func (w *Workspace) wrapTarget() (path string, diff bool) {
	if t := w.active.ActiveTab(); t != nil && t.Editor != nil {
		return t.Path, false
	}
	return cmp.Or(w.diffPath, w.lastDiffPath), true
}

func (w *Workspace) ToggleWordWrap() {
	path, diff := w.wrapTarget()
	if path == "" {
		return
	}
	if _, ok := w.wrapOverrides[path]; ok {
		delete(w.wrapOverrides, path)
		return
	}
	on, _ := w.wrapping(diff, path)
	if w.wrapOverrides == nil {
		w.wrapOverrides = map[string]bool{}
	}
	w.wrapOverrides[path] = !on
}

func (w *Workspace) trackDiff(path string) {
	key := cmp.Or(w.drawingTab, path)
	if t := w.active.ActiveTab(); w.diffPath == "" || t != nil && t.Path == key {
		w.diffPath = key
	}
}

func (w *Workspace) nextDiffFrame() {
	w.lastDiffPath, w.diffPath = w.diffPath, ""
}

func (w *Workspace) dropUnusedWrapping(open map[string]bool) {
	for path := range w.wrapOverrides {
		if !open[path] {
			delete(w.wrapOverrides, path)
		}
	}
}

func (w *Workspace) applyWrapping(st *editor.State, path string) {
	st.SoftWrap, st.WrapColumn = w.wrapping(false, path)
}
