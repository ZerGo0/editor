package list

import (
	"github.com/egoist/mygo/ui"
)

type ViewOptions struct {
	Label string
	Empty func(c *ui.Context)
	Tree  bool
}

func View(c *ui.Context, s *ui.ListState, n int, o ViewOptions, item func(i int) Item) ui.Element {
	l := ui.List(c, s, n).Grow(1).FillWidth().MinHeight(0).Label(o.Label).Background(ui.Transparent)
	l.Rows(func(r ui.ListRow) {
		it := item(r.Index)
		Row(r.Context, State{Selected: r.Selected(), Focused: r.ListFocused(), Hovered: r.Hovered(), Depth: it.Depth, Branch: it.Branch, Open: it.Open, flat: o.Tree}, it)
	})
	if n == 0 && o.Empty != nil {
		l.Children(func() { o.Empty(c) })
	}
	return l
}

func Tree[K comparable](c *ui.Context, s *ui.OutlineState[K], roots []K, children func(K) []K, o ViewOptions, item func(K) Item) ui.Element {
	box := ui.Column(c).Grow(1).FillWidth().MinHeight(0)
	guides := box.Hovered()
	focused := s.List.FocusWithin(c)
	selected := selection(s)
	var outline ui.Element
	box.Children(func() {
		var cur ui.OutlineRow
		s.Prefix = func(r ui.OutlineRow) { cur = r }
		outline = ui.Outline(c, s, roots, children, func(k K) {
			st := State{Selected: selected(k), Focused: focused, Depth: cur.Depth, Branch: cur.Branch, Open: cur.Open,
				Guides: guides || focused, arrow: cur.Arrow, tree: true}
			Row(c, st, item(k))
		}).Grow(1).FillWidth().MinHeight(0).Label(o.Label).Background(ui.Transparent)
		if len(roots) == 0 && o.Empty != nil {
			outline.Children(func() { o.Empty(c) })
		}
	})
	return outline
}

func selection[K comparable](s *ui.OutlineState[K]) func(K) bool {
	if many, ok := s.List.Selection.(*ui.Selection[K]); ok && many != nil {
		return many.Has
	}
	if s.List.Selected == nil {
		return func(K) bool { return false }
	}
	row := *s.List.Selected
	if row < 0 || row >= s.Rows() {
		return func(K) bool { return false }
	}
	chosen := s.Item(row)
	return func(k K) bool { return k == chosen }
}
