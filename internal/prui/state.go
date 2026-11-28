package prui

import "errors"

var errOtherRepo = errors.New("This pull request belongs to another repository.")

const sideStateKey = "prui.side"

type sideMemo struct {
	Open   map[string]bool `json:"open,omitempty"`
	Search string          `json:"search,omitempty"`
}

func (s *sideState) everySection() []*section {
	return append([]*section{s.current, s.changes}, s.sections()...)
}

func (s *sideState) memo() sideMemo {
	m := sideMemo{Open: map[string]bool{}}
	for _, sec := range s.everySection() {
		m.Open[sec.id] = sec.open
	}
	if s.search != nil {
		m.Search = s.search.query
	}
	return m
}

func (s *sideState) restore(m sideMemo) {
	if m.Search != "" {
		s.field, s.search = m.Search, newSearchSection(m.Search)
	}
	for _, sec := range s.everySection() {
		if open, ok := m.Open[sec.id]; ok {
			sec.open = open
		}
	}
}

func (p *PRs) sideMemo() sideMemo {
	if p.sideRestored {
		p.saved = p.side.memo()
	}
	return p.saved
}
