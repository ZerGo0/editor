package workspace

import (
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/egoist/mygo/ui"
)

func (w *Workspace) iconButton(c *ui.Context, icon *ui.SVG, label, command string, active bool, fn func()) ui.Element {
	b := action.IconButton(c, icon, label, action.IconOptions{Keys: w.keyLabel(command), Active: active})
	if b.Clicked() {
		fn()
	}
	return b
}

func (w *Workspace) withKeys(label, command string) string {
	return action.Hint(label, w.keyLabel(command))
}

func (w *Workspace) commandItem(m *ui.Menu, label, command string) {
	o := action.ItemOptions{ID: command}
	if chords := w.env.Keys.Current.For(command); len(chords) > 0 && len(chords[0]) == 1 {
		o.Mods, o.Key = chords[0][0].Mods, chords[0][0].Key
	} else if len(chords) > 0 {
		o.Detail = chords[0].Label()
	}
	if action.Item(m, label, o).Chosen() {
		w.run(command)
	}
}
