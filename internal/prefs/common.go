package prefs

import "github.com/egoist/mygo/ui"

const toolbarPadding = 12

func toolbar(c *ui.Context, fn func()) {
	t := c.Theme()
	ui.Row(c).Gap(8).Padding(toolbarPadding).AlignItems(ui.Center).FillWidth().Shrink(0).
		BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(fn)
}

func problems(c *ui.Context, t *ui.Theme, errs []string) {
	if len(errs) == 0 {
		return
	}
	ui.Column(c).Gap(4).Padding(8, toolbarPadding).FillWidth().Shrink(0).Background(t.Surface).
		BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
		for _, e := range errs {
			ui.Text(c, e).TextColor(t.Warning)
		}
	})
}
