package pane

import (
	"math"

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
	sizes := s.openSizes(stack.Bounds().H, panes)
	stack.Children(func() {
		prev := ""
		for i, pn := range panes {
			open := !s.Collapsed[pn.ID]
			col := ui.Column(c.Key(pn.ID)).FillWidth().MinHeight(0).Shrink(0)
			if open {
				h, grow := sizes[pn.ID].basis, sizes[pn.ID].grow
				col.Grow(grow).Basis(h).Shrink(1).MinHeight(token.PaneMinHeight.Px())
			}
			s.heights[pn.ID] = col.Bounds().H
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

type paneSize struct {
	basis float32
	grow  float32
}

func (s *StackState) openSizes(total float32, panes []Pane) map[string]paneSize {
	sizes := map[string]paneSize{}
	var open []string
	var weights, used float32
	for _, pn := range panes {
		if s.Collapsed[pn.ID] {
			used += s.heights[pn.ID]
			continue
		}
		open = append(open, pn.ID)
		weights += s.weight(pn.ID)
	}
	free := float32(math.Floor(float64(total - used)))
	if len(open) == 0 || free <= 0 {
		for _, id := range open {
			sizes[id] = paneSize{grow: s.weight(id)}
		}
		return sizes
	}
	for i, id := range open {
		if i == len(open)-1 {
			sizes[id] = paneSize{basis: free, grow: 1}
			break
		}
		h := float32(math.Floor(float64(free * s.weight(id) / weights)))
		sizes[id] = paneSize{basis: h}
		free -= h
		weights -= s.weight(id)
	}
	return sizes
}

func (s *StackState) weight(id string) float32 {
	if w, ok := s.Weights[id]; ok && w > 0 {
		return w
	}
	return defaultWeight
}

func (s *StackState) sash(c *ui.Context, above, below string, panes []Pane) {
	var r SashResult
	sashAnchor(c.Key("sash"), true, -sashOffset).Children(func() {
		var sash ui.Element
		sash, r = Sash(c, SashOptions{Label: "Resize " + titleOf(panes, above), Rows: true})
		sash.Top(0).Left(0).Right(0).Bottom(0)
	})
	switch {
	case r.Reset:
		clear(s.Weights)
		c.Invalidate()
	case r.Delta != 0:
		s.resize(above, below, r.Delta, panes)
		c.Invalidate()
	}
}

func titleOf(panes []Pane, id string) string {
	for _, pn := range panes {
		if pn.ID == id {
			return pn.Title
		}
	}
	return id
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
