package field

import (
	"sort"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

const placeholderEllipsis = "…"

func oneLinePlaceholder(c *ui.Context, text string, room float32) (string, bool) {
	if text == "" || room <= 0 || textWidth(c, text) <= room {
		return text, false
	}
	runes := []rune(text)
	fit := sort.Search(len(runes), func(n int) bool {
		return textWidth(c, string(runes[:n+1])+placeholderEllipsis) > room
	})
	return string(runes[:fit]) + placeholderEllipsis, true
}

func textWidth(c *ui.Context, s string) float32 {
	w, _ := c.MeasureText(0, ui.Span{Text: s, Size: token.TextBody.Px(), Weight: token.TextBody.Weight()})
	return w
}
