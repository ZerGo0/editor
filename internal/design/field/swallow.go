package field

import (
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
)

// TECHDEBT: Package-level because windows build one at a time on the main thread; keep it per window if builds ever run in parallel.
var swallowed string

func Swallow(text string) { swallowed = text }

func dropSwallowed(in ui.Element, value *string) {
	if swallowed == "" || !in.Focused() || !in.Changed() {
		return
	}
	start, end := in.TextSelection()
	n := utf8.RuneCountInString(swallowed)
	runes := []rune(*value)
	if start != end || start < n || start > len(runes) || string(runes[start-n:start]) != swallowed {
		return
	}
	swallowed = ""
	in.ReplaceText(start-n, start, "")
}
