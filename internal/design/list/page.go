package list

import (
	"runtime"

	"github.com/egoist/mygo/ui"
)

type pageStep struct {
	key ui.Key
	dir int
}

var pageSteps = [...]pageStep{{ui.KeyPageDown, 1}, {ui.KeyPageUp, -1}}

func pageKeys(c *ui.Context, s *ui.ListState, n int, only func(row int)) {
	if runtime.GOOS != "darwin" || s.Selected == nil || n == 0 {
		return
	}
	for _, step := range pageSteps {
		move := s.Shortcut(c, 0, step.key)
		extend := s.Selection != nil && s.Shortcut(c, ui.Shift, step.key)
		if !move && !extend {
			continue
		}
		first, last := s.Visible()
		to := pageTarget(*s.Selected, first, last, n, step.dir)
		if extend {
			s.ExtendTo(to)
		} else {
			s.Choose(to)
			if s.Selection != nil {
				only(to)
			}
		}
		s.ScrollIntoView(to)
		c.Invalidate()
	}
}

func pageTarget(cur, first, last, n, dir int) int {
	span := max(1, last-first)
	to := cur
	switch {
	case cur < 0 || cur >= n:
		to = max(first, 0)
	case dir > 0 && cur < last:
		to = last
	case dir > 0:
		to = cur + span
	case cur > first:
		to = first
	default:
		to = cur - span
	}
	return max(0, min(to, n-1))
}

func selectOnly[K comparable](sel ui.Selector, key any) bool {
	many, ok := sel.(*ui.Selection[K])
	k, isKey := key.(K)
	if !ok || many == nil || !isKey {
		return false
	}
	many.Clear()
	many.Add(k)
	return true
}
