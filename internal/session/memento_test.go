package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

type paneSizes struct {
	Width float32
	Open  bool
}

func TestMementoRoundTripsTrackedAndSetValues(t *testing.T) {
	m := NewMemento(nil)
	width := float32(120)
	m.Track("tracked", func() any { return paneSizes{Width: width, Open: true} })
	m.Set("pushed", []string{"a", "b"})
	width = 240
	back := NewMemento(m.Snapshot())
	var got paneSizes
	if !back.Load("tracked", &got) || got != (paneSizes{Width: 240, Open: true}) {
		t.Fatalf("tracked = %+v", got)
	}
	var list []string
	if !back.Load("pushed", &list) || len(list) != 2 {
		t.Fatalf("pushed = %v", list)
	}
	if back.Load("missing", &got) {
		t.Fatal("Load reported a key that was never saved")
	}
}

func TestMementoKeepsKeysNobodyTracksYet(t *testing.T) {
	m := NewMemento(nil)
	m.Set("later", 7)
	back := NewMemento(m.Snapshot())
	back.Track("other", func() any { return 1 })
	var n int
	if !NewMemento(back.Snapshot()).Load("later", &n) || n != 7 {
		t.Fatalf("an untracked key was lost: %d", n)
	}
}

func TestSaveSkipsTheWriteWhenNothingChanged(t *testing.T) {
	p := filepath.Join(t.TempDir(), "session.json")
	s, _ := Load(p)
	s.AddRecent("/a")
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	old := time.Unix(1, 0)
	if err := os.Chtimes(p, old, old); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(p); err != nil || !info.ModTime().Equal(old) {
		t.Fatalf("an unchanged session was written again: %v %v", info.ModTime(), err)
	}
	s.AddRecent("/b")
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(p); info.ModTime().Equal(old) {
		t.Fatal("a changed session was not written")
	}
}
