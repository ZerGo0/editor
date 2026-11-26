package action

import (
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

type IconOptions struct {
	Keys      string
	Tooltip   string
	Active    bool
	Disabled  bool
	KeepFocus bool
	Tone      token.Tone
	ID        string
}

func IconButton(c *ui.Context, icon *ui.SVG, label string, o IconOptions) ui.Element {
	p := token.Of(c)
	b := ui.ButtonBase(c.Key(keyOf(o.ID, label))).Size(token.IconButtonSize.Px(), token.IconButtonSize.Px()).Radius(token.RadiusControl.Px()).
		Center().Shrink(0).Label(label).Disabled(o.Disabled)
	Tooltip(c, b, tooltipText(label, o.Tooltip, o.Keys))
	keepFocus(b, o)
	bg := p.Fill(b.Hovered(), b.Pressed())
	if o.Active && bg == ui.Transparent {
		bg = p.Control.Toggled
	}
	color := p.Icon
	if o.Tone != token.ToneNeutral {
		color = p.Tone(o.Tone)
	}
	b.Background(bg).Children(func() {
		ui.Icon(c, icon).Size(token.IconSize.Px(), token.IconSize.Px()).TextColor(color)
	})
	return b
}

func MenuIconButton(c *ui.Context, icon *ui.SVG, label string, o IconOptions, menu func(m *ui.Menu)) ui.Element {
	return IconButton(c, icon, label, o).PopoverMenuButton(menu, ui.PopoverMenuButtonOptions{At: ui.AnchorBottomEnd, Self: ui.AnchorTopEnd})
}

func ToggleIconButton(c *ui.Context, icon *ui.SVG, label string, on *bool, o IconOptions) ui.Element {
	p := token.Of(c)
	b := ui.ToggleBase(c.Key(keyOf(o.ID, label)), on).Size(token.ToggleSize.Px(), token.ToggleSize.Px()).Radius(token.RadiusInline.Px()).
		Center().Shrink(0).Label(label).Disabled(o.Disabled)
	Tooltip(c, b, tooltipText(label, o.Tooltip, o.Keys))
	keepFocus(b, o)
	color := p.Icon
	switch {
	case *on:
		b.Background(p.Control.ToggleActive).Border(token.BorderWidth.Px(), p.Control.ToggleActiveBorder)
		color = p.Text.Strong
	default:
		b.Background(p.Fill(b.Hovered(), b.Pressed())).Border(token.BorderWidth.Px(), ui.Transparent)
	}
	b.Children(func() {
		ui.Icon(c, icon).Size(token.IconSize.Px(), token.IconSize.Px()).TextColor(color)
	})
	return b
}

func keepFocus(b ui.Element, o IconOptions) {
	if o.KeepFocus {
		b.KeepFocus()
	}
}
