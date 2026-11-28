package findwidget

import (
	"strings"

	"github.com/ZerGo0/editor/internal/design/field"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

type Submit uint8

const (
	NoSubmit Submit = iota
	Enter
	ShiftEnter
	AltEnter
	CmdEnter
	newline
)

const lineBreak = "\n"

type FieldOptions struct {
	Error     string
	Toggles   []field.Toggle
	Disabled  bool
	AutoFocus bool
	History   *History
}

type fieldKeys struct{}

type fieldState struct {
	pending         Submit
	history         *History
	onFirst, onLast bool
}

func Field(c *ui.Context, label string, value *string, o FieldOptions) (ui.Element, Submit) {
	in := field.TextArea(c, label, value, field.AreaOptions{Placeholder: label, Error: o.Error, MaxLines: token.FindInputMaxLines,
		Toggles: o.Toggles, Disabled: o.Disabled, AutoFocus: o.AutoFocus}).FixedLineHeight(token.FindLineHeight.Px())
	st := *ui.Local(in, fieldKeys{}, func() *fieldState { return &fieldState{} })
	start, end := in.TextSelection()
	runes := []rune(*value)
	st.history = o.History
	st.onFirst = !strings.Contains(string(runes[:min(start, len(runes))]), lineBreak)
	st.onLast = !strings.Contains(string(runes[min(end, len(runes)):]), lineBreak)
	in.OnTextKey(st.take)
	got := st.pending
	st.pending = NoSubmit
	if got == newline {
		start, end := in.TextSelection()
		in.ReplaceText(start, end, lineBreak)
		return in, NoSubmit
	}
	return in, got
}

func (st *fieldState) take(k ui.TextKey) bool {
	if k.Key == ui.KeyUp || k.Key == ui.KeyDown {
		return st.browse(k)
	}
	if k.Key != ui.KeyEnter {
		return false
	}
	switch k.Mods {
	case 0:
		st.pending = Enter
	case ui.Shift:
		st.pending = ShiftEnter
	case ui.Alt:
		st.pending = AltEnter
	case ui.Cmd:
		st.pending = CmdEnter
	case ui.Ctrl:
		st.pending = newline
	default:
		return false
	}
	return true
}

func (st *fieldState) browse(k ui.TextKey) bool {
	if st.history == nil || k.Mods != 0 {
		return false
	}
	step, edge := st.history.Next, st.onLast
	if k.Key == ui.KeyUp {
		step, edge = st.history.Prev, st.onFirst
	}
	if !edge {
		return false
	}
	text, ok := step(k.Text())
	if ok {
		k.SetText(text)
	}
	return ok
}
