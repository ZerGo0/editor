package badge

import (
	"cmp"
	"math"
	"strconv"

	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

type TagOptions struct {
	Tone     token.Tone
	Icon     *ui.SVG
	Tooltip  string
	Hex      string
	MaxWidth float32
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

func tagMax(o TagOptions) float32 {
	if o.MaxWidth > 0 {
		return o.MaxWidth
	}
	return token.TagMaxWidth.Px()
}

func tagChrome(o TagOptions, text string) float32 {
	chrome := 2 * (token.PadChipX.Px() + token.BorderWidth.Px())
	if o.Icon != nil {
		chrome += token.IconSizeGlyph.Px()
		if text != "" {
			chrome += token.GapInline.Px()
		}
	}
	return chrome
}

func TagWidth(c *ui.Context, text string, o TagOptions) float32 {
	chrome := tagChrome(o, text)
	if text == "" {
		return chrome
	}
	w, _ := c.MeasureText(0, ui.Span{Text: text, Size: token.TextCaption.Px(), Weight: token.TextCaption.Weight()})
	return min(chrome+float32(math.Ceil(float64(w))), tagMax(o))
}

func Tag(c *ui.Context, text string, o TagOptions) ui.Element {
	p := token.Of(c)
	tint := p.Tone(o.Tone)
	if c, ok := hexColor(o.Hex); ok {
		tint = c
	}
	el := ui.Row(c).Height(token.TagHeight.Px()).Padding(0, token.PadChipX.Px()).Gap(token.GapInline.Px()).AlignItems(ui.Center).Shrink(0).
		MaxWidth(tagMax(o)).Radius(token.RadiusPill.Px()).Background(tint.Alpha(token.OpacityTintFill.Value())).
		Border(token.BorderWidth.Px(), tint.Alpha(token.OpacityTintBorder.Value())).Label(cmp.Or(text, o.Tooltip))
	room := tagMax(o) - tagChrome(o, text)
	cut := false
	el.Children(func() {
		if o.Icon != nil {
			ui.Icon(c, o.Icon).Size(token.IconSizeGlyph.Px(), token.IconSizeGlyph.Px()).TextColor(p.Text.Primary).AccessHidden(true)
		}
		if text == "" {
			return
		}
		var label ui.Element
		label, cut = token.TextCaption.WholeTextMax(c, text, room)
		label.TextColor(p.Text.Primary).AccessHidden(true)
	})
	switch {
	case o.Tooltip != "":
		action.Tooltip(c, el, o.Tooltip)
	case cut:
		action.Tooltip(c, el, text)
	}
	return el
}
