package workspace

import (
	"github.com/ZerGo0/editor/internal/settings"
	"github.com/egoist/mygo/ui/editor"
)

func (w *Workspace) wrapping(diff bool) (on bool, column int) {
	s := w.settings
	mode := s.String(settings.WordWrap)
	if diff {
		if m := s.String(settings.DiffWordWrap); m != settings.WrapInherit {
			mode = m
		}
	}
	if w.wrapOverride != nil {
		return *w.wrapOverride, 0
	}
	if mode == settings.WrapColumn || mode == settings.WrapBounded {
		column = s.Int(settings.WordWrapColumn)
	}
	return mode != settings.WrapOff, column
}

// TECHDEBT: The toggle overrides wrapping in every editor and diff at once; VS Code overrides it per model. Track the focused editor's path to narrow it.
func (w *Workspace) ToggleWordWrap() {
	on, _ := w.wrapping(false)
	next := !on
	w.wrapOverride = &next
}

func (w *Workspace) applyWrapping(st *editor.State) {
	st.SoftWrap, st.WrapColumn = w.wrapping(false)
}
