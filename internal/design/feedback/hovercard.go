package feedback

import (
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

func HoverCard(c *ui.Context, anchor ui.Element, body func()) ui.Element {
	p := token.Of(c)
	return ui.TooltipBase(c, anchor, func(tip ui.Element) {
		tip.AttachTo(anchor, ui.AnchorTopRight, ui.AnchorTopLeft).Margin(0, 0, 0, token.OffsetPopover.Px()).AttachedOrigin().
			Width(token.PopoverMaxWidth.Px()).Padding(token.PadPanel.Px()).Radius(token.RadiusPopover.Px()).Background(p.Layer.Widget).
			Border(token.BorderWidth.Px(), p.Border.Widget).ZIndex(token.LayerTooltip.Z()).TextColor(p.Text.Primary)
		token.ElevationPopover.Apply(tip, p)
		tip.Children(func() {
			ui.Column(c).FillWidth().Gap(token.GapStack.Px()).Children(body)
		})
	})
}
