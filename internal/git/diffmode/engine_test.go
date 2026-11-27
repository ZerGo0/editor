package diffmode

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	testDelay   = 20 * time.Millisecond
	testTimeout = 5 * time.Second
	indentedOld = "func f() {\n\tx := 1\n\treturn x\n}\n\nfunc g() {\n\treturn 2\n}\n"
	indentedNew = "func f() {\n    x := 1\n    return x\n}\n\nfunc g() {\n\treturn 3\n}\n"
)

type harness struct {
	t     *testing.T
	posts chan func()
	e     *Engine
	d     *editor.DiffState
}

func newHarness(t *testing.T, differ Differ) *harness {
	t.Helper()
	saved := Session
	t.Cleanup(func() { Session = saved })
	Session = Options{ShowMoves: true}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	h := &harness{t: t, posts: make(chan func(), 64), d: &editor.DiffState{}}
	h.e = NewEngine(ctx, h.d, func(fn func()) { h.posts <- fn })
	h.e.Delay = testDelay
	if differ != nil {
		h.e.Differ = differ
	}
	t.Cleanup(h.e.Close)
	return h
}

func (h *harness) until(done func() bool) {
	h.t.Helper()
	deadline := time.After(testTimeout)
	for {
		h.e.Refresh()
		if done() {
			return
		}
		select {
		case fn := <-h.posts:
			fn()
		case <-deadline:
			h.t.Fatalf("condition not met; notice %q", h.e.Notice())
		}
	}
}

func TestEnginePlainModeIsUnchanged(t *testing.T) {
	h := newHarness(t, nil)
	h.e.SetSource(Source{Old: indentedOld, New: indentedNew})
	if !h.e.Ready() || h.e.Filtered() || h.d.Old.String() != indentedOld || h.d.New.String() != indentedNew {
		t.Fatalf("state = %v %v", h.e.Ready(), h.e.Filtered())
	}
	if len(h.d.Hunks()) != 2 {
		t.Fatalf("hunks = %+v", h.d.Hunks())
	}
}

func TestEngineIgnoreWhitespaceHidesIndentChange(t *testing.T) {
	if !git.Available() {
		t.Skip("git is not installed")
	}
	h := newHarness(t, nil)
	Session.Whitespace = git.WhitespaceIgnore
	h.e.SetSource(Source{Old: indentedOld, New: indentedNew})
	h.until(h.e.Filtered)
	hunks := h.d.Hunks()
	if len(hunks) != 1 || hunks[0].NewStart != 6 {
		t.Fatalf("hunks = %+v", hunks)
	}
	if h.d.New.String() != indentedNew || h.d.Editable {
		t.Fatalf("new text changed or editable: %q", h.d.New.String())
	}
	Session.Whitespace = git.WhitespaceNone
	h.until(func() bool { return !h.e.Filtered() && len(h.d.Hunks()) == 2 })
	if h.d.Old.String() != indentedOld {
		t.Fatalf("old = %q", h.d.Old.String())
	}
}

func TestEngineFallsBackWithNotice(t *testing.T) {
	h := newHarness(t, func(context.Context, string, string, git.WhitespaceMode) ([]byte, error) {
		return nil, errors.New("git exploded")
	})
	Session.Whitespace = git.WhitespaceIgnore
	h.e.SetSource(Source{Old: indentedOld, New: indentedNew})
	h.until(func() bool { return h.e.Notice() != "" })
	if !strings.Contains(h.e.Notice(), "git exploded") || h.e.Filtered() || h.e.Busy() {
		t.Fatalf("notice %q filtered %v busy %v", h.e.Notice(), h.e.Filtered(), h.e.Busy())
	}
	if len(h.d.Hunks()) != 2 {
		t.Fatalf("plain diff not shown: %+v", h.d.Hunks())
	}
}

func TestEngineDropsStaleResults(t *testing.T) {
	release := make(chan struct{})
	calls := make(chan git.WhitespaceMode, 4)
	h := newHarness(t, func(ctx context.Context, old, new string, m git.WhitespaceMode) ([]byte, error) {
		calls <- m
		if m == git.WhitespaceIgnore {
			<-release
		}
		return nil, nil
	})
	Session.Whitespace = git.WhitespaceIgnore
	h.e.SetSource(Source{Old: "a\n  b\n", New: "a\nb\n"})
	<-calls
	Session.Whitespace = git.WhitespaceTrim
	h.until(func() bool { return h.e.Filtered() })
	close(release)
	select {
	case fn := <-h.posts:
		fn()
	case <-time.After(100 * time.Millisecond):
	}
	if !h.e.Filtered() || h.e.mode != git.WhitespaceTrim {
		t.Fatalf("stale result applied: mode %v", h.e.mode)
	}
}

func TestEngineRecomputesLiveEditsAfterDelay(t *testing.T) {
	var runs atomic.Int32
	h := newHarness(t, func(_ context.Context, old, new string, m git.WhitespaceMode) ([]byte, error) {
		runs.Add(1)
		return nil, nil
	})
	live := editor.NewBuffer("a\nb\n")
	Session.Whitespace = git.WhitespaceIgnore
	h.e.SetSource(Source{Old: "a\n b\n", New: "a\nb\n", Live: live})
	h.until(h.e.Filtered)
	before := runs.Load()
	live.Reset("a\nb\nc\n")
	h.e.Refresh()
	if !h.e.Busy() {
		t.Fatal("an edit did not schedule a recompute")
	}
	h.until(func() bool { return runs.Load() > before && !h.e.Busy() })
}

