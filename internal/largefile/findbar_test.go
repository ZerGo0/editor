package largefile

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestFindBarSearchesAndCloses(t *testing.T) {
	d := openTemp(t, []byte("alpha\nbeta\ngamma\n"))
	waitIndexed(t, d)
	s := &State{Doc: d}
	tt := ui.NewTester(func(c *ui.Context) { View(c, s) }, testViewW, testViewH)
	s.openFind()
	tt.Frame()
	if _, ok := tt.AccessByRole(ui.RoleSearch, "Find"); !ok {
		t.Fatal("find bar is not shown")
	}
	tt.Type("beta")
	tt.Key(0, ui.KeyEnter)
	waitFound(t, tt, s)
	if a, b := s.selection(); a != 6 || b != 10 {
		t.Fatalf("selection %d..%d, want 6..10", a, b)
	}
	if err := tt.Click("Match Case"); err != nil {
		t.Fatal(err)
	}
	if !s.find.matchCase {
		t.Fatal("Match Case did not toggle")
	}
	for _, name := range []string{"Previous Match", "Next Match", "Close"} {
		if _, ok := tt.Access(name); !ok {
			t.Errorf("find bar has no %q button", name)
		}
	}
	if err := tt.Click("Close"); err != nil {
		t.Fatal(err)
	}
	if s.find.open {
		t.Fatal("Close left the find bar open")
	}
}

func TestFindBarMultilineQuery(t *testing.T) {
	d := openTemp(t, []byte("alpha\r\nbeta\r\ngamma\r\n"))
	waitIndexed(t, d)
	s := &State{Doc: d}
	tt := ui.NewTester(func(c *ui.Context) { View(c, s) }, testViewW, testViewH)
	s.openFind()
	tt.Frame()
	tt.Type("beta")
	tt.Frame()
	tt.Key(ui.Ctrl, ui.KeyEnter)
	tt.Frame()
	tt.Type("gamma")
	tt.Frame()
	if s.find.query != "beta\ngamma" {
		t.Fatalf("query %q, want Ctrl+Enter to insert a newline", s.find.query)
	}
	tt.Key(0, ui.KeyEnter)
	waitFound(t, tt, s)
	if a, b := s.selection(); a != 7 || b != 18 {
		t.Fatalf("selection %d..%d, want 7..18 across the CRLF", a, b)
	}
	tt.Key(ui.Shift, ui.KeyEnter)
	waitFound(t, tt, s)
	if a, b := s.selection(); a != 7 || b != 18 {
		t.Fatalf("selection %d..%d after Shift+Enter, want the only match 7..18", a, b)
	}
}

func waitFound(t *testing.T, tt *ui.Tester, s *State) {
	t.Helper()
	for range 200 {
		tt.Frame()
		s.find.mu.Lock()
		pending := s.find.result != nil
		s.find.mu.Unlock()
		if !pending && s.find.status == "" {
			return
		}
	}
	t.Fatal("search did not finish")
}
