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

func firstLine(s string) string {
	for i, ch := range s {
		if ch == '\n' {
			return s[:i] + " …"
		}
	}
	return s
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
	if v.showsReplace(r) {
		if m.Item("Replace").Disabled(!v.canReplace()).Chosen() {
			v.replaceRow(r)
		}
	}
	if v.env.Reveal != nil && path != "" {
		m.Separator()
		if m.Item("Reveal in Explorer").Chosen() {
			v.env.Reveal(path)
		}
	}
}
