package picker

import (
	"strconv"

	"github.com/ZerGo0/editor/internal/design/badge"
	"github.com/ZerGo0/editor/internal/design/list"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

const (
	none        = -1
	noResults   = "No results"
	oneResult   = "1 result"
	manyResults = " results"
)

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
	status   string
}

type ResultsOptions struct {
	Label string
	Rows  int
	Fill  bool
	Empty func(c *ui.Context)
	Input *ui.Handle
	Busy  bool

	OnSelect func(i int)
}

func (s *ResultsState) selectable(i int) bool {
	return i >= 0 && i < len(s.rows) && s.rows[i].Kind != RowHeader
}

func (s *ResultsState) SelectFirst(rows []Row) {
	s.rows = rows
	s.Selected = none
	s.Clamp(rows)
}

func (s *ResultsState) Reset(rows []Row) {
	s.SelectFirst(rows)
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
	l := ui.List(c, &s.list, len(rows)).FillWidth().Label(o.Label).Background(ui.Transparent).KeepFocus()
	if o.Fill {
		l.Grow(1).Basis(0).Shrink(1).MinHeight(token.PaneMinHeight.Px())
	} else {
		l.Height(token.RowHeight.Px() * float32(n)).Shrink(0)
	}
	s.height = l.Bounds().H
	places, count := setPlaces(rows)
	var active ui.Element
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
		if o.OnSelect != nil {
			it.OnClick = func() { s.Selected = i; o.OnSelect(i) }
			it.OnDoubleClick = func() {
				if r.Context.Modifiers() == 0 {
					s.Selected, clicked = i, i
				}
			}
		}
		selected := i == s.Selected
		el := list.Row(r.Context, list.State{Selected: selected, Focused: true, Hovered: r.Hovered()}, it).
			Role(ui.RoleOption).Label(rowLabel(rows, i)).PositionInSet(places[i], count).AccessSelected(selected)
		if selected {
			active = el
		}
	})
	if s.scroll && s.Selected != none {
		s.list.ScrollIntoView(s.Selected)
		s.scroll = false
	}
	if input := resolveInput(c, o.Input); input.Valid() && active.Valid() {
		input.ActiveDescendant(active)
	}
	if !o.Busy {
		s.status = resultCount(items(rows))
	}
	ui.Box(c.Key("results-status")).Absolute().Size(token.Space0.Px(), token.Space0.Px()).Role(ui.RoleStatus).Label(s.status)
	if len(rows) == 0 && o.Empty != nil {
		l.Children(func() { o.Empty(c) })
	}
	return clicked
}

func setPlaces(rows []Row) (places []int, count int) {
	places = make([]int, len(rows))
	for i, r := range rows {
		if r.Kind != RowHeader {
			count++
			places[i] = count
		}
	}
	return places, count
}

func items(rows []Row) int {
	n := 0
	for _, r := range rows {
		if r.Kind == RowItem {
			n++
		}
	}
	return n
}

func resultCount(n int) string {
	switch n {
	case 0:
		return noResults
	case 1:
		return oneResult
	}
	return strconv.Itoa(n) + manyResults
}

func rowLabel(rows []Row, i int) string {
	if i < 0 || i >= len(rows) {
		return ""
	}
	if rows[i].Kind == RowItem {
		return list.AccessName(rows[i].Item)
	}
	return rows[i].Title
}

func header(c *ui.Context, p *token.Palette, row Row) {
	ui.Row(c).Height(token.RowHeight.Px()).FillWidth().Padding(0, token.PadRow.Px()).AlignItems(ui.Center).Background(p.Layer.QuickInput).Role(ui.RoleHeading).Label(row.Title).Children(func() {
		token.TextTitle.Apply(ui.Text(c, token.TextTitle.Format(row.Title))).TextColor(p.Text.Secondary).SingleLine().AccessHidden(true)
	})
}

func resolveInput(c *ui.Context, h *ui.Handle) ui.Element {
	if h == nil {
		return ui.Element{}
	}
	return c.Resolve(*h)
}
