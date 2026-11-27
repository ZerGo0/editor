package feedback

import (
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

const indeterminate = -1

func ProgressBar(c *ui.Context, label string) ui.Element {
	return Progress(c, label, indeterminate)
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
