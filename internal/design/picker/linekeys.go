package picker

import (
	"strings"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

const lineBreak = "\n"

type lineKeysID struct{}

type lineKeyState struct {
	pending                    Keys
	newline                    bool
	upToResults, downToResults bool
}

func lineKeys(in ui.Element, query string, k *Keys) {
	st := *ui.Local(in, lineKeysID{}, func() *lineKeyState { return &lineKeyState{} })
	start, end := in.TextSelection()
	lines := strings.Contains(query, lineBreak) || in.Bounds().H > token.ControlHeight.Px()
	st.upToResults = !lines || start <= 0
	st.downToResults = !lines || end >= len([]rune(query))
	in.OnTextKey(st.take)
	got, newline := st.pending, st.newline
	st.pending, st.newline = Keys{}, false
	if newline {
		start, end := in.TextSelection()
		in.ReplaceText(start, end, lineBreak)
	}
	k.Up, k.Down, k.PageUp, k.PageDown = got.Up, got.Down, got.PageUp, got.PageDown
	k.Side, k.Submitted = got.Side, got.Submitted
	k.Older, k.Newer = got.Older, got.Newer
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
	case k.Key == ui.KeyUp && k.Mods == 0 && st.upToResults:
		p.Up = true
	case k.Key == ui.KeyDown && k.Mods == 0 && st.downToResults:
		p.Down = true
	case k.Key == ui.KeyUp && k.Mods == ui.Alt:
		p.Older = true
	case k.Key == ui.KeyDown && k.Mods == ui.Alt:
		p.Newer = true
	case k.Key == ui.KeyPageUp && k.Mods == 0:
		p.PageUp = true
	case k.Key == ui.KeyPageDown && k.Mods == 0:
		p.PageDown = true
	default:
		return false
	}
	return true
}
