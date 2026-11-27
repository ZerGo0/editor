package token

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

const FontMono = "Menlo"

const (
	WeightRegular  = 400
	WeightMedium   = 500
	WeightSemibold = 600
	WeightBold     = 700
)

type TextStyle struct {
	size   float32
	weight int
	line   float32
	mono   bool
	upper  bool
	muted  bool
	rise   Size
}

var (
	TextBody      = TextStyle{size: 13, weight: WeightRegular, line: 16}
	TextStrong    = TextStyle{size: 13, weight: WeightSemibold, line: 16}
	TextLabel     = TextStyle{size: 12, weight: WeightRegular, line: 16, rise: TextRise}
	TextParagraph = TextStyle{size: 12, weight: WeightRegular, line: 18, muted: true}
	TextCaption   = TextStyle{size: 11, weight: WeightRegular, line: 14, muted: true}
	TextTitle     = TextStyle{size: 11, weight: WeightBold, line: 14, upper: true}
	TextHeading   = TextStyle{size: 15, weight: WeightSemibold, line: 20}
	TextBadge     = TextStyle{size: 11, weight: WeightSemibold, line: 14}
	TextMono      = TextStyle{size: 12, weight: WeightRegular, line: 16, mono: true}
)

func (t TextStyle) Px() float32 { return t.size }

func (t TextStyle) Weight() int { return t.weight }

func (t TextStyle) LineHeight() float32 { return t.line }

func (t TextStyle) Muted() bool { return t.muted }

func (t TextStyle) Format(s string) string {
	if t.upper {
		return strings.ToUpper(s)
	}
	return s
}

func (t TextStyle) Apply(el ui.Element) ui.Element {
	el.FontSize(t.size).FontWeight(t.weight).FixedLineHeight(t.line)
	if t.mono {
		el.Font(FontMono)
	}
	return el
}
