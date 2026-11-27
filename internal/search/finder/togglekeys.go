package finder

import (
	"github.com/ZerGo0/editor/internal/design/picker"
	"github.com/ZerGo0/editor/internal/findwidget"
)

func (d *Dialog) toggleShortcuts() []picker.Shortcut {
	if d.tab != TabText {
		return nil
	}
	toggles := findwidget.Toggles(&d.st.MatchCase, &d.st.WholeWord, &d.st.Regex)
	if d.replaceVisible() {
		toggles = append(toggles, findwidget.PreserveCase(&d.st.PreserveCase))
	}
	out := make([]picker.Shortcut, 0, len(toggles))
	for _, t := range toggles {
		on := t.On
		out = append(out, picker.Shortcut{Mods: findwidget.ToggleMods, Key: findwidget.ToggleKey(t.Label), Run: func() { *on = !*on }})
	}
	return out
}
