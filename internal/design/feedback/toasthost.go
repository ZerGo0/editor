package feedback

import (
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const closeToastLabel = "Close Notification"

type toastBusy struct{}

func ToneOf(kind string) token.Tone {
	for _, t := range []token.Tone{token.ToneInfo, token.ToneSuccess, token.ToneWarning, token.ToneError} {
		if t.String() == kind {
			return t
		}
	}
	return token.ToneNeutral
}

func ToastHost(c *ui.Context) ui.Element {
	p := token.Of(c)
	return ui.ToastViewportBase(c, func(viewport ui.Element, toasts []ui.Toast) {
		viewport.Padding(token.PadPanel.Px(), token.PadPanel.Px(), token.StatusBarHeight.Px()+token.PadPanel.Px(), token.PadPanel.Px()).
			AlignItems(ui.End).Justify(ui.End).Gap(token.GapStack.Px()).ZIndex(token.LayerToast.Z())
		for _, t := range toasts {
			toast(c, p, t)
		}
	})
}

func toast(c *ui.Context, p *token.Palette, t ui.Toast) {
	parts := ui.ToastBase(c, t)
	tone := ToneOf(t.Type)
	root := parts.Root.Width(token.ToastWidth.Px()).Radius(token.RadiusCard.Px()).Background(p.Layer.Widget).
		Border(token.BorderWidth.Px(), p.Border.Widget).Clip()
	token.ElevationDialog.Apply(root, p)
	root.Children(func() {
		ui.Column(c).FillWidth().Children(func() {
			ui.Row(c).FillWidth().Padding(token.PadNoticeY.Px(), token.PadInputY.Px(), token.PadNoticeY.Px(), token.PadNoticeX.Px()).
				Gap(token.GapControl.Px()).AlignItems(ui.Start).Children(func() {
				ui.Icon(c, ToneIcon(tone)).Size(token.IconSize.Px(), token.IconSize.Px()).TextColor(p.Tone(tone)).Shrink(0).
					Margin(token.PadInputEnd.Px(), 0, 0, 0).AccessHidden(true)
				ui.Column(c).Grow(1).MinWidth(0).Gap(token.GapTight.Px()).Children(func() {
					token.TextBody.Apply(ui.Text(c, t.Title)).TextColor(p.Text.Primary).Wrap()
					if t.Description != "" {
						token.TextLabel.Apply(ui.Text(c, t.Description)).TextColor(p.Text.Secondary).Wrap().Selectable()
					}
					if t.Action != "" {
						ui.Row(c).Justify(ui.End).FillWidth().Margin(token.GapInline.Px(), 0, 0, 0).Children(func() {
							toastAction(c, p, parts, t.Action)
						})
					}
				})
				closeToast(c, p, parts)
			})
			if _, busy := t.Data.(toastBusy); busy {
				ProgressBar(c, t.Title)
			}
		})
	})
}

func toastAction(c *ui.Context, p *token.Palette, parts ui.ToastParts, label string) {
	b := parts.ActionButton().Height(token.ControlHeightSmall.Px()).Padding(0, token.PadControlX.Px()).Radius(token.RadiusControl.Px()).
		Center().Label(label).FocusRingOffset(token.FocusRingOffsetOuter.Px())
	bg := p.Control.Secondary
	if b.Hovered() || b.Pressed() {
		bg = p.Control.SecondaryHover
	}
	b.Background(bg).Border(token.BorderWidth.Px(), p.Control.ButtonBorder).Children(func() {
		token.TextBody.WholeText(c, label).TextColor(p.Control.SecondaryText).SingleLine()
	})
}

func closeToast(c *ui.Context, p *token.Palette, parts ui.ToastParts) {
	b := parts.CloseButton().Size(token.IconButtonSize.Px(), token.IconButtonSize.Px()).Radius(token.RadiusControl.Px()).Center().Shrink(0).
		Label(closeToastLabel)
	action.Tooltip(c, b, closeToastLabel)
	b.Background(p.Fill(b.Hovered(), b.Pressed())).Children(func() {
		ui.Icon(c, icons.Close).Size(token.IconSize.Px(), token.IconSize.Px()).TextColor(p.Icon)
	})
}
