package detail

import (
	"strconv"

	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/badge"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

type HeaderOptions struct {
	Title   string
	Number  int
	Editor  func(c *ui.Context)
	Actions func(c *ui.Context)
	Meta    func(c *ui.Context)
	Toolbar func(c *ui.Context)
	Busy    bool
}

func Header(c *ui.Context, o HeaderOptions) ui.Element {
	p := token.Of(c)
	col := ui.Column(c.Key("detail-header")).FillWidth().Shrink(0).Padding(token.PadPanel.Px(), token.PadView.Px()).Gap(token.GapControl.Px()).
		Background(p.Layer.Panel).BorderWidth(0, 0, token.BorderWidth.Px(), 0).BorderColor(p.Border.Default)
	col.Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(token.GapControl.Px()).Children(func() {
			if o.Editor != nil {
				o.Editor(c)
			} else {
				token.TextHeading.Apply(ui.Text(c, o.Title)).TextColor(p.Text.Primary).Wrap().Grow(1).MinWidth(0).Role(ui.RoleHeading).Level(1)
			}
			if o.Number > 0 {
				token.TextHeading.Apply(ui.Text(c, "#"+strconv.Itoa(o.Number))).TextColor(p.Text.Secondary).SingleLine().Shrink(0)
			}
			if o.Actions != nil {
				action.Toolbar(c, o.Title).Children(func() { o.Actions(c) })
			}
		})
		if o.Meta != nil {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Wrap().Gap(token.GapControl.Px()).Children(func() { o.Meta(c) })
		}
		if o.Toolbar != nil {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Wrap().Gap(token.GapControl.Px()).Role(ui.RoleToolbar).Label(o.Title + " Toolbar").Children(func() { o.Toolbar(c) })
		}
		if o.Busy {
			feedback.ProgressBar(c, o.Title).Absolute().Left(0).Right(0).Bottom(0)
		}
	})
	return col
}

func Body(c *ui.Context, label string, width token.Size, body func()) ui.Element {
	scroll := ui.Scroll(c).Grow(1).FillWidth().Label(label)
	scroll.Children(func() {
		ui.Column(c).FillWidth().MaxWidth(width.Px()).Padding(token.PadView.Px()).Gap(token.GapSection.Px()).Children(body)
	})
	return scroll
}

func Page(c *ui.Context, body func()) ui.Element {
	p := token.Of(c)
	return ui.Column(c).Fill().Background(p.Layer.Editor).Children(body)
}

func Centered(c *ui.Context, body func()) ui.Element {
	return ui.Column(c).Grow(1).FillWidth().Center().Children(body)
}

func Loading(c *ui.Context, label string) ui.Element {
	return Centered(c, func() { feedback.LoadingPane(c, label) })
}

func Failed(c *ui.Context, title string, err error, retry func()) ui.Element {
	return Centered(c, func() { feedback.Error(c, title, err, retry) })
}

type CardOptions struct {
	Count   int
	Actions func(c *ui.Context)
}

func Card(c *ui.Context, title string, o CardOptions, body func()) ui.Element {
	p := token.Of(c)
	card := ui.Column(c.Key("card:"+title)).FillWidth().Radius(token.RadiusCard.Px()).Border(token.BorderWidth.Px(), p.Border.Default).Role(ui.RoleGroup).Label(title)
	card.Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(token.GapControl.Px()).Padding(token.GapInline.Px(), token.PadPanel.Px()).
			Background(p.Layer.Panel).BorderWidth(0, 0, token.BorderWidth.Px(), 0).BorderColor(p.Border.Default).
			Radius(token.RadiusCard.Px(), token.RadiusCard.Px(), 0, 0).Children(func() {
			token.TextStrong.Apply(ui.Text(c, title)).TextColor(p.Text.Primary).SingleLine().Role(ui.RoleHeading).Level(2)
			if o.Count > 0 {
				badge.Count(c, o.Count, badge.CountOptions{})
			}
			ui.Box(c).Grow(1)
			if o.Actions != nil {
				ui.Row(c).Gap(token.GapControl.Px()).AlignItems(ui.Center).Shrink(0).Children(func() { o.Actions(c) })
			}
		})
		ui.Column(c).FillWidth().Padding(token.PadPanel.Px()).Gap(token.GapControl.Px()).Children(body)
	})
	return card
}

func Property(c *ui.Context, label string, values, action func(c *ui.Context)) ui.Element {
	p := token.Of(c)
	return ui.Row(c.Key("property:" + label)).FillWidth().AlignItems(ui.Center).Wrap().Gap(token.GapControl.Px()).Children(func() {
		token.TextLabel.Apply(ui.Text(c, label)).TextColor(p.Text.Secondary).Width(token.LabelColumnWidth.Px()).Shrink(0)
		values(c)
		if action != nil {
			action(c)
		}
	})
}

func Field(c *ui.Context, label string, control func()) ui.Element {
	p := token.Of(c)
	return ui.Column(c.Key("field:" + label)).FillWidth().Gap(token.GapInline.Px()).Role(ui.RoleGroup).Label(label).Children(func() {
		token.TextLabel.Apply(ui.Text(c, label)).TextColor(p.Text.Secondary)
		control()
	})
}

type CommentOptions struct {
	Author  string
	Verb    string
	Time    string
	Actions func(c *ui.Context)
}

func Comment(c *ui.Context, o CommentOptions, body func()) ui.Element {
	p := token.Of(c)
	card := ui.Column(c).FillWidth().Padding(token.GapControl.Px(), token.PadPanel.Px()).Gap(token.GapControl.Px()).Radius(token.RadiusCard.Px()).
		Border(token.BorderWidth.Px(), p.Border.Default).Role(ui.RoleArticle).Label(o.Author + " " + o.Verb)
	card.Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Wrap().Gap(token.GapRowIcon.Px()).Children(func() {
			badge.Avatar(c, o.Author, token.AvatarSize)
			token.TextStrong.WholeText(c, o.Author).TextColor(p.Text.Primary).SingleLine()
			token.TextLabel.WholeText(c, o.Verb).TextColor(p.Text.Secondary).SingleLine()
			if o.Time != "" {
				token.TextLabel.WholeText(c, o.Time).TextColor(p.Text.Secondary).SingleLine()
			}
			ui.Box(c).Grow(1)
			if o.Actions != nil {
				ui.Row(c).Gap(token.GapTight.Px()).AlignItems(ui.Center).Shrink(0).Children(func() { o.Actions(c) })
			}
		})
		if body != nil {
			body()
		}
	})
	return card
}

func Activity(c *ui.Context, actor, text, when string) ui.Element {
	p := token.Of(c)
	return ui.Row(c).FillWidth().AlignItems(ui.Center).Wrap().Gap(token.GapRowIcon.Px()).Children(func() {
		token.TextLabel.Apply(ui.Text(c, actor)).FontWeight(token.WeightSemibold).TextColor(p.Text.Primary)
		token.TextLabel.Apply(ui.Text(c, text)).TextColor(p.Text.Secondary)
		token.TextLabel.Apply(ui.Text(c, when)).TextColor(p.Text.Muted)
	})
}
