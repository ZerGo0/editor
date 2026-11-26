package notices

import (
	"slices"
	"testing"
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
	for range Limit + 3 {
		s.Add(Notice{Title: "n"})
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
