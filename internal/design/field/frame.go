package field

import (
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

const (
	messageFocusInset = 0
)

type Toggle struct {
	Icon  *ui.SVG
	Label string
	Keys  string
	On    *bool
}

type frame struct {
	label    string
	error    string
	leading  *ui.SVG
	toggles  []Toggle
	trailing func()
	disabled bool
}

func (f frame) build(c *ui.Context, p *token.Palette, multiline bool, input func() ui.Element) ui.Element {
	var in ui.Element
	col := ui.Column(c).FillWidth().Gap(0)
	col.Children(func() {
		box := ui.Row(c).FillWidth().MinHeight(token.ControlHeight.Px()).Radius(token.RadiusControl.Px()).Background(p.Layer.Input).
			Padding(0, token.PadInputEnd.Px(), 0, 0).Gap(token.GapTight.Px()).Disabled(f.disabled)
		if multiline {
			box.AlignItems(ui.Start)
		} else {
			box.AlignItems(ui.Center)
		}
		box.Children(func() {
			if f.leading != nil {
				ui.Icon(c, f.leading).Size(token.IconSize.Px(), token.IconSize.Px()).TextColor(p.Text.Secondary).Shrink(0).
					Margin(0, 0, 0, token.PadInputX.Px()).AccessHidden(true)
			}
			in = input().Grow(1).MinWidth(0).Padding(token.PadInputY.Px(), token.PadInputX.Px()).FontSize(token.TextBody.Px()).
				TextColor(p.Text.Primary).PlaceholderColor(p.Text.Muted).CaretColor(p.Text.Primary).FocusRing(false).Label(f.label)
			if f.trailing != nil {
				f.trailing()
			}
			for _, t := range f.toggles {
				action.ToggleIconButton(c, t.Icon, t.Label, t.On, action.IconOptions{Keys: t.Keys, KeepFocus: true})
			}
		})
		border := p.Border.Input
		switch {
		case f.error != "":
			border = p.Severity.ErrorBorder
		case in.Focused():
			border = p.Accent.Focus
		}
		box.Border(token.BorderWidth.Px(), border)
		if f.error == "" {
			return
		}
		in.Invalid(true)
		msg := ui.Row(c).FillWidth().Padding(token.PadInputY.Px(), token.PadInputX.Px()).Background(p.Severity.ErrorBackground).
			Border(token.BorderWidth.Px(), p.Severity.ErrorBorder).Radius(0, 0, token.RadiusControl.Px(), token.RadiusControl.Px()).
			Role(ui.RoleAlert).Label(f.error)
		msg.Children(func() {
			token.TextLabel.Apply(ui.Text(c, f.error)).TextColor(p.Text.Primary).Wrap().Grow(1).MinWidth(0).AccessHidden(true)
		})
		in.ErrorMessage(msg)
	})
	return in
}