func TestEngineFindsMovesAndKeepsOtherAnnotations(t *testing.T) {
	head, block, mid, tail := numbered("head", 0, 6), numbered("moved", 0, 4), numbered("mid", 0, 8), numbered("tail", 0, 6)
	h := newHarness(t, nil)
	h.e.SetSource(Source{Old: joinLines(head, block, mid, tail), New: joinLines(head, mid, tail, block)})
	h.until(func() bool { return len(h.e.Moves()) == 1 })
	other := editor.DiffAnnotation{Side: editor.DiffNew, Line: 1, Height: 10, Key: "thread"}
	got := h.e.Annotations([]editor.DiffAnnotation{other})
	if len(got) != 3 || got[0] != other {
		t.Fatalf("annotations = %+v", got)
	}
	Session.ShowMoves = false
	h.e.Refresh()
	if len(h.e.Moves()) != 0 || len(h.e.Annotations(nil)) != 0 {
		t.Fatal("moves stay after they are turned off")
	}
}

func TestEngineSkipsMovesAboveTheLimit(t *testing.T) {
	var oldLines, newLines []string
	for i := 0; i < MaxMoveChangedLines; i++ {
		oldLines = append(oldLines, "old line "+strings.Repeat("x", i%7)+string(rune('a'+i%26)))
		newLines = append(newLines, "new line "+strings.Repeat("y", i%5)+string(rune('a'+i%26)))
	}
	h := newHarness(t, nil)
	h.e.SetSource(Source{Old: strings.Join(oldLines, "\n") + "\n", New: strings.Join(newLines, "\n") + "\n"})
	h.e.Refresh()
	select {
	case <-h.posts:
		t.Fatal("move detection ran above the limit")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestMoveRowShowsAndJumps(t *testing.T) {
	head, block, mid, tail := numbered("head", 0, 6), numbered("moved", 0, 4), numbered("mid", 0, 8), numbered("tail", 0, 6)
	h := newHarness(t, nil)
	h.e.SetSource(Source{Old: joinLines(head, block, mid, tail), New: joinLines(head, mid, tail, block)})
	h.e.Install(nil)
	h.until(func() bool { return len(h.e.Moves()) == 1 })
	tt := ui.NewTester(func(c *ui.Context) {
		h.e.Refresh()
		ui.Row(c).FillWidth().Children(func() { Controls(c, h.e) })
		h.d.Annotations = h.e.Annotations(nil)
		editor.DiffView(c, h.d).Grow(1).FillWidth()
	}, 1000, 700)
	for _, want := range []string{"Moved to line 21", "Moved from line 7", "Do not ignore", "Show Moved Code"} {
		if !tt.HasText(want) {
			t.Fatalf("missing %q in %v", want, tt.Texts())
		}
	}
	if err := tt.Click("Moved to line 21"); err != nil {
		t.Fatal(err)
	}
	sel, ok := h.d.SelectedLines()
	if !ok || sel.Side != editor.DiffNew || sel.Start != 20 || sel.End != 23 {
		t.Fatalf("selection = %+v, %v", sel, ok)
	}
	h.until(func() bool { _, ok := h.d.SelectedLines(); return !ok })
}

func TestWhitespaceMenuChoosesMode(t *testing.T) {
	h := newHarness(t, func(context.Context, string, string, git.WhitespaceMode) ([]byte, error) { return nil, nil })
	h.e.SetSource(Source{Old: "a\n", New: "b\n"})
	tt := ui.NewTester(func(c *ui.Context) { Controls(c, h.e) }, 600, 100)
	if err := tt.Click("Ignore Whitespace"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Ignore whitespaces"); err != nil {
		t.Fatal(err)
	}
	if Session.Whitespace != git.WhitespaceIgnore {
		t.Fatalf("mode = %v", Session.Whitespace)
	}
}

func TestBuildPatchIsAlwaysAccepted(t *testing.T) {
	if !git.Available() {
		t.Skip("git is not installed")
	}
	cases := []struct{ name, old, new string }{
		{"empty old", "", "a\nb\n"},
		{"empty new", "a\nb\n", ""},
		{"both empty", "", ""},
		{"crlf", "a\r\nb\r\nc\n", "a\nb\nC\n"},
		{"no final newline", "a\nb", "a\nb\n"},
		{"final newline removed", "a\n  b\n", "a\nb"},
		{"blank lines moved", "a\n\nb\nc\n", "a\nb\n\nc\nd\n"},
		{"tabs to spaces", indentedOld, indentedNew},
		{"trailing spaces", "x  \ny\t\nz\n", "x\ny\nZ\n"},
	}
	for _, c := range cases {
		for _, m := range []git.WhitespaceMode{git.WhitespaceTrim, git.WhitespaceIgnore, git.WhitespaceIgnoreBlank} {
			res, err := buildPatch(context.Background(), git.DiffTexts, c.old, c.new, m)
			if err != nil {
				t.Errorf("%s mode %d: %v", c.name, m, err)
				continue
			}
			d := &editor.DiffState{}
			if err := d.SetPatchFiles(res.patch, res.old, c.new); err != nil {
				t.Errorf("%s mode %d: %v", c.name, m, err)
			}
		}
	}
}
