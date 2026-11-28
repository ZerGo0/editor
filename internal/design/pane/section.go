package pane

import (
	"strings"

	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/badge"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

type SectionOptions struct {
	Count       int
	Actions     func(c *ui.Context)
	ShowActions bool
	Busy        bool
	Border      bool
}

func Header(c *ui.Context, title string, open *bool, o SectionOptions) ui.Element {
	p := token.Of(c)
	row := ui.Row(c.Key("header:"+title)).Height(token.HeaderHeight.Px()).FillWidth().Shrink(0).AlignItems(ui.Center).
		Padding(0, token.PadRow.Px(), 0, 0).Gap(token.GapInline.Px())
	if o.Border {
		row.Height(token.HeaderHeight.Px()+token.BorderWidth.Px()).BorderWidth(token.BorderWidth.Px(), 0, 0, 0).BorderColor(p.Border.Section)
	}
	show := o.ShowActions || row.Hovered() || row.FocusWithin()
	row.Children(func() {
		toggle := ui.ButtonBase(c.Key("toggle")).Grow(1).MinWidth(0).FillHeight().AlignItems(ui.Center).Justify(ui.Start).
			Gap(token.GapTight.Px()).Role(ui.RoleDisclosure).Label(title).Expanded(*open).FocusRingOffset(-token.FocusRingWidth.Px())
		target := float32(0)
		if *open {
			target = token.TwistieOpenAngle
		}
		toggle.Children(func() {
			ui.Icon(c, icons.ChevronRight).Size(token.IconSize.Px(), token.IconSize.Px()).TextColor(p.Icon).Rotate(target).
				Margin(0, 0, 0, token.GapTight.Px()).Shrink(0)
			token.TextCaption.Apply(ui.Text(c, strings.ToUpper(title))).FontWeight(token.WeightBold).
				TextColor(p.Text.Primary).SingleLine().MinWidth(0).Shrink(1)
		})
		if toggle.Clicked() {
			*open = !*open
		}
		if o.Actions != nil {
			actions := action.Toolbar(c, title)
			if !show {
				actions.Opacity(token.OpacityHidden.Value())
			}
			actions.Children(func() { o.Actions(c) })
		}
		if o.Count > 0 {
			badge.Count(c, o.Count, badge.CountOptions{})
		}
		if o.Busy {
			feedback.ProgressBar(c, title).Absolute().Left(0).Right(0).Bottom(0)
		}
	})
	return row
}

func Section(c *ui.Context, title string, open *bool, o SectionOptions, body func(c *ui.Context)) ui.Element {
	col := ui.Column(c.Key(title)).FillWidth().MinHeight(0).Shrink(0)
	col.Children(func() {
		Header(c, title, open, o)
		if *open {
			ui.Column(c.Key("body")).FillWidth().Grow(1).MinHeight(0).Children(func() { body(c) })
		}
	})
	if *open {
		col.Grow(1).Shrink(1)
	}
	return col
}
