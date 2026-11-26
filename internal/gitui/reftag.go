package gitui

import (
	"math"

	"github.com/ZerGo0/editor/internal/design/badge"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

const (
	refNameMax      = 24
	refNameEllipsis = "…"
	tagSides        = 2
)

func refLabel(name string) string {
	runes := []rune(name)
	if len(runes) <= refNameMax {
		return name
	}
	return string(runes[:refNameMax-1]) + refNameEllipsis
}

func wholeTag(c *ui.Context, text string, o badge.TagOptions) ui.Element {
	textW, _ := c.MeasureText(0, ui.Span{Text: text, Size: token.TextCaption.Px(), Weight: token.TextCaption.Weight()})
	width := float32(textW) + tagSides*(token.PadChipX.Px()+token.BorderWidth.Px())
	if o.Icon != nil {
		width += token.IconSizeGlyph.Px() + token.GapInline.Px()
	}
	whole := float32(math.Ceil(float64(width)))
	return badge.Tag(c, text, o).Width(whole)
}
