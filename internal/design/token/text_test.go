package token

import (
	"math"
	"testing"
)

func TestTextLinesAreWholePixels(t *testing.T) {
	styles := map[string]TextStyle{
		"Body": TextBody, "Strong": TextStrong, "Label": TextLabel, "Paragraph": TextParagraph, "Caption": TextCaption,
		"Title": TextTitle, "Heading": TextHeading, "Badge": TextBadge, "Mono": TextMono,
	}
	for name, s := range styles {
		if s.line != float32(math.Round(float64(s.line))) || s.line < s.size {
			t.Errorf("Text%s line height %v; want a whole px of at least the %v px size", name, s.line, s.size)
		}
	}
}
