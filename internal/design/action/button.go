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
	bg, fg, border := variantColors(p, o.Variant, b.Hovered(), b.Pressed())
	b.Background(bg).Border(token.BorderWidth.Px(), border)
	if tip := tooltipText(label, o.Tooltip, o.Keys); tip != label {
		b.Tooltip(tip)
	}
	b.Children(func() {
		if o.Icon != nil {
			ui.Icon(c, o.Icon).Size(token.IconSize.Px(), token.IconSize.Px()).TextColor(fg).Shrink(0)
		}
		ui.Text(c, label).FontSize(token.TextBody.Px()).TextColor(fg).SingleLine()
	})
	return b
}

func variantColors(p *token.Palette, v Variant, hovered, pressed bool) (bg, fg, border ui.Color) {
	switch v {
	case Primary:
		bg = p.Accent.Base
		if hovered || pressed {
			bg = p.Accent.Hover
		}
		return bg, p.Accent.Text, p.Control.ButtonBorder
	case Ghost:
		return p.Fill(hovered, pressed), p.Text.Primary, ui.Transparent
	}
	bg = p.Control.Secondary
	if hovered || pressed {
		bg = p.Control.SecondaryHover
	}
	return bg, p.Control.SecondaryText, p.Control.ButtonBorder
}

func tooltipText(label, tooltip, keys string) string {
	if tooltip == "" {
		tooltip = label
	}
	return Hint(tooltip, keys)
}
