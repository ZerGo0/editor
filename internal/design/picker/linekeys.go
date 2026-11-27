package picker

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

const lineBreak = "\n"

type lineKeysID struct{}

type lineKeyState struct {
	pending             Keys
	newline             bool
	firstLine, lastLine bool
}

func lineKeys(in ui.Element, query string, k *Keys) {
	st := *ui.Local(in, lineKeysID{}, func() *lineKeyState { return &lineKeyState{} })
	runes := []rune(query)
	_, caret := in.TextAnchorCaret()
	caret = min(max(caret, 0), len(runes))
	st.firstLine = !strings.Contains(string(runes[:caret]), lineBreak)
	st.lastLine = !strings.Contains(string(runes[caret:]), lineBreak)
	in.OnTextKey(st.take)
	got, newline := st.pending, st.newline
	st.pending, st.newline = Keys{}, false
	if newline {
		start, end := in.TextSelection()
		in.ReplaceText(start, end, lineBreak)
	}
	k.Up, k.Down, k.PageUp, k.PageDown = got.Up, got.Down, got.PageUp, got.PageDown
	k.Side, k.Submitted = got.Side, got.Submitted
}

func (st *lineKeyState) take(k ui.TextKey) bool {
	p := &st.pending
	switch {
	case k.Key == ui.KeyEnter && k.Mods == 0:
		p.Submitted = true
	case k.Key == ui.KeyEnter && k.Mods == ui.Cmd:
		p.Side = true
	case k.Key == ui.KeyEnter && k.Mods == ui.Ctrl:
		st.newline = true
	case k.Key == ui.KeyUp && k.Mods == 0 && st.firstLine:
		p.Up = true
	case k.Key == ui.KeyDown && k.Mods == 0 && st.lastLine:
		p.Down = true
	case k.Key == ui.KeyPageUp && k.Mods == 0:
		p.PageUp = true
	case k.Key == ui.KeyPageDown && k.Mods == 0:
		p.PageDown = true
	default:
		return false
	}
	return true
}
