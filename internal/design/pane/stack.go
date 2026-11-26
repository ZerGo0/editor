package pane

import (
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

const defaultWeight float32 = 1

var sashOffset = -token.SashSize.Px() / 2

type Pane struct {
	ID          string
	Title       string
	Count       int
	Actions     func(c *ui.Context)
	ShowActions bool
	Busy        bool
	Body        func(c *ui.Context)
}

type StackState struct {
	Collapsed map[string]bool
	Weights   map[string]float32
	heights   map[string]float32
}

func Stack(c *ui.Context, s *StackState, panes []Pane) ui.Element {
	if s.Collapsed == nil {
		s.Collapsed = map[string]bool{}
	}
	if s.Weights == nil {
		s.Weights = map[string]float32{}
	}
	if s.heights == nil {
		s.heights = map[string]float32{}
	}
	stack := ui.Column(c.Key("pane-stack")).Fill().MinHeight(0)
	stack.Children(func() {
		prev := ""
		for i, pn := range panes {
			open := !s.Collapsed[pn.ID]
			col := ui.Column(c.Key(pn.ID)).FillWidth().MinHeight(0).Shrink(0)
			if open {
				col.Grow(s.weight(pn.ID)).Basis(0).Shrink(1).MinHeight(token.PaneMinHeight.Px())
				s.heights[pn.ID] = col.Bounds().H
			}
			col.Children(func() {
				if open && prev != "" {
					s.sash(c, prev, pn.ID, panes)
				}
				o := SectionOptions{Count: pn.Count, Actions: pn.Actions, ShowActions: pn.ShowActions, Busy: pn.Busy, Border: i > 0}
				Header(c, pn.Title, &open, o)
				if open && pn.Body != nil {
					ui.Column(c.Key("body")).FillWidth().Grow(1).MinHeight(0).Children(func() { pn.Body(c) })
				}
			})
			s.Collapsed[pn.ID] = !open
			if open {
				prev = pn.ID
			}
		}
	})
	return stack
}

func (s *StackState) weight(id string) float32 {
	if w, ok := s.Weights[id]; ok && w > 0 {
		return w
	}
	return defaultWeight
}

func (s *StackState) sash(c *ui.Context, above, below string, panes []Pane) {
	p := token.Of(c)
	sash := ui.Box(c.Key("sash")).Absolute().Top(sashOffset).Left(0).Right(0).Height(token.SashSize.Px()).
		Cursor(ui.CursorResizeRow).ZIndex(token.LayerSticky.Z()).Role(ui.RoleSplitter).Label("Resize " + above)
	if sash.Hovered() || sash.Dragging() {
		sash.Background(p.Accent.Focus)
	}
	if _, dy, ok := sash.Dragged(); ok && dy != 0 {
		s.resize(above, below, dy, panes)
	}
}

func (s *StackState) resize(above, below string, dy float32, panes []Pane) {
	for _, pn := range panes {
		if !s.Collapsed[pn.ID] && s.heights[pn.ID] > 0 {
			s.Weights[pn.ID] = s.heights[pn.ID]
		}
	}
	a, b := s.heights[above], s.heights[below]
	if a <= 0 || b <= 0 {
		return
	}
	dy = max(dy, token.PaneMinHeight.Px()-a)
	dy = min(dy, b-token.PaneMinHeight.Px())
	s.Weights[above], s.Weights[below] = a+dy, b-dy
	s.heights[above], s.heights[below] = a+dy, b-dy
}
