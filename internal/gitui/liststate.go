package gitui

import "github.com/egoist/mygo/ui"

type listState[T any] struct {
	open     bool
	loading  bool
	err      error
	gen      int
	items    []T
	list     ui.ListState
	selected int
}

func openList[T any]() listState[T] { return listState[T]{open: true, selected: -1} }

func (s *listState[T]) begin() int {
	s.gen++
	s.loading, s.err = true, nil
	return s.gen
}

func (s *listState[T]) apply(gen int, items []T, err error) bool {
	if gen != s.gen {
		return false
	}
	s.loading, s.err, s.items = false, err, items
	return true
}

func (s *listState[T]) clampSelection() {
	if s.selected >= len(s.items) || (s.selected < 0 && len(s.items) > 0) {
		s.selected = min(max(s.selected, 0), len(s.items)-1)
	}
}
