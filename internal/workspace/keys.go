package workspace

import (
	"maps"

	"github.com/ZerGo0/editor/internal/keymap"
	"github.com/ZerGo0/editor/internal/keymap/when"
	"github.com/egoist/mygo/ui"
)

type keyState struct {
	pending     keymap.Chord
	swallowText bool
	byCode      bool
	cache       singlesCache
}

type singlesCache struct {
	keymap  *keymap.Keymap
	sig     uint32
	singles []keymap.Binding
}

func (w *Workspace) shortcuts(c *ui.Context) {
	km := w.env.Keys.Current
	byCode := w.keys.byCode
	w.keys.byCode = false
	shortcut := func(s keymap.Stroke) bool {
		return !s.ByCode() && c.Shortcut(s.Mods, s.Key) && !byCode
	}
	if len(w.keys.pending) > 0 {
		for _, s := range km.Completions(w.keys.pending[0]) {
			if shortcut(s) {
				w.runChord(append(w.keys.pending, s))
				return
			}
		}
		if c.Shortcut(0, ui.KeyEscape) {
			w.keys.pending = nil
		}
		return
	}
	for _, s := range km.Prefixes() {
		if shortcut(s) {
			w.keys.pending = keymap.Chord{s}
			return
		}
	}
	for _, b := range w.liveSingles(c) {
		if _, ok := keymap.EditorCommand(b.Command); ok {
			continue
		}
		if s, ok := km.MenuStroke(b.Command); ok && s == b.Chord[0] && w.env.Commands.InMenu(b.Command) {
			continue
		}
		if shortcut(b.Chord[0]) {
			w.run(b.Command)
			return
		}
	}
}

func (w *Workspace) liveSingles(c *ui.Context) []keymap.Binding {
	km := w.env.Keys.Current
	ctx, sig := w.whenContext(c)
	cache := &w.keys.cache
	if cache.keymap == km && cache.sig == sig && cache.singles != nil {
		return cache.singles
	}
	owners := map[keymap.Stroke]int{}
	var order []keymap.Stroke
	for i, b := range km.Bindings() {
		if len(b.Chord) != 1 || !b.When.Eval(ctx) {
			continue
		}
		if _, seen := owners[b.Chord[0]]; !seen {
			order = append(order, b.Chord[0])
		}
		owners[b.Chord[0]] = i
	}
	singles := make([]keymap.Binding, 0, len(order))
	for _, s := range order {
		singles = append(singles, km.Bindings()[owners[s]])
	}
	*cache = singlesCache{keymap: km, sig: sig, singles: singles}
	return singles
}

func (w *Workspace) whenContext(c *ui.Context) (when.Context, uint32) {
	editorFocus := w.focusedTab != nil && w.focusedTab.Editor != nil
	textFocus := c != nil && c.FocusTakesText()
	t := w.active.ActiveTab()
	flags := []struct {
		key string
		on  bool
	}{
		{"editorTextFocus", editorFocus},
		{"editorFocus", editorFocus},
		{"textInputFocus", textFocus},
		{"inputFocus", textFocus && !editorFocus},
		{"sideBarFocus", w.focus.side},
		{"panelFocus", w.focus.panel},
		{"searchViewletFocus", w.focus.side && w.tools.side == SearchView},
		{"explorerViewletFocus", w.focus.side && w.tools.side == ExplorerView},
		{"filesExplorerFocus", w.focus.side && w.tools.side == ExplorerView},
		{"listFocus", w.focus.side && !textFocus},
		{"editorIsOpen", t != nil},
		{"canNavigateBack", w.canNavigate(true)},
		{"canNavigateForward", w.canNavigate(false)},
		{"multipleEditorGroups", len(w.layout.Groups()) > 1},
		{"activeEditorIsPinned", t != nil && t.Pinned},
		{"inQuickOpen", w.palette.IsOpen()},
		{"notificationCenterVisible", w.noteCenter},
	}
	ctx := maps.Clone(keymap.DefaultContext())
	var sig uint32
	for i, f := range flags {
		ctx[f.key] = f.on
		if f.on {
			sig |= 1 << i
		}
	}
	return ctx, sig
}

func (w *Workspace) runChord(chord keymap.Chord) {
	w.keys.pending = nil
	ctx, _ := w.whenContext(nil)
	b, ok := w.env.Keys.Current.Resolve(chord, ctx)
	if !ok {
		b, ok = w.env.Keys.Current.Lookup(chord)
	}
	if !ok {
		w.setNotice("The key combination " + chord.Label() + " is not a command.")
		return
	}
	w.run(b.Command)
}

func (w *Workspace) editorInput(ev ui.InputEvent) bool {
	switch {
	case ev.Kind == ui.InputText && w.keys.swallowText:
		w.keys.swallowText = false
		return true
	case ev.Kind != ui.InputKeyDown || len(w.keys.pending) == 0 || ev.Key == ui.KeyUnknown:
		return false
	}
	w.keys.swallowText = true
	w.runChord(append(w.keys.pending, keymap.StrokeFor(ev)))
	return true
}

func (w *Workspace) run(id string) {
	w.update(func() { w.env.Commands.Run(id, w.win) })
}

func (w *Workspace) chordHint() string {
	if len(w.keys.pending) == 0 {
		return ""
	}
	return "(" + w.keys.pending.Label() + ") was pressed. Waiting for the second key of the chord..."
}
