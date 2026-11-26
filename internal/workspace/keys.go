package workspace

import (
	"time"

	"github.com/ZerGo0/editor/internal/keymap"
	"github.com/egoist/mygo/ui"
)

const noticeDuration = 3 * time.Second

type keyState struct {
	pending     keymap.Chord
	swallowText bool
	byCode      bool
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
			w.notice = ""
			return
		}
	}
	for _, b := range km.Singles() {
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

func (w *Workspace) runChord(chord keymap.Chord) {
	w.keys.pending = nil
	b, ok := w.env.Keys.Current.Lookup(chord)
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
	w.runChord(append(w.keys.pending, keymap.Stroke{Mods: ev.Mods, Key: ev.Key}))
	return true
}

func (w *Workspace) run(id string) {
	w.update(func() { w.env.Commands.Run(id, w.win) })
}

func (w *Workspace) setNotice(text string) {
	w.notice = text
	w.noticeSeq++
	seq := w.noticeSeq
	time.AfterFunc(noticeDuration, func() {
		w.update(func() {
			if w.noticeSeq == seq {
				w.notice = ""
			}
		})
	})
}

func (w *Workspace) chordHint() string {
	if len(w.keys.pending) == 0 {
		return w.notice
	}
	return "(" + w.keys.pending.Label() + ") was pressed. Waiting for the second key of the chord..."
}
