package action

import (
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

func ExpandButton(c *ui.Context, label string, expanded bool, o IconOptions) ui.Element {
	p := token.Of(c)
	icon := icons.ChevronRight
	if expanded {
		icon = icons.ChevronDown
	}
	b := ui.ButtonBase(c.Key(keyOf(o.ID, label))).Width(token.HitTargetMin.Px()).AlignSelf(ui.Stretch).Radius(token.RadiusControl.Px()).
		Center().Shrink(0).Label(label).Tooltip(tooltipText(label, o.Tooltip, o.Keys)).Disabled(o.Disabled).Expanded(expanded)
	keepFocus(b, o)
	b.Background(p.Fill(b.Hovered(), b.Pressed())).Children(func() { glyph(c, icon, p.Icon) })
	return b
}
