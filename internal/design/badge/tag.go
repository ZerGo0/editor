package badge

import (
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

type TagOptions struct {
	Tone    token.Tone
	Icon    *ui.SVG
	Tooltip string
}

func Tag(c *ui.Context, text string, o TagOptions) ui.Element {
	p := token.Of(c)
	tint := p.Tone(o.Tone)
	el := ui.Row(c).Height(token.TagHeight.Px()).Padding(0, token.PadChipX.Px()).Gap(token.GapInline.Px()).AlignItems(ui.Center).Shrink(0).
		Radius(token.RadiusPill.Px()).Background(tint.Alpha(token.OpacityTintFill.Value())).Border(token.BorderWidth.Px(), tint.Alpha(token.OpacityTintBorder.Value())).Label(text)
	if o.Tooltip != "" {
		el.Tooltip(o.Tooltip)
	}
	el.Children(func() {
		if o.Icon != nil {
			ui.Icon(c, o.Icon).Size(token.IconSizeGlyph.Px(), token.IconSizeGlyph.Px()).TextColor(p.Text.Primary).AccessHidden(true)
		}
		ui.Text(c, text).FontSize(token.TextCaption.Px()).TextColor(p.Text.Primary).SingleLine().AccessHidden(true)
	})
	return el
}
