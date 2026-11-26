package workspace

import (
	"github.com/egoist/mygo/ui"
)

const (
	iconButtonSize   = 24
	iconButtonIcon   = 16
	iconButtonRadius = 5
	sectionFontSize  = 11
	uiFontSize       = 13
	smallFontSize    = 12
)

func iconButton(c *ui.Context, key, label, tooltip string, svg *ui.SVG, active bool, action func()) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c.Key(key)).Size(iconButtonSize, iconButtonSize).Radius(iconButtonRadius).Center().
		Label(label).Tooltip(tooltip).Shrink(0)
	color := t.TextMuted
	if active {
		color = t.Text
	}
	if pressable(b, t) {
		color = t.Text
	}
	b.TextColor(color).Children(func() { ui.Icon(c, svg).Size(iconButtonIcon, iconButtonIcon) })
	b.OnClick(action)
	return b
}

func pressable(b ui.Element, t *ui.Theme) bool {
	switch {
	case b.Pressed():
		b.Background(t.SurfacePressed)
	case b.Hovered():
		b.Background(t.SurfaceHover)
	default:
		return false
	}
	return true
}

func (w *Workspace) withKeys(label, command string) string {
	if keys := w.keyLabel(command); keys != "" {
		return label + " (" + keys + ")"
	}
	return label
}

func (w *Workspace) commandItem(m *ui.Menu, label, command string) {
	item := m.Item(label)
	if chords := w.env.Keys.Current.For(command); len(chords) > 0 && len(chords[0]) == 1 {
		item.Shortcut(chords[0][0].Mods, chords[0][0].Key)
	}
	if item.Chosen() {
		w.run(command)
	}
}

func activeAccent(c *ui.Context) ui.Color {
	if c.WindowActive() {
		return c.Theme().Accent
	}
	return c.Theme().TextMuted
}
