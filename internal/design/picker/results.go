package picker

import (
	"github.com/ZerGo0/editor/internal/design/badge"
	"github.com/ZerGo0/editor/internal/design/list"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

const none = -1

type RowKind uint8

const (
	RowItem RowKind = iota
	RowHeader
	RowMore
)

type Row struct {
	Kind  RowKind
	Item  list.Item
	Title string
	Keys  string
}

type ResultsState struct {
	Selected int
	list     ui.ListState
	scroll   bool
	rows     []Row
	height   float32
}

type ResultsOptions struct {
	Label string
	Rows  int
	Fill  bool
	Empty func(c *ui.Context)
}

func (s *ResultsState) selectable(i int) bool {
	return i >= 0 && i < len(s.rows) && s.rows[i].Kind != RowHeader
}

func (s *ResultsState) Reset(rows []Row) {
	s.rows = rows
	s.Selected = none
	s.Clamp(rows)
	s.list.ScrollIntoView(0)
}

func (s *ResultsState) Clamp(rows []Row) {
	s.rows = rows
	if s.selectable(s.Selected) {
		return
	}
	s.Selected = none
	for i := range rows {
		if s.selectable(i) {
			s.Selected = i
			return
		}
	}
}

func (s *ResultsState) Move(delta int) {
	n := len(s.rows)
	if n == 0 || s.Selected == none {
		return
	}
	step := 1
	if delta < 0 {
		step = -1
	}
	at := s.Selected
	for range max(delta*step, 1) {
		for next := 1; next <= n; next++ {
			if i := (at + step*next%n + n) % n; s.selectable(i) {
				at = i
				break
			}
		}
	}
	s.Selected, s.scroll = at, true
}

func (s *ResultsState) Current() (Row, bool) {
	if !s.selectable(s.Selected) {
		return Row{}, false
	}
	return s.rows[s.Selected], true
}

func Results(c *ui.Context, s *ResultsState, rows []Row, o ResultsOptions) int {
	s.Clamp(rows)
	s.list.Header = func(i int) bool { return i >= 0 && i < len(rows) && rows[i].Kind == RowHeader }
	s.list.Label = func(i int) string { return rowLabel(rows, i) }
	p := token.Of(c)
	clicked := none
	n := o.Rows
	if n <= 0 {
		n = token.SearchPopupRows
	}
	l := ui.List(c, &s.list, len(rows)).FillWidth().Label(o.Label).Background(ui.Transparent)
	if o.Fill {
		l.Grow(1).Basis(0).Shrink(1).MinHeight(token.PaneMinHeight.Px())
	} else {
		l.Height(token.RowHeight.Px() * float32(n)).Shrink(0)
	}
	s.height = l.Bounds().H
	l.Rows(func(r ui.ListRow) {
		row := rows[r.Index]
		if row.Kind == RowHeader {
			header(r.Context, p, row)
			return
		}
		it := row.Item
		if row.Kind == RowMore {
			title := row.Title
			it = list.Item{Rich: func(s list.Style) []ui.Span { return []ui.Span{{Text: title, Color: s.Secondary}} }}
		}
		if row.Keys != "" {
			it.Trailing = func(c *ui.Context) { badge.Keys(c, row.Keys) }
		}
		i := r.Index
		it.OnClick = func() { clicked = i }
		list.Row(r.Context, list.State{Selected: i == s.Selected, Focused: true, Hovered: r.Hovered()}, it)
	})
	if s.scroll && s.Selected != none {
		s.list.ScrollIntoView(s.Selected)
		s.scroll = false
	}
	if len(rows) == 0 && o.Empty != nil {
		l.Children(func() { o.Empty(c) })
	}
	return clicked
}

func rowLabel(rows []Row, i int) string {
	if i < 0 || i >= len(rows) {
		return ""
	}
	if rows[i].Kind == RowItem {
		return rows[i].Item.Label
	}
	return rows[i].Title
}

func header(c *ui.Context, p *token.Palette, row Row) {
	ui.Row(c).Height(token.RowHeight.Px()).FillWidth().Padding(0, token.PadRow.Px()).AlignItems(ui.Center).Background(p.Layer.QuickInput).Label(row.Title).Children(func() {
		token.TextTitle.Apply(ui.Text(c, token.TextTitle.Format(row.Title))).TextColor(p.Text.Secondary).SingleLine().AccessHidden(true)
	})
}
