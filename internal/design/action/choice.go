package action

import (
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

var (
	switchInset  = (token.SwitchHeight.Px() - token.SwitchKnob.Px()) / 2
	switchTravel = token.SwitchWidth.Px() - token.SwitchKnob.Px() - 2*switchInset
)

type ChoiceOptions struct {
	Description string
	Disabled    bool
	ID          string
}

func Checkbox(c *ui.Context, label string, checked *bool, o ChoiceOptions) ui.Element {
	p := token.Of(c)
	row := ui.CheckboxBase(c.Key(keyOf(o.ID, label)), checked).Gap(token.GapRowIcon.Px()).AlignItems(ui.Center).
		Label(label).Disabled(o.Disabled).Radius(token.RadiusInline.Px())
	if o.Description != "" {
		row.Description(o.Description)
	}
	hovered := row.Hovered()
	row.Children(func() {
		box := ui.Box(c).Size(token.CheckboxSize.Px(), token.CheckboxSize.Px()).Radius(token.RadiusInline.Px()).Center().Shrink(0)
		switch {
		case *checked:
			box.Background(p.Accent.Base).Border(token.BorderWidth.Px(), p.Accent.Base).Children(func() {
				ui.Icon(c, icons.Check).Size(token.IconSizeGlyph.Px(), token.IconSizeGlyph.Px()).TextColor(p.Accent.Text)
			})
		case hovered:
			box.Background(p.Control.Checkbox).Border(token.BorderWidth.Px(), p.Accent.Focus)
		default:
			box.Background(p.Control.Checkbox).Border(token.BorderWidth.Px(), p.Border.Input)
		}
		choiceLabel(c, p, label)
	})
	return row
}

func Switch(c *ui.Context, label string, on *bool, o ChoiceOptions) ui.Element {
	p := token.Of(c)
	row := ui.SwitchBase(c.Key(keyOf(o.ID, label)), on).Gap(token.GapControl.Px()).AlignItems(ui.Center).
		Label(label).Disabled(o.Disabled).Radius(token.RadiusInline.Px())
	if o.Description != "" {
		row.Description(o.Description)
	}
	row.Children(func() {
		track := ui.Box(c).Size(token.SwitchWidth.Px(), token.SwitchHeight.Px()).Radius(token.RadiusPill.Px()).Shrink(0)
		target := float32(0)
		if *on {
			target = switchTravel
			track.Background(p.Accent.Base)
		} else {
			track.Background(p.Control.Secondary).Border(token.BorderWidth.Px(), p.Border.Input)
		}
		x := track.AnimateTo("knob", target, token.Quick)
		track.Children(func() {
			ui.Box(c).Absolute().Top(switchInset).Left(switchInset+x).Size(token.SwitchKnob.Px(), token.SwitchKnob.Px()).
				Radius(token.RadiusPill.Px()).Background(p.Accent.Text)
		})
		choiceLabel(c, p, label)
	})
	return row
}

func choiceLabel(c *ui.Context, p *token.Palette, label string) {
	if label == "" {
		return
	}
	ui.Text(c, label).FontSize(token.TextBody.Px()).TextColor(p.Text.Primary).SingleLine()
}
