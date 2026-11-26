package badge

import (
	"strconv"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

type TagOptions struct {
	Tone    token.Tone
	Icon    *ui.SVG
	Tooltip string
	Hex     string
}

const (
	hexLength   = 6
	hexBase     = 16
	hexByteBits = 8
	rgbBits     = 24
	byteMask    = 0xff
)

func hexColor(hex string) (ui.Color, bool) {
	if len(hex) != hexLength {
		return ui.Color{}, false
	}
	v, err := strconv.ParseUint(hex, hexBase, rgbBits)
	if err != nil {
		return ui.Color{}, false
	}
	return ui.RGB(uint8(v>>(2*hexByteBits)&byteMask), uint8(v>>hexByteBits&byteMask), uint8(v&byteMask)), true
}

func Tag(c *ui.Context, text string, o TagOptions) ui.Element {
	p := token.Of(c)
	tint := p.Tone(o.Tone)
	if c, ok := hexColor(o.Hex); ok {
		tint = c
	}
	el := ui.Row(c).Height(token.TagHeight.Px()).Padding(0, token.PadChipX.Px()).Gap(token.GapInline.Px()).AlignItems(ui.Center).Shrink(0).
		Radius(token.RadiusPill.Px()).Background(tint.Alpha(token.OpacityTintFill.Value())).Border(token.BorderWidth.Px(), tint.Alpha(token.OpacityTintBorder.Value())).Label(text)
	if o.Tooltip != "" {
		el.Tooltip(o.Tooltip)
	}
	el.Children(func() {
		if o.Icon != nil {
			ui.Icon(c, o.Icon).Size(token.IconSizeGlyph.Px(), token.IconSizeGlyph.Px()).TextColor(p.Text.Primary).AccessHidden(true)
		}
		token.TextCaption.Apply(ui.Text(c, text)).TextColor(p.Text.Primary).SingleLine().AccessHidden(true)
	})
	return el
}
