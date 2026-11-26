package pane

import (
	"strings"

	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

type TitleOptions struct {
	Actions     func(c *ui.Context)
	ShowActions bool
	Busy        bool
}

func ViewTitle(c *ui.Context, title string, o TitleOptions) ui.Element {
	p := token.Of(c)
	row := ui.Row(c.Key("view-title:"+title)).Height(token.ViewTitleHeight.Px()).FillWidth().Shrink(0).AlignItems(ui.Center).
		Padding(0, token.PadRow.Px(), 0, token.PadView.Px()).Gap(token.GapInline.Px())
	show := o.ShowActions || row.Hovered() || row.FocusWithin()
	row.Children(func() {
		token.TextCaption.Apply(ui.Text(c, strings.ToUpper(title))).TextColor(p.Text.Primary).SingleLine().
			Grow(1).MinWidth(0).Role(ui.RoleHeading).Level(1).Label(title)
		if o.Actions != nil {
			actions := ui.Row(c).Gap(token.GapTight.Px()).AlignItems(ui.Center).Shrink(0).Role(ui.RoleToolbar).Label(title + " Actions")
			if !show {
				actions.Opacity(token.OpacityHidden.Value())
			}
			actions.Children(func() { o.Actions(c) })
		}
		if o.Busy {
			feedback.ProgressBar(c, title).Absolute().Left(0).Right(0).Bottom(0)
		}
	})
	return row
}
