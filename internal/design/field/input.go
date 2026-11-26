package field

import (
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

type InputOptions struct {
	Placeholder string
	Error       string
	Icon        *ui.SVG
	Toggles     []Toggle
	Actions     []Action
	Disabled    bool
	ReadOnly    bool
	AutoFocus   bool
	ID          string
}

func TextInput(c *ui.Context, label string, value *string, o InputOptions) ui.Element {
	p := token.Of(c)
	f := frame{label: label, error: o.Error, leading: o.Icon, toggles: o.Toggles, actions: o.Actions, disabled: o.Disabled}
	return f.build(c, p, false, func() ui.Element {
		in := ui.TextInputBase(c.Key(keyOf(o.ID, label)), value).Placeholder(o.Placeholder).ReadOnly(o.ReadOnly)
		if o.AutoFocus {
			in.AutoFocus()
		}
		return in
	})
}

func keyOf(id, label string) string {
	if id != "" {
		return id
	}
	return label
}
