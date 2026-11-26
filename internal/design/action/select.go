package action

import (
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

type Option[T comparable] struct {
	Value T
	Label string
}

type SelectOptions struct {
	Disabled bool
	ID       string
	Width    token.Size
}

func Select[T comparable](c *ui.Context, label string, selected *T, options []Option[T], o SelectOptions) ui.Element {
	p := token.Of(c)
	sel := ui.SelectBase(c.Key(keyOf(o.ID, label)), selected)
	current := ""
	for _, opt := range options {
		if opt.Value == *selected {
			current = opt.Label
		}
	}
	border := p.Border.Input
	if sel.Open() {
		border = p.Accent.Focus
	}
	trigger := sel.Trigger.Height(token.ControlHeight.Px()).Padding(0, token.PadControlEnd.Px(), 0, token.PadControlX.Px()).Gap(token.GapInline.Px()).
		AlignItems(ui.Center).Radius(token.RadiusControl.Px()).Background(p.Layer.Input).Border(token.BorderWidth.Px(), border).
		Label(label).Disabled(o.Disabled).Shrink(0)
	if o.Width.Px() > 0 {
		trigger.Width(o.Width.Px())
	}
	trigger.Children(func() {
		token.TextBody.Apply(ui.Text(c, current)).TextColor(p.Text.Primary).SingleLine().Grow(1).MinWidth(0)
		ui.Icon(c, icons.ChevronDown).Size(token.IconSize.Px(), token.IconSize.Px()).TextColor(p.Icon).Shrink(0)
	})
	sel.Popup(func(panel ui.Element) {
		panel.Margin(token.OffsetPopover.Px(), 0, 0, 0).Padding(token.PadPopover.Px()).Radius(token.RadiusPopover.Px()).Background(p.Layer.Menu).
			Border(token.BorderWidth.Px(), p.Menu.Border).ZIndex(token.LayerPopover.Z())
		token.ElevationPopover.Apply(panel, p)
		for _, opt := range options {
			item := sel.Item(opt.Value).Height(token.RowHeight.Px()).Padding(0, token.PadControlX.Px()).AlignItems(ui.Center).Radius(token.RadiusInline.Px())
			color := p.Text.Primary
			if item.Highlighted() {
				item.Background(p.Menu.Selection)
				color = p.Menu.SelectionText
			}
			item.Children(func() {
				token.TextBody.Apply(ui.Text(c, opt.Label)).TextColor(color).SingleLine()
			})
		}
	})
	return trigger
}
