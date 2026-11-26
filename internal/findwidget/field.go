package findwidget

import (
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
	Error    string
	Toggles  []field.Toggle
	Disabled bool
}

type fieldKeys struct{}

type fieldState struct {
	pending Submit
}

func Field(c *ui.Context, label string, value *string, o FieldOptions) (ui.Element, Submit) {
	in := field.TextArea(c, label, value, field.AreaOptions{Placeholder: label, Error: o.Error, MaxLines: token.FindInputMaxLines,
		Toggles: o.Toggles, Disabled: o.Disabled}).FixedLineHeight(token.FindLineHeight.Px())
	st := *ui.Local(in, fieldKeys{}, func() *fieldState { return &fieldState{} })
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
