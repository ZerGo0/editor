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

const (
	LineHeight      float32 = 1.4
	LineHeightTight float32 = 1.2
)

type TextStyle struct {
	size   float32
	weight int
	line   float32
	mono   bool
	upper  bool
	muted  bool
}

var (
	TextBody      = TextStyle{size: 13, weight: WeightRegular}
	TextStrong    = TextStyle{size: 13, weight: WeightSemibold}
	TextLabel     = TextStyle{size: 12, weight: WeightRegular}
	TextParagraph = TextStyle{size: 12, weight: WeightRegular, line: LineHeight, muted: true}
	TextCaption   = TextStyle{size: 11, weight: WeightRegular, muted: true}
	TextTitle     = TextStyle{size: 11, weight: WeightBold, upper: true}
	TextHeading   = TextStyle{size: 15, weight: WeightSemibold}
	TextBadge     = TextStyle{size: 11, weight: WeightSemibold}
	TextMono      = TextStyle{size: 12, weight: WeightRegular, mono: true}
)

func (t TextStyle) Px() float32 { return t.size }

func (t TextStyle) Weight() int { return t.weight }

func (t TextStyle) Muted() bool { return t.muted }

func (t TextStyle) Format(s string) string {
	if t.upper {
		return strings.ToUpper(s)
	}
	return s
}

func (t TextStyle) Apply(el ui.Element) ui.Element {
	el.FontSize(t.size).FontWeight(t.weight)
	if t.line > 0 {
		el.LineHeight(t.line)
	}
	if t.mono {
		el.Font(FontMono)
	}
	return el
}
