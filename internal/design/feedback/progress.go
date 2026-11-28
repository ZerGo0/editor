package feedback

import (
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const Indeterminate = -1

type TaskOptions struct {
	Value    float64
	Cancel   string
	Keys     string
	OnCancel func()
}

func ProgressBar(c *ui.Context, label string) ui.Element {
	return Progress(c, label, Indeterminate)
}

func Progress(c *ui.Context, label string, value float64) ui.Element {
	p := token.Of(c)
	return ui.Progress(c, value).Height(token.ProgressHeight.Px()).Radius(0).FillWidth().Background(ui.Transparent).
		TextColor(p.Accent.Base).Label(label)
}

func Spinner(c *ui.Context, label string) ui.Element {
	p := token.Of(c)
	return ui.Spinner(c).Size(token.IconSize.Px(), token.IconSize.Px()).TextColor(p.Text.Secondary).Label(label)
}

func Task(c *ui.Context, label string, o TaskOptions) ui.Element {
	p := token.Of(c)
	return ui.Column(c).FillWidth().Shrink(0).Children(func() {
		ui.Row(c).FillWidth().Height(token.RowHeight.Px()).Role(ui.RoleStatus).Label(label).Padding(0, token.PadInputY.Px(), 0, token.PadView.Px()).
			Gap(token.GapControl.Px()).AlignItems(ui.Center).Children(func() {
			token.TextLabel.ApplyLine(ui.Text(c, label)).TextColor(p.Text.Secondary).SingleLine().Grow(1).MinWidth(0).AccessHidden(true)
			if o.OnCancel != nil && action.IconButton(c, icons.Close, o.Cancel, action.IconOptions{Keys: o.Keys}).Clicked() {
				o.OnCancel()
			}
		})
		Progress(c, label, o.Value)
	})
}
