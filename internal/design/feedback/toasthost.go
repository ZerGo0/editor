package feedback

import (
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const closeToastLabel = "Clear Notification"

type toastData struct {
	busy        bool
	secondary   string
	onSecondary func()
}

type ToastHostOptions struct {
	Focus *bool
}

var toneWords = map[token.Tone]string{token.ToneError: "Error", token.ToneWarning: "Warning", token.ToneInfo: "Info", token.ToneSuccess: "Success"}

func ToneOf(kind string) token.Tone {
	for _, t := range []token.Tone{token.ToneInfo, token.ToneSuccess, token.ToneWarning, token.ToneError} {
		if t.String() == kind {
			return t
		}
	}
	return token.ToneNeutral
}

func ToastHost(c *ui.Context, o ToastHostOptions) ui.Element {
	p := token.Of(c)
	return ui.ToastViewportBase(c, func(viewport ui.Element, toasts []ui.Toast) {
		viewport.Padding(token.PadPanel.Px(), token.PadPanel.Px(), token.StatusBarHeight.Px()+token.PadPanel.Px(), token.PadPanel.Px()).
			AlignItems(ui.End).Justify(ui.End).Gap(token.GapStack.Px()).ZIndex(token.LayerToast.Z())
		focus := o.Focus != nil && *o.Focus
		if o.Focus != nil {
			*o.Focus = false
		}
		for i, t := range toasts {
			toast(c, p, t, focus && i == len(toasts)-1)
		}
	})
}

func toast(c *ui.Context, p *token.Palette, t ui.Toast, focus bool) {
	parts := ui.ToastBase(c, t)
	data, _ := t.Data.(toastData)
	var first ui.Element
	hasAction := false
	tone := ToneOf(t.Type)
	firstLine := token.IconButtonSize.Px()
	iconInset := (firstLine - token.IconSize.Px()) / 2
	textInset := (firstLine - token.TextBody.LineHeight()) / 2
	root := parts.Root.Width(token.ToastWidth.Px()).Radius(token.RadiusCard.Px()).Background(p.Layer.Widget).
		Border(token.BorderWidth.Px(), p.Border.Widget).Clip()
	token.ElevationDialog.Apply(root, p)
	if tone == token.ToneError {
		root.Role(ui.RoleAlert)
	}
	root.Children(func() {
		ui.Column(c).FillWidth().Children(func() {
			ui.Row(c).FillWidth().Padding(token.PadNoticeY.Px(), token.PadInputY.Px(), token.PadNoticeY.Px(), token.PadNoticeX.Px()).
				Gap(token.GapControl.Px()).AlignItems(ui.Start).Children(func() {
				icon := ui.Icon(c, ToneIcon(tone)).Size(token.IconSize.Px(), token.IconSize.Px()).TextColor(p.Tone(tone)).Shrink(0).
					Margin(iconInset, 0, 0, 0)
				if word := toneWords[tone]; word != "" {
					icon.Role(ui.RoleImage).Label(word)
				} else {
					icon.AccessHidden(true)
				}
				ui.Column(c).Grow(1).MinWidth(0).Margin(textInset, 0, 0, 0).Gap(token.GapTight.Px()).Children(func() {
					inline := actionFitsTitleRow(c, t, data)
					if inline {
						ui.Row(c).FillWidth().Gap(token.GapControl.Px()).AlignItems(ui.Center).Children(func() {
							token.TextBody.Apply(ui.Text(c, t.Title)).TextColor(p.Text.Primary).Wrap().Grow(1).MinWidth(0)
							first, hasAction = toastAction(c, p, parts, t.Action), true
						})
					} else {
						token.TextBody.Apply(ui.Text(c, t.Title)).TextColor(p.Text.Primary).Wrap()
					}
					if t.Description != "" {
						token.TextLabel.Apply(ui.Text(c, t.Description)).TextColor(p.Text.Secondary).Wrap().Selectable()
					}
					if (t.Action != "" || data.secondary != "") && !inline {
						ui.Row(c).Justify(ui.End).FillWidth().Gap(token.GapControl.Px()).Margin(token.GapInline.Px(), 0, 0, 0).Children(func() {
							if data.secondary != "" {
								first, hasAction = toastSecondary(c, p, t.ID, data), true
							}
							if t.Action != "" {
								first, hasAction = toastAction(c, p, parts, t.Action), true
							}
						})
					}
				})
				closed := closeToast(c, p, parts)
				if !hasAction {
					first = closed
				}
			})
			if data.busy {
				ProgressBar(c, t.Title)
			}
		})
	})
	if focus {
		first.Focus()
	}
}

func actionFitsTitleRow(c *ui.Context, t ui.Toast, data toastData) bool {
	if t.Action == "" || t.Description != "" || data.secondary != "" {
		return false
	}
	titleWidth, _ := c.MeasureText(0, ui.Span{Text: t.Title, Size: token.TextBody.Px(), Weight: token.TextBody.Weight()})
	actionWidth, _ := c.MeasureText(0, ui.Span{Text: t.Action, Size: token.TextBody.Px(), Weight: token.TextBody.Weight()})
	actionWidth += 2*token.PadControlX.Px() + 2*token.BorderWidth.Px()
	room := token.ToastWidth.Px() - token.PadNoticeX.Px() - token.PadInputY.Px() - token.IconSize.Px() - token.IconButtonSize.Px() - 3*token.GapControl.Px()
	return titleWidth+actionWidth <= room
}

func toastAction(c *ui.Context, p *token.Palette, parts ui.ToastParts, label string) ui.Element {
	return toastButton(c, p, parts.ActionButton(), label)
}

func toastSecondary(c *ui.Context, p *token.Palette, id string, data toastData) ui.Element {
	b := toastButton(c, p, ui.ButtonBase(c.Key("secondary")), data.secondary)
	if b.Clicked() {
		if data.onSecondary != nil {
			data.onSecondary()
		}
		c.CloseToast(id)
	}
	return b
}

func toastButton(c *ui.Context, p *token.Palette, b ui.Element, label string) ui.Element {
	b.Height(token.ControlHeightSmall.Px()).Padding(0, token.PadControlX.Px()).Radius(token.RadiusControl.Px()).
		Center().Label(label).FocusRingOffset(token.FocusRingOffsetOuter.Px())
	bg := p.Control.Secondary
	if b.Hovered() || b.Pressed() {
		bg = p.Control.SecondaryHover
	}
	return b.Background(bg).Border(token.BorderWidth.Px(), p.Control.ButtonBorder).Children(func() {
		token.TextBody.WholeText(c, label).TextColor(p.Control.SecondaryText).SingleLine()
	})
}

func closeToast(c *ui.Context, p *token.Palette, parts ui.ToastParts) ui.Element {
	b := parts.CloseButton().Size(token.IconButtonSize.Px(), token.IconButtonSize.Px()).Radius(token.RadiusControl.Px()).Center().Shrink(0).
		Label(closeToastLabel)
	action.Tooltip(c, b, closeToastLabel)
	return b.Background(p.Fill(b.Hovered(), b.Pressed())).Children(func() {
		ui.Icon(c, icons.Close).Size(token.IconSize.Px(), token.IconSize.Px()).TextColor(p.Icon)
	})
}
