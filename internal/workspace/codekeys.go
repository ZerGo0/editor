package workspace

import (
	"slices"

	"github.com/ZerGo0/editor/internal/keymap"
	"github.com/egoist/mygo/ui"
)

const noTextMods = ui.Ctrl | ui.Super

func (w *Workspace) observeCodeKey(ev ui.InputEvent) {
	if ev.Kind != ui.InputKeyDown {
		return
	}
	w.keys.swallowText = false
	s := keymap.StrokeFor(ev)
	if !s.ByCode() {
		return
	}
	w.keys.byCode = true
	active := w.activeKeys(nil)
	switch {
	case len(w.keys.pending) > 0:
		w.runChord(append(w.keys.pending, s))
	case slices.Contains(active.Prefixes(), s):
		w.keys.pending = keymap.Chord{s}
	default:
		b, ok := active.Lookup(keymap.Chord{s})
		if !ok {
			return
		}
		w.run(b.Command)
	}
	w.keys.swallowText = ev.Key != ui.KeyUnknown && ev.Mods&noTextMods == 0
}
