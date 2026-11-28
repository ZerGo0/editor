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

type Action struct {
	Label string
	Run   func()
}

type Notice struct {
	ID      string
	Tone    token.Tone
	Title   string
	Detail  string
	Raw     string
	Actions []Action
	Busy    bool
	Sticky  bool
	At      time.Time
}

func (n Notice) sameAs(o Notice) bool {
	return !n.Busy && !o.Busy && n.Tone == o.Tone && n.Title == o.Title && n.Detail == o.Detail && n.Raw == o.Raw
}

type Store struct {
	mu      sync.Mutex
	list    []Notice
	pending []Notice
	closing []string
	unread  map[string]bool
	next    int
}

func (s *Store) Add(n Notice) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n.ID == "" {
		if i := slices.IndexFunc(s.list, n.sameAs); i >= 0 {
			n.ID = s.list[i].ID
		} else {
			s.next++
			n.ID = idPrefix + strconv.Itoa(s.next)
		}
	}
	if n.At.IsZero() {
		n.At = time.Now()
	}
	s.remove(n.ID)
	s.list = append(s.list, n)
	if over := len(s.list) - Limit; over > 0 {
		for _, o := range s.list[:over] {
			delete(s.unread, o.ID)
		}
		s.list = slices.Delete(s.list, 0, over)
	}
	s.pending = append(s.pending, n)
	if s.unread == nil {
		s.unread = map[string]bool{}
	}
	s.unread[n.ID] = true
	return n.ID
}

func (s *Store) Close(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.remove(id)
	delete(s.unread, id)
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
	s.list, s.pending, s.unread = nil, nil, nil
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
	return len(s.unread)
}

func (s *Store) MarkRead() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.unread = nil
}
