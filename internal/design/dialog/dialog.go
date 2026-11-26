package dialog

import (
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

type Options struct {
	Width token.Size
	Hint  string
}

func Modal(c *ui.Context, open *bool, title string, o Options, body func()) ui.Element {
	p := token.Of(c)
	width := o.Width
	if width.Px() <= 0 {
		width = token.DialogWidth
	}
	return ui.DialogBase(c, open, func(backdrop, panel ui.Element) {
		backdrop.Background(p.Shadow.Alpha(token.OpacityScrim.Value()))
		panel.Width(width.Px()).Padding(token.PadPanel.Px()).Radius(token.RadiusDialog.Px()).Background(p.Layer.Widget).
			Border(token.BorderWidth.Px(), p.Border.Widget).Label(title)
		token.ElevationDialog.Apply(panel, p)
		ui.Column(c).FillWidth().Gap(token.GapGroup.Px()).Children(func() {
			token.TextHeading.Apply(ui.Text(c, title)).TextColor(p.Text.Primary).Wrap().Role(ui.RoleHeading).Level(1)
			if o.Hint != "" {
				token.TextParagraph.Apply(ui.Text(c, o.Hint)).TextColor(p.Text.Secondary).Wrap()
			}
			body()
		})
	})
}

func Actions(c *ui.Context, buttons func()) ui.Element {
	return ui.Row(c).FillWidth().Justify(ui.End).AlignItems(ui.Center).Gap(token.GapControl.Px()).Children(buttons)
}
