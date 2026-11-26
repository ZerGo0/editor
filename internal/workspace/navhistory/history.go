package navhistory

import (
	"slices"

	"github.com/ZerGo0/editor/internal/session"
)

const Limit = 50

type History struct {
	back, forward []session.Location
}

func (h *History) Record(from session.Location) {
	if n := len(h.back); n > 0 && h.back[n-1] == from {
		h.forward = nil
		return
	}
	h.back = append(h.back, from)
	if over := len(h.back) - Limit; over > 0 {
		h.back = slices.Delete(h.back, 0, over)
	}
	h.forward = nil
}

func (h *History) CanBack() bool { return len(h.back) > 0 }

func (h *History) CanForward() bool { return len(h.forward) > 0 }

func (h *History) Back(current session.Location, steps int) (session.Location, bool) {
	return step(&h.back, &h.forward, current, steps)
}

func (h *History) Forward(current session.Location, steps int) (session.Location, bool) {
	return step(&h.forward, &h.back, current, steps)
}

func step(from, to *[]session.Location, current session.Location, steps int) (session.Location, bool) {
	if steps < 1 || steps > len(*from) {
		return session.Location{}, false
	}
	cut := len(*from) - steps
	target := (*from)[cut]
	*to = append(*to, current)
	for i := len(*from) - 1; i > cut; i-- {
		*to = append(*to, (*from)[i])
	}
	*from = (*from)[:cut]
	return target, true
}

func (h *History) BackList() []session.Location { return newestFirst(h.back) }

func (h *History) ForwardList() []session.Location { return newestFirst(h.forward) }

func newestFirst(s []session.Location) []session.Location {
	out := slices.Clone(s)
	slices.Reverse(out)
	return out
}

func (h *History) Snapshot() (back, forward []session.Location) {
	return slices.Clone(h.back), slices.Clone(h.forward)
}

func (h *History) Restore(back, forward []session.Location) {
	h.back = slices.Clone(back[max(len(back)-Limit, 0):])
	h.forward = slices.Clone(forward[max(len(forward)-Limit, 0):])
}

func (h *History) Map(fn func(session.Location) (session.Location, bool)) {
	h.back = mapLocations(h.back, fn)
	h.forward = mapLocations(h.forward, fn)
}

func mapLocations(s []session.Location, fn func(session.Location) (session.Location, bool)) []session.Location {
	out := s[:0]
	for _, l := range s {
		if m, ok := fn(l); ok {
			out = append(out, m)
		}
	}
	return out
}
