package pane

import (
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

func Toolbar(c *ui.Context, label string, children func(c *ui.Context)) ui.Element {
	return ui.Row(c).Height(token.ToolbarHeight.Px()).Gap(token.GapTight.Px()).AlignItems(ui.Center).Shrink(0).
		Role(ui.RoleToolbar).Label(label).FocusGroup(ui.Horizontal).Children(func() { children(c) })
}

func Separator(c *ui.Context, vertical bool) ui.Element {
	p := token.Of(c)
	line := ui.Box(c).Background(p.Border.Default).Shrink(0).Role(ui.RoleSeparator)
	if vertical {
		return line.Width(token.BorderWidth.Px()).Height(token.IconSize.Px()).Margin(0, token.GapInline.Px())
	}
	return line.Height(token.BorderWidth.Px()).FillWidth()
}
