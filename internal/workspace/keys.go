package workspace

import (
	"maps"
	"strings"

	"github.com/ZerGo0/editor/internal/keymap"
	"github.com/ZerGo0/editor/internal/keymap/when"
	"github.com/ZerGo0/editor/internal/layout"
	"github.com/egoist/mygo/ui"
)

const plainTextLanguage = "plaintext"

type keyState struct {
	pending     keymap.Chord
	code        codeKey
	swallowText bool
	textFocus   bool
	rootFocus   bool
	cache       activeCache
}

type activeCache struct {
	keymap *keymap.Keymap
	ctx    when.Context
	active *keymap.Active
}

func (w *Workspace) shortcuts(c *ui.Context, call keyScope) {
	km := w.env.Keys.Current
	active := w.activeKeys(c)
	shortcut := func(s keymap.Stroke) bool { return w.pressed(c, call, s) }
	if len(w.keys.pending) > 0 {
		for _, s := range active.Completions(w.keys.pending[0]) {
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
	for _, s := range active.Prefixes() {
		if shortcut(s) {
			w.keys.pending = keymap.Chord{s}
			return
		}
	}
	for _, b := range active.Singles() {
		if _, ok := keymap.EditorCommand(b.Command); ok && !b.Chord[0].ByCode() && w.editorRunsItsKeys() {
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

func (w *Workspace) editorRunsItsKeys() bool {
	t := w.focusedTab
	return t == nil || t.Editor == nil || t.Editor.Focused()
}

func (w *Workspace) activeKeys(c *ui.Context) *keymap.Active {
	km := w.env.Keys.Current
	ctx := w.whenContext(c)
	cache := &w.keys.cache
	if cache.keymap != km || cache.active == nil || !maps.Equal(cache.ctx, ctx) {
		*cache = activeCache{keymap: km, ctx: ctx, active: km.In(ctx)}
	}
	return cache.active
}

func (w *Workspace) whenContext(c *ui.Context) when.Context {
	editorFocus := w.focusedTab != nil && w.focusedTab.Editor != nil
	editorText := editorFocus && w.focusedTab.Editor.Focused()
	if c != nil {
		w.keys.textFocus = c.FocusTakesText()
	}
	textFocus := w.keys.textFocus
	t := w.active.ActiveTab()
	ctx := maps.Clone(keymap.DefaultContext())
	for key, on := range map[string]bool{
		"editorTextFocus":           editorText,
		"editorFocus":               editorFocus,
		"findInputFocussed":         editorFocus && !editorText && textFocus,
		"textInputFocus":            textFocus,
		"inputFocus":                textFocus && !editorFocus,
		"sideBarFocus":              w.focus.side,
		"panelFocus":                w.focus.panel,
		"explorerViewletFocus":      w.focus.side && w.tools.side == ExplorerView,
		"filesExplorerFocus":        w.focus.side && w.tools.side == ExplorerView,
		"listFocus":                 w.focus.side && !textFocus,
		"editorIsOpen":              t != nil,
		"canNavigateBack":           w.canNavigate(true),
		"canNavigateForward":        w.canNavigate(false),
		"multipleEditorGroups":      len(w.layout.Groups()) > 1,
		"activeEditorIsPinned":      t != nil && t.Pinned,
		"inQuickOpen":               w.palette.IsOpen() || w.searchOpen(),
		"notificationCenterVisible": w.noteCenter,
		"sideBarVisible":            !w.sidebarHidden,
	} {
		ctx[key] = on
	}
	ctx["editorLangId"] = w.languageID(t)
	return ctx
}

func (w *Workspace) languageID(t *layout.Tab) string {
	if t == nil || t.Editor == nil {
		return ""
	}
	lang := t.Editor.Buffer.Language()
	if lang == nil {
		return plainTextLanguage
	}
	return strings.ReplaceAll(strings.ToLower(lang.Name), " ", "")
}

func (w *Workspace) runChord(chord keymap.Chord) {
	w.keys.pending = nil
	b, ok := w.env.Keys.Current.Resolve(chord, w.whenContext(nil))
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
	w.keys.code.taken = true
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
