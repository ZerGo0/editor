package pane

import (
	"math"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

const fullPercent float32 = 100

type Snap int

const (
	SnapNone Snap = iota
	SnapFirst
	SnapSecond
)

type SplitOptions struct {
	Label     string
	Vertical  bool
	Percent   bool
	FirstMin  token.Size
	SecondMin token.Size
	Reset     float32
	Snap      Snap
	Hidden    *bool
	Divider   bool
}

type SplitState struct {
	want     float32
	dragging bool
	total    float32
}

func (s *SplitState) Total() float32 { return s.total }

func Split(c *ui.Context, s *SplitState, size *float32, o SplitOptions, first, second func()) ui.Element {
	box := ui.Row(c)
	if o.Vertical {
		box = ui.Column(c)
	}
	box.AlignItems(ui.Stretch).MinWidth(0).MinHeight(0)
	b := box.Bounds()
	s.total = b.W
	if o.Vertical {
		s.total = b.H
	}
	if o.Divider {
		s.total = max(s.total-token.SeparatorWidth.Px(), 0)
	}
	firstMin, secondMin := o.mins()
	hidden := o.Hidden != nil && *o.Hidden && o.Snap != SnapNone
	sash := func(edge float32) {
		if (!hidden || s.dragging) && s.sash(c, size, o, hidden, edge) {
			c.Invalidate()
		}
	}
	half := token.SashSize.Px() / 2
	box.Children(func() {
		switch {
		case hidden && o.Snap == SnapFirst:
			sash(0)
			splitPane(c, "split-second", o.Vertical).Grow(1).Children(second)
			return
		case hidden:
			splitPane(c, "split-first", o.Vertical).Grow(1).Children(first)
			sash(-token.SashSize.Px())
			return
		}
		a := splitPane(c, "split-first", o.Vertical)
		share := min(max(*size, 0), fullPercent) / fullPercent
		length := *size
		if o.Percent {
			length = float32(math.Round(float64(share * s.total)))
		}
		switch {
		case o.Percent && s.total <= 0:
			a.Grow(share).Basis(0)
		case o.Vertical:
			a.Shrink(1).Height(length)
		default:
			a.Shrink(1).Width(length)
		}
		minLength(a, o.Vertical, firstMin)
		a.Children(first)
		sash(-half)
		if o.Divider {
			divider(c, o.Vertical, s.dragging)
		}
		bb := splitPane(c, "split-second", o.Vertical)
		if o.Percent && s.total <= 0 {
			bb.Grow(1 - share).Basis(0)
		} else {
			bb.Grow(1)
		}
		minLength(bb, o.Vertical, secondMin)
		bb.Children(second)
	})
	return box
}

func minOr(v token.Size) float32 {
	if v.Px() > 0 {
		return v.Px()
	}
	return token.SplitMinPane.Px()
}

func splitPane(c *ui.Context, key string, vertical bool) ui.Element {
	e := ui.Column(c.Key(key)).Shrink(1).Clip()
	if vertical {
		return e.FillWidth()
	}
	return e.FillHeight()
}

func minLength(e ui.Element, vertical bool, v float32) {
	if vertical {
		e.MinHeight(v)
	} else {
		e.MinWidth(v)
	}
}

func divider(c *ui.Context, vertical, active bool) {
	p := token.Of(c)
	line := ui.Box(c.Key("split-divider")).Shrink(0).Background(p.Border.Default)
	if active {
		line.Background(p.Accent.Focus)
	}
	if vertical {
		line.Height(token.SeparatorWidth.Px()).FillWidth()
	} else {
		line.Width(token.SeparatorWidth.Px()).FillHeight()
	}
}

func (s *SplitState) shown(size float32, o SplitOptions, hidden bool) float32 {
	firstMin, secondMin := o.mins()
	switch {
	case hidden && o.Snap == SnapFirst:
		return 0
	case hidden:
		return s.total
	case o.Percent:
		return fitPane(size*s.total/fullPercent, s.total, firstMin, secondMin)
	}
	return fitPane(size, s.total, firstMin, secondMin)
}

func (o SplitOptions) mins() (first, second float32) {
	return minOr(o.FirstMin), minOr(o.SecondMin)
}

func fitPane(v, total, firstMin, secondMin float32) float32 {
	if total <= 0 {
		return v
	}
	return max(min(v, total-secondMin), min(firstMin, total/2))
}

func (s *SplitState) sash(c *ui.Context, size *float32, o SplitOptions, hidden bool, edge float32) bool {
	shown := s.shown(*size, o, hidden)
	var r SashResult
	sashAnchor(c.Key("split-sash"), o.Vertical, -edge).Children(func() {
		var el ui.Element
		el, r = Sash(c, SashOptions{Label: o.Label, Rows: o.Vertical})
		el.Top(0).Left(0).Bottom(0).Right(0)
	})
	switch {
	case r.Reset && o.Reset > 0:
		return s.set(size, o, o.Reset)
	case r.Held && !s.dragging:
		s.dragging, s.want = true, shown
	case !r.Held:
		s.dragging = false
	}
	if r.Delta == 0 || s.total <= 0 {
		return false
	}
	want := shown + r.Delta
	if s.dragging {
		s.want += r.Delta
		want = s.want
	}
	return s.drag(size, o, want)
}

func (s *SplitState) drag(size *float32, o SplitOptions, want float32) bool {
	firstMin, secondMin := o.mins()
	hide := false
	switch o.Snap {
	case SnapFirst:
		hide = want < firstMin/2
	case SnapSecond:
		hide = s.total-want < secondMin/2
	}
	if hide {
		return s.setHidden(o, true)
	}
	v := fitPane(want, s.total, firstMin, secondMin)
	if o.Percent {
		v = v * fullPercent / s.total
	}
	return s.set(size, o, v)
}

func (s *SplitState) set(size *float32, o SplitOptions, v float32) bool {
	changed := s.setHidden(o, false)
	if v != *size {
		*size, changed = v, true
	}
	return changed
}

func (s *SplitState) setHidden(o SplitOptions, hide bool) bool {
	if o.Hidden == nil || o.Snap == SnapNone || *o.Hidden == hide {
		return false
	}
	*o.Hidden = hide
	return true
}
