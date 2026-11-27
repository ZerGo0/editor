package picker

import (
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/field"
	"github.com/ZerGo0/editor/internal/design/token"
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
	NoAutoFocus bool
	Multiline   bool
}

type Keys struct {
	Up, Down, PageUp, PageDown bool
	Next, Previous             bool
	Side, Submitted, Changed   bool
}

func Query(c *ui.Context, label string, query *string, o QueryOptions) Keys {
	var in ui.Element
	if o.Multiline {
		in = field.TextArea(c, label, query, field.AreaOptions{Placeholder: o.Placeholder, Icon: icons.Search, Toggles: o.Toggles,
			Actions: o.Actions, Error: o.Error, AutoFocus: !o.NoAutoFocus, ID: queryID, MaxLines: token.FindInputMaxLines})
	} else {
		in = field.TextInput(c, label, query, field.InputOptions{Placeholder: o.Placeholder, Icon: icons.Search, Toggles: o.Toggles,
			Actions: o.Actions, Error: o.Error, AutoFocus: !o.NoAutoFocus, ID: queryID})
	}
	if o.Handle != nil {
		in.Bind(o.Handle)
	}
	k := Keys{
		Next:     in.Shortcut(0, ui.KeyTab),
		Previous: in.Shortcut(ui.Shift, ui.KeyTab),
	}
	if o.Multiline {
		lineKeys(in, *query, &k)
	} else {
		k.Up, k.Down = in.Shortcut(0, ui.KeyUp), in.Shortcut(0, ui.KeyDown)
		k.PageUp, k.PageDown = in.Shortcut(0, ui.KeyPageUp), in.Shortcut(0, ui.KeyPageDown)
		k.Side, k.Submitted = in.Shortcut(ui.Cmd, ui.KeyEnter), in.Submitted()
	}
	k.Changed = in.Changed()
	if o.Busy {
		feedback.ProgressBar(c, label).Absolute().Left(0).Right(0).Bottom(0)
	}
	return k
}
