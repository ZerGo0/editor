package list

import (
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

func Spans(label string, matches []int, color, highlight ui.Color) []ui.Span {
	if len(matches) == 0 {
		return []ui.Span{{Text: label, Color: color}}
	}
	marked := make(map[int]bool, len(matches))
	for _, m := range matches {
		marked[m] = true
	}
	var spans []ui.Span
	runes := []rune(label)
	start := 0
	for i := 1; i <= len(runes); i++ {
		if i < len(runes) && marked[i] == marked[start] {
			continue
		}
		s := ui.Span{Text: string(runes[start:i]), Color: color}
		if marked[start] {
			s.Color, s.Weight = highlight, token.WeightBold
		}
		spans = append(spans, s)
		start = i
	}
	return spans
}
