package design

import (
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

func Text(c *ui.Context, s string, style token.TextStyle) ui.Element {
	p := token.Of(c)
	color := p.Text.Primary
	if style.Muted() {
		color = p.Text.Secondary
	}
	el := style.Apply(ui.Text(c, style.Format(s))).TextColor(color)
	if style == token.TextHeading || style == token.TextTitle {
		el.Role(ui.RoleHeading)
	}
	return el
}

func Stack(c *ui.Context, gap token.Space, body func()) ui.Element {
	return ui.Column(c).FillWidth().Gap(gap.Px()).Children(body)
}

func Inline(c *ui.Context, gap token.Space, body func()) ui.Element {
	return ui.Row(c).AlignItems(ui.Center).Gap(gap.Px()).Children(body)
}

func Spacer(c *ui.Context) ui.Element {
	return ui.Box(c).Grow(1)
}

func ViewBody(c *ui.Context, body func()) ui.Element {
	return ui.Column(c).FillWidth().Padding(token.GapStack.Px(), token.PadView.Px()).Gap(token.GapStack.Px()).Children(body)
}

func Page(c *ui.Context, title string, body func()) ui.Element {
	scroll := ui.Scroll(c).Fill().Label(title)
	scroll.Children(func() {
		ui.Column(c).FillWidth().Padding(token.PadView.Px()).Gap(token.GapSection.Px()).Children(func() {
			Text(c, title, token.TextHeading)
			body()
		})
	})
	return scroll
}

func Form(c *ui.Context, label string, body func()) ui.Element {
	return ui.Column(c).FillWidth().Gap(token.GapGroup.Px()).Role(ui.RoleGroup).Label(label).Children(body)
}

func Setting(c *ui.Context, title, description string, control func()) ui.Element {
	return ui.Column(c).FillWidth().Gap(token.GapInline.Px()).Role(ui.RoleGroup).Label(title).Children(func() {
		Text(c, title, token.TextStrong)
		if description != "" {
			Text(c, description, token.TextParagraph).Wrap()
		}
		control()
	})
}
