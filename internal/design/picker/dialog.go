package picker

import (
	"github.com/ZerGo0/editor/internal/design/focus"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

type Shortcut struct {
	Mods ui.Modifiers
	Key  ui.Key
	Run  func()
}

type DialogOptions struct {
	Label      string
	ID         string
	Layout     *Layout
	Shortcuts  []Shortcut
	WindowKeys func(c *ui.Context)
}

func Dialog(c *ui.Context, o DialogOptions, body func()) (closed bool) {
	p := token.Of(c)
	focus.Return(c, "picker-dialog:"+o.ID)
	ui.Overlay(c, func() {
		back := ui.Column(c.Key("picker-dialog:" + o.ID)).Absolute().Left(0).Top(0).Right(0).Bottom(0).AlignItems(ui.Center).Justify(ui.Center).Modal()
		win := back.Bounds()
		back.Children(func() {
			if o.WindowKeys != nil {
				o.WindowKeys(c)
			}
			margin := token.GapControl.Px()
			panel := ui.Column(c).Margin(margin).FillWidth().Padding(token.PadPopover.Px()).Gap(token.PadPopover.Px()).
				Background(p.Layer.QuickInput).Radius(token.RadiusPopover.Px()).Border(token.BorderWidth.Px(), p.Border.Widget).
				ZIndex(token.LayerQuickInput.Z()).Role(ui.RoleDialog).Label(o.Label)
			token.ElevationDialog.Apply(panel, p)
			if o.Layout != nil {
				w, h := o.Layout.frame(win)
				panel.MaxWidth(w).Height(h).Shrink(1).MinHeight(0)
			} else {
				panel.MaxWidth(token.SearchPopupWidth.Px())
				if room := win.H - 2*margin; room > 0 {
					panel.MaxHeight(room)
				}
			}
			panel.Children(func() {
				body()
				if o.Layout != nil {
					o.Layout.sashes(c, win, panel.Bounds())
				}
			})
			for _, s := range o.Shortcuts {
				if back.OverlayShortcut(s.Mods, s.Key) {
					s.Run()
				}
			}
			if panel.PressedOutside() || !c.WindowActive() || back.OverlayShortcut(0, ui.KeyEscape) {
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
