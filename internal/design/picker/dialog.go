package picker

import (
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

type DialogOptions struct {
	Label string
	ID    string
}

func Dialog(c *ui.Context, o DialogOptions, body func()) (closed bool) {
	p := token.Of(c)
	ui.Overlay(c, func() {
		back := ui.Column(c.Key("picker-dialog:" + o.ID)).Absolute().Left(0).Top(0).Right(0).Bottom(0).AlignItems(ui.Center).Modal()
		top := token.TitleBarHeight.Px() + token.OffsetQuickOpen.Px()
		room := back.Bounds().H - top - token.GapControl.Px()
		back.Children(func() {
			panel := ui.Column(c).Margin(top, token.GapControl.Px(), 0, token.GapControl.Px()).
				FillWidth().MaxWidth(token.SearchPopupWidth.Px()).Padding(token.PadPopover.Px()).Gap(token.PadPopover.Px()).
				Background(p.Layer.QuickInput).Radius(token.RadiusPopover.Px()).Border(token.BorderWidth.Px(), p.Border.Widget).
				ZIndex(token.LayerQuickInput.Z()).Role(ui.RoleDialog).Label(o.Label)
			token.ElevationDialog.Apply(panel, p)
			if room > 0 {
				panel.MaxHeight(room)
			}
			panel.Children(body)
			if panel.PressedOutside() || back.OverlayShortcut(0, ui.KeyEscape) {
				closed = true
			}
		})
	})
	return closed
}

func Header(c *ui.Context, left, right func()) ui.Element {
	return ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(token.GapControl.Px()).Children(func() {
		ui.Row(c).Grow(1).MinWidth(0).AlignItems(ui.Center).Gap(token.GapTight.Px()).Children(left)
		if right != nil {
			ui.Row(c).Shrink(0).AlignItems(ui.Center).Gap(token.GapTight.Px()).Padding(0, token.PadInputEnd.Px()+token.BorderWidth.Px(), 0, 0).Children(right)
		}
	})
}
