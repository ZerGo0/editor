package picker

import (
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/field"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const queryID = "picker-query"

type QueryOptions struct {
	Placeholder string
	Toggles     []field.Toggle
	Actions     []field.Action
	Error       string
	Busy        bool
	Handle      *ui.Handle
}

type Keys struct {
	Up, Down, PageUp, PageDown bool
	Next, Previous             bool
	Side, Submitted, Changed   bool
}

func Query(c *ui.Context, label string, query *string, o QueryOptions) Keys {
	in := field.TextInput(c, label, query, field.InputOptions{Placeholder: o.Placeholder, Icon: icons.Search, Toggles: o.Toggles,
		Actions: o.Actions, Error: o.Error, AutoFocus: true, ID: queryID})
	if o.Handle != nil {
		in.Bind(o.Handle)
	}
	k := Keys{
		Up:       in.Shortcut(0, ui.KeyUp),
		Down:     in.Shortcut(0, ui.KeyDown),
		PageUp:   in.Shortcut(0, ui.KeyPageUp),
		PageDown: in.Shortcut(0, ui.KeyPageDown),
		Next:     in.Shortcut(0, ui.KeyTab),
		Previous: in.Shortcut(ui.Shift, ui.KeyTab),
		Side:     in.Shortcut(ui.Cmd, ui.KeyEnter),
	}
	k.Submitted, k.Changed = in.Submitted(), in.Changed()
	if o.Busy {
		feedback.ProgressBar(c, label)
	}
	return k
}
