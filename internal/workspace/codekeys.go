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
	manual    bool
	taken     bool
	typesText bool
}

const (
	usKeys        = "1234567890-=qwrtyopasdfghjklzxcvbm,./;'[]\\"
	usOption      = "\u00a1\u2122\u00a3\u00a2\u221e\u00a7\u00b6\u2022\u00aa\u00ba\u2013\u2260\u0153\u2211\u00ae\u2020\u00a5\u00f8\u03c0\u00e5\u00df\u2202\u0192\u00a9\u02d9\u2206\u02da\u00ac\u03a9\u2248\u00e7\u221a\u222b\u00b5\u2264\u2265\u00f7\u2026\u00e6\u201c\u2018\u00ab"
	usShiftOption = "\u2044\u20ac\u2039\u203a\ufb01\ufb02\u2021\u00b0\u00b7\u201a\u2014\u00b1\u0152\u201e\u2030\u02c7\u00c1\u00d8\u220f\u00c5\u00cd\u00ce\u00cf\u02dd\u00d3\u00d4\uf8ff\u00d2\u00b8\u02db\u00c7\u25ca\u0131\u00c2\u00af\u02d8\u00bf\u00da\u00c6\u201d\u2019\u00bb"
)

var usOptionText = buildUSOptionText()

func buildUSOptionText() map[keymap.Stroke]string {
	keys, plain, shifted := []rune(usKeys), []rune(usOption), []rune(usShiftOption)
	out := make(map[keymap.Stroke]string, len(keys)*2)
	for i, r := range keys {
		for prefix, text := range map[string]rune{"alt+": plain[i], "shift+alt+": shifted[i]} {
			if chord, err := keymap.ParseChord(prefix + string(r)); err == nil {
				out[chord[0]] = string(text)
			}
		}
	}
	return out
}

func layoutTypes(s keymap.Stroke, text string) bool {
	if s.Mods&ui.Alt == 0 || s.Mods&noTextMods != 0 || text == "" {
		return false
	}
	return usOptionText[s] != text
}

var characterKeys = map[ui.Key]bool{
	ui.KeySpace: true, ui.KeyMinus: true, ui.KeyEqual: true, ui.KeyComma: true, ui.KeyPeriod: true, ui.KeySlash: true,
	ui.KeySemicolon: true, ui.KeyQuote: true, ui.KeyBracketLeft: true, ui.KeyBracketRight: true, ui.KeyBackslash: true, ui.KeyBackquote: true,
}

func (k codeKey) active() bool { return k.manual }

func typesCharacter(s keymap.Stroke) bool {
	if s.Mods&^(ui.Shift|ui.Alt) != 0 {
		return false
	}
	return characterKeys[s.Key] || s.Key >= ui.Key0 && s.Key <= ui.Key9 || s.Key >= ui.KeyA && s.Key <= ui.KeyZ
}

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
		w.keys.swallowText = false
	case ui.InputText:
		if code := &w.keys.code; code.active() && !code.taken && layoutTypes(code.stroke, ev.Text) {
			code.taken, w.keys.swallowText = true, false
			return
		}
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
	typesText := ev.Mods&noTextMods == 0 && w.takesKey(s)
	if !s.ByCode() && !(typesText && typesCharacter(s)) {
		return
	}
	w.keys.code = codeKey{stroke: s, manual: true, typesText: typesText}
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
	fired := !s.ByCode() && c.Shortcut(s.Mods, s.Key)
	if !code.active() || code.stroke != s {
		return fired && !code.active()
	}
	if code.taken || !w.codeReaches(call) {
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
