package searchview

import (
	"github.com/ZerGo0/editor/internal/search/everywhere"
	"github.com/ZerGo0/editor/internal/search/symbols"
	"github.com/egoist/mygo/ui"
)

var kindLetters = map[symbols.Kind]string{
	symbols.Function: "f", symbols.Method: "m", symbols.Class: "C", symbols.Struct: "S", symbols.Interface: "I",
	symbols.Enum: "E", symbols.Type: "T", symbols.Const: "K", symbols.Var: "v", symbols.Field: "F", symbols.Module: "M",
	symbols.Heading: "H", symbols.Key: "k", symbols.Selector: "#", symbols.Table: "T", symbols.Target: ">",
}

func kindColor(t *ui.Theme, k symbols.Kind) ui.Color {
	switch k {
	case symbols.Function, symbols.Method:
		return t.Accent
	case symbols.Class, symbols.Struct, symbols.Interface, symbols.Enum, symbols.Type:
		return t.Warning
	case symbols.Const, symbols.Var, symbols.Field:
		return t.Success
	}
	return t.TextMuted
}

func kindBadge(c *ui.Context, res everywhere.Result, selected bool) {
	t := c.Theme()
	color := kindColor(t, res.Symbol)
	bg, fg := color.Alpha(tintAlpha), color
	if selected {
		bg, fg = t.AccentText.Alpha(tintAlpha), t.AccentText
	}
	ui.Text(c, kindLetters[res.Symbol]).FontSize(tinyFont).Bold().SingleLine().Center().TextAlign(ui.Center).
		Size(kindBadgeSize, kindBadgeSize).Radius(4).Background(bg).TextColor(fg).Shrink(0).Tooltip(res.Symbol.String())
}

func (v *View) resultDetail(c *ui.Context, res everywhere.Result, sel bool) {
	t := c.Theme()
	color := mutedColor(t, sel)
	switch res.Kind {
	case everywhere.KindFile:
		if res.Detail != "" {
			ui.RichText(c, highlightSpans(t, res.Detail, res.DetailPositions, sel)...).FontSize(smallFont).TextColor(color).SingleLine().TruncateStart().Shrink(1).MinWidth(0)
		}
	case everywhere.KindSymbol:
		text := res.Detail + ":" + itoa(res.Line+1)
		if res.Container != "" {
			text = res.Container + "  " + text
		}
		ui.Text(c, text).FontSize(smallFont).TextColor(color).SingleLine().TruncateStart().Shrink(1).MinWidth(0)
	case everywhere.KindGitRef, everywhere.KindGitCommit:
		ui.Text(c, res.Detail).FontSize(smallFont).TextColor(color).SingleLine().Shrink(1).MinWidth(0)
	}
}

func (v *View) recentView(lr ui.ListRow, r row) {
	c := lr.Context
	sel := lr.Selected()
	el := v.box(lr, r)
	el.Children(func() {
		ui.Text(c, "↺").FontSize(smallFont).TextColor(mutedColor(c.Theme(), sel)).Shrink(0)
		ui.Text(c, firstLine(r.query)).FontSize(bodyFont).SingleLine().Grow(1).MinWidth(0)
	})
	if el.RowClicked() {
		v.useQuery(r.query)
	}
}

func firstLine(s string) string {
	for i, ch := range s {
		if ch == '\n' {
			return s[:i] + " …"
		}
	}
	return s
}

func (v *View) gotoView(lr ui.ListRow, r row) {
	c := lr.Context
	sel := lr.Selected()
	path := v.relPath(v.h.ActivePath())
	el := v.box(lr, r)
	el.Children(func() {
		ui.Text(c, r.label).FontSize(bodyFont).SingleLine().Shrink(0)
		if path != "" {
			mutedText(c, "in "+path, sel).SingleLine().TruncateStart().Shrink(1).MinWidth(0)
		}
	})
}

func (v *View) rowMenu(c *ui.Context, m *ui.Menu, r row) {
	path := r.path()
	if r.kind == rowResult && r.res.Kind != everywhere.KindFile && r.res.Kind != everywhere.KindSymbol {
		if m.Item("Copy").Chosen() {
			c.WriteClipboard(r.res.Label)
		}
		return
	}
	if m.Item("Open").Chosen() {
		v.activateRow(v.indexOfKey(r.key), OpenOptions{Focus: true})
	}
	if m.Item("Open to the Side").Chosen() {
		v.activateRow(v.indexOfKey(r.key), OpenOptions{Side: true, Focus: true})
	}
	m.Separator()
	if m.Item("Copy").Chosen() {
		c.WriteClipboard(v.rowText(r))
	}
	if m.Item("Copy Path").Disabled(path == "").Chosen() {
		c.WriteClipboard(path)
	}
	if m.Item("Copy All").Disabled(v.scope != ScopeText || len(v.text.files) == 0).Chosen() {
		c.WriteClipboard(v.allText())
	}
	if r.canDismiss() {
		m.Separator()
		if m.Item("Dismiss").Chosen() {
			v.dismiss(r)
		}
	}
	if v.scope == ScopeText && v.st.ShowReplace && (r.kind == rowMatch || r.kind == rowFile) {
		if m.Item("Replace").Disabled(!v.canReplace()).Chosen() {
			if r.kind == rowMatch {
				v.replaceMatch(r.file, r.match)
			} else {
				v.replaceFile(r.file)
			}
		}
	}
	if v.env.Reveal != nil && path != "" {
		m.Separator()
		if m.Item("Reveal in Explorer").Chosen() {
			v.env.Reveal(path)
		}
	}
}
