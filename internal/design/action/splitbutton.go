package action

import (
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const (
	defaultMenuLabel = "More Actions..."
)

type SplitOptions struct {
	Variant   Variant
	Icon      *ui.SVG
	Keys      string
	Tooltip   string
	Disabled  bool
	ID        string
	MenuLabel string
	Menu      func(m *ui.Menu)
	Wide      bool
}

func SplitButton(c *ui.Context, label string, o SplitOptions) ui.Element {
	p := token.Of(c)
	id := keyOf(o.ID, label)
	menuLabel := o.MenuLabel
	if menuLabel == "" {
		menuLabel = defaultMenuLabel
	}
	var button ui.Element
	group := ui.Row(c.Key(id)).Height(token.ControlHeight.Px()).Radius(token.RadiusControl.Px()).AlignItems(ui.Stretch).Shrink(0).
		Role(ui.RoleGroup).Label(label).Disabled(o.Disabled)
	if o.Wide {
		group.FillWidth()
	}
	base, fg, border := variantColors(p, o.Variant, false, false)
	separator := p.Control.ButtonBorder
	if o.Variant == Primary {
		separator = p.Accent.Text.Alpha(token.OpacitySeparator.Value())
	}
	group.Background(base).Border(token.BorderWidth.Px(), border).Children(func() {
		button = ui.ButtonBase(c.Key("action")).Grow(1).Padding(0, token.PadButtonX.Px()).Gap(token.GapInline.Px()).Center().
			Radius(token.RadiusControl.Px(), 0, 0, token.RadiusControl.Px()).Label(label).FocusRingOffset(-token.BorderWidth.Px())
		if tip := tooltipText(label, o.Tooltip, o.Keys); tip != label {
			button.Tooltip(tip)
		}
		segment(p, o.Variant, button)
		button.Children(func() {
			if o.Icon != nil {
				ui.Icon(c, o.Icon).Size(token.IconSize.Px(), token.IconSize.Px()).TextColor(fg).Shrink(0)
			}
			token.TextBody.Apply(ui.Text(c, label)).TextColor(fg).SingleLine()
		})
		ui.Box(c).Width(token.BorderWidth.Px()).Height(token.IconSize.Px()).AlignSelf(ui.Center).Background(separator).Shrink(0)
		arrow := ui.ButtonBase(c.Key("menu")).Width(token.SplitArrowWidth.Px()).Center().Shrink(0).
			Radius(0, token.RadiusControl.Px(), token.RadiusControl.Px(), 0).Label(menuLabel).Tooltip(menuLabel).FocusRingOffset(-token.BorderWidth.Px())
		segment(p, o.Variant, arrow)
		arrow.Children(func() {
			ui.Icon(c, icons.ChevronDown).Size(token.IconSize.Px(), token.IconSize.Px()).TextColor(fg)
		})
		if o.Menu != nil {
			arrow.PopoverMenuButton(o.Menu, ui.PopoverMenuButtonOptions{At: ui.AnchorBottomEnd, Self: ui.AnchorTopEnd})
		}
	})
	return button
}

func segment(p *token.Palette, v Variant, b ui.Element) {
	hovered, pressed := b.Hovered(), b.Pressed()
	if !hovered && !pressed {
		return
	}
	bg, _, _ := variantColors(p, v, hovered, pressed)
	b.Background(bg)
}
