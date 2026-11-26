package searchview

import (
	"github.com/ZerGo0/editor/internal/fileicons"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const (
	bodyFont       = 13
	smallFont      = 12
	tinyFont       = 11
	rowHeight      = 24
	indentStep     = 14
	sidePad        = 8
	iconSize       = 14
	fileIconSize   = 16
	buttonSize     = 22
	toggleSize     = 20
	chevronWidth   = 18
	fieldRadius    = 5
	chipRadius     = 10
	badgeRadius    = 7
	spinnerSize    = 14
	lineNumWidth   = 30
	maxFieldLines  = 8
	beforeContext  = 24
	tintAlpha      = 0.22
	strongTint     = 0.33
	headerAlpha    = 0.9
	highlightAlpha = 0.38
	selectedHigh   = 0.30
	mutedSelected  = 0.78
	stateGap       = 8
	statePadding   = 16
	kindBadgeSize  = 16
)

func iconButton(c *ui.Context, key, label string, svg *ui.SVG, disabled bool) ui.Element {
	return iconButtonOn(c, key, label, svg, disabled, false)
}

func iconButtonOn(c *ui.Context, key, label string, svg *ui.SVG, disabled, onAccent bool) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c.Key(key)).Size(buttonSize, buttonSize).Radius(fieldRadius).Center().Shrink(0).
		TextColor(mutedColor(t, onAccent)).Label(label).Tooltip(label).Disabled(disabled)
	if b.Hovered() && !disabled {
		hover := ifColor(onAccent, t.AccentText.Alpha(tintAlpha), t.SurfaceHover)
		b.Background(hover).TextColor(ifColor(onAccent, t.AccentText, t.Text))
	}
	b.Children(func() { ui.Icon(c, svg).Size(iconSize, iconSize) })
	return b
}

func textButton(c *ui.Context, key, label string, disabled bool) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c.Key(key)).Height(buttonSize).Padding(0, 6).Radius(fieldRadius).Center().Shrink(0).Label(label).Disabled(disabled)
	if b.Hovered() && !disabled {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		text := ui.Text(c, label).FontSize(smallFont).SingleLine()
		if disabled {
			text.TextColor(t.TextMuted)
		}
	})
	return b
}

func flagToggle(c *ui.Context, on *bool, svg *ui.SVG, label, tip string) bool {
	t := c.Theme()
	b := ui.ToggleBase(c.Key(label), on).Size(toggleSize, toggleSize).Radius(4).Center().Shrink(0).Label(label).Tooltip(tip)
	switch {
	case *on:
		b.Background(t.Accent.Alpha(tintAlpha)).BorderWidth(1).BorderColor(t.Accent)
	case b.Hovered():
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		ui.Icon(c, svg).Size(iconSize, iconSize).TextColor(ifColor(*on, t.Text, t.TextMuted))
	})
	return b.Changed()
}

func ifColor(cond bool, a, b ui.Color) ui.Color {
	if cond {
		return a
	}
	return b
}

func mutedText(c *ui.Context, s string, selected bool) ui.Element {
	return ui.Text(c, s).FontSize(smallFont).TextColor(mutedColor(c.Theme(), selected))
}

func mutedColor(t *ui.Theme, selected bool) ui.Color {
	if selected {
		return t.AccentText.Alpha(mutedSelected)
	}
	return t.TextMuted
}

func countBadge(c *ui.Context, n int, selected bool) {
	t := c.Theme()
	bg, fg := t.TextMuted.Alpha(tintAlpha), t.TextMuted
	if selected {
		bg, fg = t.AccentText.Alpha(tintAlpha), t.AccentText
	}
	ui.Text(c, formatCount(n)).FontSize(tinyFont).SingleLine().TextColor(fg).Background(bg).Radius(badgeRadius).Padding(0, 5).Shrink(0)
}

func fileIcon(c *ui.Context, name string) {
	fileicons.For(name).ViewSize(c, fileIconSize).Shrink(0)
}

func folderIcon(c *ui.Context, name string, open bool) {
	fileicons.Folder(name, open).ViewSize(c, fileIconSize).Shrink(0)
}

func chevron(c *ui.Context, open, selected bool) {
	svg := icons.ChevronRight
	if open {
		svg = icons.ChevronDown
	}
	ui.Icon(c, svg).Size(iconSize, iconSize).TextColor(mutedColor(c.Theme(), selected)).Shrink(0)
}

func stateText(c *ui.Context, title, detail string) {
	t := c.Theme()
	ui.Column(c).FillWidth().Center().Gap(stateGap).Padding(statePadding).Children(func() {
		ui.Text(c, title).FontSize(smallFont).TextColor(t.TextMuted).TextAlign(ui.Center)
		if detail != "" {
			ui.Text(c, detail).FontSize(tinyFont).TextColor(t.TextMuted).TextAlign(ui.Center)
		}
	})
}

func formatCount(n int) string {
	if n >= ResultLimit {
		return itoa(ResultLimit) + "+"
	}
	return itoa(n)
}
