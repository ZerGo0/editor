package action

import (
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

func Tooltip(c *ui.Context, anchor ui.Element, text string) ui.Element {
	if text == "" {
		return anchor
	}
	anchor.Description(text)
	below := anchor.Bounds().Y < token.TitleBarHeight.Px()+token.TooltipHeight.Px()+token.OffsetTooltip.Px()
	p := token.Of(c)
	ui.TooltipBase(c, anchor, func(tip ui.Element) {
		tip.MaxWidth(token.TooltipMaxWidth.Px()).Padding(token.PadTooltipY.Px(), token.PadTooltipX.Px()).Radius(token.RadiusControl.Px()).
			Background(p.Layer.Widget).TextColor(p.Text.Primary)
		token.ElevationPopover.Apply(tip, p)
		tip.Children(func() { token.TextLabel.Apply(ui.Text(c, text)).Wrap() })
		if below {
			tip.AttachTo(anchor, ui.AnchorBottom, ui.AnchorTop).Margin(token.OffsetTooltip.Px(), 0, 0, 0)
			return
		}
		tip.AttachTo(anchor, ui.AnchorTop, ui.AnchorBottom).Margin(0, 0, token.OffsetTooltip.Px(), 0)
	})
	return anchor
}
