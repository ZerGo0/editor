package notices

import (
	"slices"
	"strconv"
	"testing"

	"github.com/ZerGo0/editor/internal/design/token"
)

func TestAddTakeAndList(t *testing.T) {
	var s Store
	a := s.Add(Notice{Title: "a"})
	s.Add(Notice{Title: "b"})
	show, closed := s.Take()
	if len(show) != 2 || len(closed) != 0 {
		t.Fatalf("take = %d shown, %d closed", len(show), len(closed))
	}
	if again, _ := s.Take(); len(again) != 0 {
		t.Fatal("take returns the same notices two times")
	}
	if l := s.List(); l[0].Title != "b" || s.Unread() != 2 {
		t.Fatalf("list = %v, unread %d", l, s.Unread())
	}
	s.Close(a)
	if _, closed := s.Take(); !slices.Equal(closed, []string{a}) || len(s.List()) != 1 {
		t.Fatalf("closed = %v, list %v", closed, s.List())
	}
}

func TestSameIDReplaces(t *testing.T) {
	var s Store
	s.Add(Notice{ID: "save", Title: "Saving", Busy: true})
	s.Add(Notice{ID: "save", Title: "Saved"})
	if l := s.List(); len(l) != 1 || l[0].Title != "Saved" {
		t.Fatalf("list = %v", l)
	}
}

func TestClearAndLimit(t *testing.T) {
	var s Store
	for i := range Limit + 3 {
		s.Add(Notice{Title: strconv.Itoa(i)})
	}
	if n := len(s.List()); n != Limit {
		t.Fatalf("kept %d notices, want %d", n, Limit)
	}
	s.MarkRead()
	s.Clear()
	if _, closed := s.Take(); len(closed) != Limit || len(s.List()) != 0 || s.Unread() != 0 {
		t.Fatalf("clear closed %d, left %d", len(closed), len(s.List()))
	}
}

func TestEqualNoticesMerge(t *testing.T) {
	var s Store
	first := s.Add(Notice{Tone: token.ToneError, Title: "Cannot fetch", Detail: "Offline."})
	for range 11 {
		if id := s.Add(Notice{Tone: token.ToneError, Title: "Cannot fetch", Detail: "Offline."}); id != first {
			t.Fatalf("id %q, want the merged id %q", id, first)
		}
	}
	if n, u := len(s.List()), s.Unread(); n != 1 || u != 1 {
		t.Fatalf("list %d, unread %d, want 1 and 1", n, u)
	}
	if other := s.Add(Notice{Tone: token.ToneWarning, Title: "Cannot fetch", Detail: "Offline."}); other == first {
		t.Fatal("a notice with another tone merged")
	}
}

func TestCloseLowersUnread(t *testing.T) {
	var s Store
	busy := s.Add(Notice{Title: "Saving a.txt...", Busy: true})
	s.Close(busy)
	s.Add(Notice{Title: "Saved a.txt"})
	if u := s.Unread(); u != 1 {
		t.Fatalf("unread %d with one notice", u)
	}
}
