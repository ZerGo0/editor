package workspace

import (
	"slices"

	"github.com/ZerGo0/editor/internal/design/field"
	"github.com/ZerGo0/editor/internal/design/focus"
	"github.com/ZerGo0/editor/internal/keymap"
	"github.com/egoist/mygo/ui"
)

const noTextMods = ui.Ctrl | ui.Super

type keyScope int

const (
	scopeRoot keyScope = iota
	scopeMain
	scopeModal
)

type codeKey struct {
	stroke    keymap.Stroke
	scope     keyScope
	taken     bool
	typesText bool
}

func (k codeKey) active() bool { return k.stroke.ByCode() }

func (w *Workspace) modalKeys() focus.Keys {
	return focus.Keys{
		Observe: func(ev ui.InputEvent) { w.observeScope(scopeModal, ev) },
		Handle:  func(c *ui.Context) { w.shortcuts(c, scopeModal) },
	}
}

func (w *Workspace) observeMainKey(ev ui.InputEvent) { w.observeScope(scopeMain, ev) }

func (w *Workspace) observeScope(scope keyScope, ev ui.InputEvent) {
	if ev.Kind == ui.InputKeyDown && w.keys.code.active() {
		w.keys.code.scope = scope
		w.keys.swallowText = w.keys.code.typesText
	}
}

func (w *Workspace) observeKey(ev ui.InputEvent) {
	switch ev.Kind {
	case ui.InputKeyUp:
		w.keys.swallowText, w.keys.dropEditorText = false, false
	case ui.InputText:
		if w.keys.swallowText {
			field.Swallow(ev.Text)
		}
	case ui.InputKeyDown:
		w.recordKey(ev)
	}
}

func (w *Workspace) recordKey(ev ui.InputEvent) {
	w.keys.swallowText = false
	w.keys.code = codeKey{}
	s := keymap.StrokeFor(ev)
	if !s.ByCode() {
		w.keys.dropEditorText = ev.Mods&noTextMods == 0 && w.takesKey(s)
		return
	}
	w.keys.code = codeKey{stroke: s, typesText: ev.Mods&noTextMods == 0 && w.takesKey(s)}
	w.keys.swallowText = w.keys.code.typesText && w.keys.rootFocus
}

func (w *Workspace) takesKey(s keymap.Stroke) bool {
	if len(w.keys.pending) > 0 {
		return true
	}
	active := w.activeKeys(nil)
	if slices.Contains(active.Prefixes(), s) {
		return true
	}
	_, ok := active.Lookup(keymap.Chord{s})
	return ok
}

func (w *Workspace) pressed(c *ui.Context, call keyScope, s keymap.Stroke) bool {
	code := &w.keys.code
	if !s.ByCode() {
		return c.Shortcut(s.Mods, s.Key) && !code.active()
	}
	if code.taken || code.stroke != s || !w.codeReaches(call) {
		return false
	}
	code.taken = true
	return true
}

func (w *Workspace) codeReaches(call keyScope) bool {
	scope := w.keys.code.scope
	if call == scopeModal {
		return scope == scopeModal
	}
	return scope == scopeMain || scope == scopeRoot && w.keys.rootFocus
}

func (w *Workspace) endKeyFrame() {
	w.keys.code = codeKey{}
	field.Swallow("")
}
