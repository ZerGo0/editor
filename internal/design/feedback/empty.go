package feedback

import (
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

type EmptyOptions struct {
	Icon     *ui.SVG
	Title    string
	Message  string
	Code     string
	Action   string
	OnAction func()
	Link     string
	OnLink   func()
}

func Empty(c *ui.Context, o EmptyOptions) ui.Element {
	p := token.Of(c)
	return ui.Column(c).FillWidth().Padding(token.PadEmpty.Px(), token.PadView.Px()).Gap(token.GapStack.Px()).AlignItems(ui.Center).
		Children(func() {
			ui.Column(c).MaxWidth(token.EmptyStateWidth.Px()).FillWidth().Gap(token.GapStack.Px()).AlignItems(ui.Center).Children(func() {
				if o.Icon != nil {
					ui.Icon(c, o.Icon).Size(token.IconSizeLarge.Px(), token.IconSizeLarge.Px()).TextColor(p.Text.Secondary).AccessHidden(true)
				}
				if o.Title != "" {
					token.TextStrong.Apply(ui.Text(c, o.Title)).TextColor(p.Text.Primary).
						TextAlign(ui.Center).Wrap().Role(ui.RoleHeading).Level(2)
				}
				if o.Message != "" {
					token.TextParagraph.Apply(ui.Text(c, o.Message)).TextColor(p.Text.Secondary).TextAlign(ui.Center).Wrap()
				}
				if o.Code != "" {
					token.TextMono.Apply(ui.Text(c, o.Code)).TextColor(p.Text.Primary).Selectable().Wrap().Background(p.Layer.Input).
						Radius(token.RadiusControl.Px()).Padding(token.PadInputY.Px(), token.PadInputX.Px())
				}
				if o.Action != "" && o.OnAction != nil {
					if action.Button(c, o.Action, action.ButtonOptions{Variant: action.Primary}).Margin(token.GapInline.Px(), 0, 0, 0).Clicked() {
						o.OnAction()
					}
				}
				if o.Link != "" && o.OnLink != nil {
					if action.Button(c, o.Link, action.ButtonOptions{Variant: action.Ghost, Small: true}).Clicked() {
						o.OnLink()
					}
				}
			})
		})
}

func Error(c *ui.Context, title string, err error, retry func()) ui.Element {
	message := ""
	if err != nil {
		message = err.Error()
	}
	o := EmptyOptions{Title: title, Message: message}
	if retry != nil {
		o.Action, o.OnAction = "Retry", retry
	}
	return Empty(c, o).Role(ui.RoleAlert).Label(title)
}
