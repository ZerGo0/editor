package field

import (
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const clearLabel = "Clear"

type SearchOptions struct {
	Placeholder string
	Error       string
	Toggles     []Toggle
	Disabled    bool
	AutoFocus   bool
	NoIcon      bool
	ID          string
}

func SearchInput(c *ui.Context, label string, query *string, o SearchOptions) ui.Element {
	p := token.Of(c)
	f := frame{label: label, error: o.Error, toggles: o.Toggles, disabled: o.Disabled}
	if !o.NoIcon {
		f.leading = icons.Search
	}
	f.trailing = func() {
		if *query == "" {
			return
		}
		if action.IconButton(c, icons.Close, clearLabel+" "+label, action.IconOptions{Tooltip: clearLabel, Keys: "Esc"}).Clicked() {
			*query = ""
		}
	}
	in := f.build(c, p, false, func() ui.Element {
		in := ui.TextInputBase(c.Key(keyOf(o.ID, label)), query).Placeholder(o.Placeholder).Search()
		dropSwallowed(in, query)
		if o.AutoFocus {
			in.AutoFocus()
		}
		return in
	})
	if *query != "" && in.Shortcut(0, ui.KeyEscape) {
		in.ReplaceText(0, len([]rune(*query)), "")
	}
	return in
}
