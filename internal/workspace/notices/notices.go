package notices

import (
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/ZerGo0/editor/internal/design/token"
)

const (
	Limit    = 50
	idPrefix = "notice-"
)

type Notice struct {
	ID       string
	Tone     token.Tone
	Title    string
	Detail   string
	Action   string
	OnAction func()
	Busy     bool
	Sticky   bool
	At       time.Time
}

type Store struct {
	mu      sync.Mutex
	list    []Notice
	pending []Notice
	closing []string
	unread  int
	next    int
}

func (s *Store) Add(n Notice) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n.ID == "" {
		s.next++
		n.ID = idPrefix + strconv.Itoa(s.next)
	}
	if n.At.IsZero() {
		n.At = time.Now()
	}
	s.list = slices.DeleteFunc(s.list, func(o Notice) bool { return o.ID == n.ID })
	s.list = append(s.list, n)
	if over := len(s.list) - Limit; over > 0 {
		s.list = slices.Delete(s.list, 0, over)
	}
	s.pending = append(s.pending, n)
	s.unread++
	return n.ID
}

func (s *Store) Close(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.remove(id)
	s.closing = append(s.closing, id)
}

func (s *Store) remove(id string) {
	s.list = slices.DeleteFunc(s.list, func(n Notice) bool { return n.ID == id })
	s.pending = slices.DeleteFunc(s.pending, func(n Notice) bool { return n.ID == id })
}

func (s *Store) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, n := range s.list {
		s.closing = append(s.closing, n.ID)
	}
	s.list, s.pending, s.unread = nil, nil, 0
}

func (s *Store) Take() (show []Notice, closed []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	show, closed = s.pending, s.closing
	s.pending, s.closing = nil, nil
	return show, closed
}

func (s *Store) List() []Notice {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := slices.Clone(s.list)
	slices.Reverse(out)
	return out
}

func (s *Store) Unread() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.unread
}

func (s *Store) MarkRead() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.unread = 0
}
