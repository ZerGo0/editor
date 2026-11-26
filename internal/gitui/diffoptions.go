package gitui

import (
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

type diffOptions struct {
	inline      editor.DiffInline
	expandAll   bool
	wrap        bool
	hideNumbers bool
}

var inlineModes = []struct {
	mode  editor.DiffInline
	label string
}{
	{editor.DiffInlineWordAlt, "Highlight Words"},
	{editor.DiffInlineWordLine, "Highlight Changed Span"},
	{editor.DiffInlineChar, "Highlight Characters"},
	{editor.DiffInlineNone, "Highlight Lines Only"},
}

func (o diffOptions) apply(d *editor.DiffState) {
	d.Inline = o.inline
	d.ExpandUnchanged = o.expandAll
	d.Wrap = o.wrap
	d.NoLineNumbers = o.hideNumbers
}

func (g *Git) diffOptionsButton(c *ui.Context) {
	iconButton(c, "Diff Options", icons.More, false).PopoverMenuButton(func(m *ui.Menu) {
		o := &g.diffOpts
		for _, im := range inlineModes {
			if m.Item(im.label).Radio(o.inline == im.mode).Chosen() {
				o.inline = im.mode
			}
		}
		m.Separator()
		if m.Item("Collapse Unchanged Fragments").Checked(!o.expandAll).Chosen() {
			o.expandAll = !o.expandAll
		}
		if m.Item("Wrap Lines").Checked(o.wrap).Chosen() {
			o.wrap = !o.wrap
		}
		if m.Item("Show Line Numbers").Checked(!o.hideNumbers).Chosen() {
			o.hideNumbers = !o.hideNumbers
		}
	})
}
