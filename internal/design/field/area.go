package field

import (
	"math"

	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

const (
	defaultMinLines = 1
	defaultMaxLines = 10
	fontLineRatio   = 1.25
)

type AreaOptions struct {
	Placeholder string
	Error       string
	MinLines    int
	MaxLines    int
	Toggles     []Toggle
	Actions     []Action
	Disabled    bool
	ReadOnly    bool
	AutoFocus   bool
	FontSize    float32
	ID          string
}

func TextArea(c *ui.Context, label string, value *string, o AreaOptions) ui.Element {
	p := token.Of(c)
	lo, hi := o.MinLines, o.MaxLines
	if lo <= 0 {
		lo = defaultMinLines
	}
	if hi < lo {
		hi = max(lo, defaultMaxLines)
	}
	f := frame{label: label, error: o.Error, toggles: o.Toggles, actions: o.Actions, disabled: o.Disabled}
	in := f.build(c, p, true, func() ui.Element {
		in := ui.TextAreaBase(c.Key(keyOf(o.ID, label)), value).ReadOnly(o.ReadOnly).Lines(lo, hi)
		dropSwallowed(in, value)
		placeholder, cut := oneLinePlaceholder(c, o.Placeholder, in.Bounds().W-2*token.PadInputX.Px())
		in.Placeholder(placeholder)
		if cut && *value == "" {
			action.Tooltip(c, in, o.Placeholder)
		}
		if o.AutoFocus {
			in.AutoFocus()
		}
		return in
	})
	if o.FontSize > 0 {
		in.FontSize(o.FontSize).FixedLineHeight(float32(math.Round(float64(o.FontSize * fontLineRatio))))
	}
	return in
}
