package largefile

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

const (
	testViewW = 640
	testViewH = 480
)

func TestViewTypingAndCaret(t *testing.T) {
	d := openTemp(t, []byte("hello\nworld\n"))
	waitIndexed(t, d)
	s := &State{Doc: d}
	tt := ui.NewTester(func(c *ui.Context) { View(c, s) }, testViewW, testViewH)
	s.focus.Focus()
	tt.Frame()
	tt.Type("abc")
	if got := string(content(t, d)); got != "abchello\nworld\n" || s.Caret != 3 {
		t.Fatalf("after typing: %q caret %d", got, s.Caret)
	}
	tt.Key(0, ui.KeyDown)
	if s.Caret != 12 {
		t.Fatalf("caret after Down = %d, want 12", s.Caret)
	}
	tt.Key(0, ui.KeyEnd)
	tt.Type("!")
	tt.Key(ui.Shift, ui.KeyHome)
	if a, b := s.selection(); a != 9 || b != 15 {
		t.Fatalf("selection %d..%d, want 9..15", a, b)
	}
	tt.Key(ui.Cmd, ui.KeyC)
	if got := tt.Clipboard(); got != "world!" {
		t.Fatalf("clipboard %q", got)
	}
	tt.Key(0, ui.KeyBackspace)
	tt.Key(0, ui.KeyEnter)
	if got := string(content(t, d)); got != "abchello\n\n\n" {
		t.Fatalf("after delete and Enter: %q", got)
	}
	tt.Key(ui.Cmd, ui.KeyZ)
	tt.Key(ui.Cmd, ui.KeyZ)
	tt.Key(ui.Cmd, ui.KeyUp)
	if got := string(content(t, d)); got != "abchello\nworld!\n" || s.Caret != 0 {
		t.Fatalf("after undo: %q caret %d", got, s.Caret)
	}
	tt.Compose("に", 1)
	if s.compose != "に" {
		t.Fatalf("composition %q", s.compose)
	}
	tt.Type("日")
	if got := string(content(t, d)); got != "日abchello\nworld!\n" || s.compose != "" {
		t.Fatalf("after commit: %q, composition %q", got, s.compose)
	}
	if tt.FramePending() {
		t.Fatal("the view keeps asking for frames while idle")
	}
}
