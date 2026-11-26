package action

import (
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

type ChipOptions struct {
	Active  bool
	Count   string
	Tooltip string
	Keys    string
	ID      string
}

func Chip(c *ui.Context, label string, o ChipOptions) ui.Element {
	p := token.Of(c)
	b := ui.ButtonBase(c.Key(keyOf(o.ID, label))).Height(token.TagHeight.Px()).Padding(0, token.PadBadgeX.Px()).Gap(token.GapInline.Px()).
		AlignItems(ui.Center).Shrink(0).Radius(token.RadiusPill.Px()).Role(ui.RoleTab).AccessSelected(o.Active).Label(label).
		FocusRingOffset(token.FocusRingOffset.Px())
	if tip := tooltipText(label, o.Tooltip, o.Keys); tip != label {
		b.Tooltip(tip)
	}
	color := p.Text.Secondary
	switch {
	case o.Active:
		b.Background(p.Control.ToggleActive).Border(token.BorderWidth.Px(), p.Control.ToggleActiveBorder)
		color = p.Text.Strong
	default:
		b.Background(p.Fill(b.Hovered(), b.Pressed())).Border(token.BorderWidth.Px(), ui.Transparent)
		if b.Hovered() || b.Pressed() {
			color = p.Text.Primary
		}
	}
	b.Children(func() {
		token.TextLabel.WholeText(c, label).TextColor(color).SingleLine().AccessHidden(true)
		if o.Count != "" {
			token.TextCaption.WholeText(c, o.Count).TextColor(p.Text.Secondary).SingleLine().AccessHidden(true)
		}
	})
	return b
}
