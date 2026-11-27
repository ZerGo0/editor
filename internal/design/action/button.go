package action

import (
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

type Variant uint8

const (
	Secondary Variant = iota
	Primary
	Ghost
)

type ButtonOptions struct {
	Variant  Variant
	Icon     *ui.SVG
	Keys     string
	Tooltip  string
	Disabled bool
	Small    bool
	ID       string
}

func Button(c *ui.Context, label string, o ButtonOptions) ui.Element {
	p := token.Of(c)
	height, padX := token.ControlHeight.Px(), token.PadButtonX.Px()
	if o.Small {
		height, padX = token.ControlHeightSmall.Px(), token.PadControlX.Px()
	}
	b := ui.ButtonBase(c.Key(keyOf(o.ID, label))).Height(height).Padding(0, padX).Gap(token.GapInline.Px()).
		Radius(token.RadiusControl.Px()).Center().Shrink(0).Label(label).Disabled(o.Disabled).FocusRingOffset(token.FocusRingOffsetOuter.Px())
	colors := variantColors(p, o.Variant, b.Hovered(), b.Pressed())
	b.Background(colors.bg).Border(token.BorderWidth.Px(), colors.border)
	if tip := tooltipText(label, o.Tooltip, o.Keys); tip != label {
		Tooltip(c, b, tip)
	}
	b.Children(func() { buttonContent(c, o.Icon, label, colors.fg) })
	return b
}

func buttonContent(c *ui.Context, icon *ui.SVG, label string, fg ui.Color) {
	if icon != nil {
		glyph(c, icon, fg).Shrink(0)
	}
	token.TextBody.WholeText(c, label).TextColor(fg).SingleLine()
}

type buttonColors struct{ bg, fg, border ui.Color }

func variantColors(p *token.Palette, v Variant, hovered, pressed bool) buttonColors {
	switch v {
	case Primary:
		bg := p.Accent.Base
		if hovered || pressed {
			bg = p.Accent.Hover
		}
		return buttonColors{bg, p.Accent.Text, p.Control.ButtonBorder}
	case Ghost:
		return buttonColors{p.Fill(hovered, pressed), p.Text.Primary, ui.Transparent}
	}
	bg := p.Control.Secondary
	if hovered || pressed {
		bg = p.Control.SecondaryHover
	}
	return buttonColors{bg, p.Control.SecondaryText, p.Control.ButtonBorder}
}

func tooltipText(label, tooltip, keys string) string {
	if tooltip == "" {
		tooltip = label
	}
	return Hint(tooltip, keys)
}
