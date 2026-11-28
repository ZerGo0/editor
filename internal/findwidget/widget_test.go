package findwidget

import (
	"strings"
	"testing"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	testW        = 900
	testH        = 600
	maxGrowLines = 8
	maxTestLines = 11
)

type harness struct {
	t  *testing.T
	st *editor.State
	tt *ui.Tester
}

func editorTester(st *editor.State) *ui.Tester {
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() { editor.View(c, st).Grow(1) })
	}, testW, testH)
	tt.Frame()
	return tt
}

func newHarness(t *testing.T, text string, readOnly bool) *harness {
	t.Helper()
	st := &editor.State{Buffer: editor.NewBuffer(text), ReadOnly: readOnly}
	st.RenderFindBar = ForEditor(st, nil)
	return &harness{t: t, st: st, tt: editorTester(st)}
}

func (h *harness) open(cmd editor.Command) {
	h.t.Helper()
	h.st.Run(cmd)
	h.tt.Frame()
	h.tt.Frame()
	if !h.tt.Focused(findLabel) {
		h.t.Fatalf("the query field does not have the focus after opening the bar: %q", h.tt.Texts())
	}
}

func (h *harness) key(mods ui.Modifiers, key ui.Key) {
	h.tt.Key(mods, key)
	h.tt.Frame()
}

func (h *harness) typeLines(lines ...string) {
	for i, l := range lines {
		if i > 0 {
			h.key(ui.Ctrl, ui.KeyEnter)
		}
		h.tt.Type(l)
		h.tt.Frame()
	}
}

func (h *harness) field(label string) ui.AccessInfo {
	h.t.Helper()
	info, ok := h.tt.AccessByRole(ui.RoleTextField, label)
	if !ok {
		h.t.Fatalf("no %q field: %q", label, h.tt.Texts())
	}
	return info
}

func (h *harness) focusReplace() {
	h.t.Helper()
	b := h.field(replaceLabel).Bounds
	h.tt.ClickAt(b.X+b.W/2, b.Y+b.H/2)
	h.tt.Frame()
	if !h.tt.Focused(replaceLabel) {
		h.t.Fatal("the replace field does not take the focus")
	}
}

func (h *harness) matches() (int, int) {
	h.tt.Frame()
	return h.st.FindMatches()
}

func TestQueryKeys(t *testing.T) {
	h := newHarness(t, "cat dog\ncat\ncat", false)
	h.open(editor.Find)
	h.tt.Type("cat")
	h.tt.Frame()
	if n, cur := h.matches(); n != 3 || cur != 0 {
		t.Fatalf("matches %d, current %d after typing, want 3 and 0", n, cur)
	}
	h.key(0, ui.KeyEnter)
	if _, cur := h.matches(); cur != 1 {
		t.Fatalf("current %d after Enter, want 1", cur)
	}
	h.key(0, ui.KeyEnter)
	h.key(ui.Shift, ui.KeyEnter)
	if _, cur := h.matches(); cur != 1 {
		t.Fatalf("current %d after Enter and Shift+Enter, want 1", cur)
	}
	if got := h.field(findLabel).Value; got != "cat" {
		t.Fatalf("Enter changed the query to %q", got)
	}
	h.key(ui.Alt, ui.KeyEnter)
	if n := len(h.st.Selections()); n != 3 {
		t.Fatalf("%d selections after Alt+Enter, want every match", n)
	}
	if _, ok := h.tt.AccessByRole(ui.RoleTextField, findLabel); ok {
		t.Fatal("Alt+Enter left the bar open")
	}
}

func TestMultilineQueryFindsAcrossLines(t *testing.T) {
	h := newHarness(t, "one\ntwo\none\nthree\none\ntwo\n", false)
	h.open(editor.Find)
	h.typeLines("one", "two")
	if got := h.field(findLabel).Value; got != "one\ntwo" {
		t.Fatalf("query %q, want Ctrl+Enter to insert a newline", got)
	}
	if n, _ := h.matches(); n != 2 {
		t.Fatalf("%d matches of a two-line query, want 2", n)
	}
	if err := h.tt.Click(useRegexp.Label); err != nil {
		t.Fatal(err)
	}
	h.key(ui.Cmd, ui.KeyA)
	h.tt.Type("one\\ntwo")
	h.tt.Frame()
	if n, _ := h.matches(); n != 2 {
		t.Fatalf("%d matches of the regular expression one\\ntwo, want 2", n)
	}
}

func TestPasteKeepsNewlines(t *testing.T) {
	h := newHarness(t, "alpha\nbeta\nalpha\nbeta", false)
	h.open(editor.Find)
	h.tt.SetClipboard("alpha\nbeta")
	h.key(ui.Cmd, ui.KeyV)
	if got := h.field(findLabel).Value; got != "alpha\nbeta" {
		t.Fatalf("pasted query %q, want the newline kept", got)
	}
	if n, _ := h.matches(); n != 2 {
		t.Fatalf("%d matches, want 2", n)
	}
}

func TestReplaceKeys(t *testing.T) {
	h := newHarness(t, "one\ntwo\none\ntwo\none\ntwo", false)
	h.open(editor.FindReplace)
	h.typeLines("one", "two")
	h.focusReplace()
	h.typeLines("x", "y")
	if got := h.field(replaceLabel).Value; got != "x\ny" {
		t.Fatalf("replacement %q, want Ctrl+Enter to insert a newline", got)
	}
	h.key(0, ui.KeyEnter)
	if got := h.st.Buffer.String(); got != "x\ny\none\ntwo\none\ntwo" {
		t.Fatalf("text after Enter %q, want the first match replaced", got)
	}
	h.key(ui.Cmd, ui.KeyEnter)
	if got := h.st.Buffer.String(); got != "x\ny\nx\ny\nx\ny" {
		t.Fatalf("text after Cmd+Enter %q, want every match replaced", got)
	}
}

func TestFieldGrowsUpToMaxLines(t *testing.T) {
	h := newHarness(t, "text", false)
	h.open(editor.Find)
	heights := map[int]float32{}
	lines := []string{"a"}
	h.typeLines("a")
	for n := 1; n <= maxTestLines; n++ {
		if n > 1 {
			h.typeLines("", "a")
			lines = append(lines, "a")
		}
		heights[n] = h.field(findLabel).Bounds.H
		bar, _ := h.tt.AccessByRole(ui.RoleToolbar, findLabel)
		if b := bar.Bounds; b.X != float32(int(b.X)) || b.Y != float32(int(b.Y)) || b.W != float32(int(b.W)) || b.H != float32(int(b.H)) {
			t.Errorf("bar at %d lines spans %+v, want whole pixels", n, b)
		}
	}
	if got := h.field(findLabel).Value; got != strings.Join(lines, "\n") {
		t.Fatalf("query %q", got)
	}
	for n := 2; n <= maxGrowLines; n++ {
		if heights[n] <= heights[n-1] {
			t.Fatalf("field %v high at %d lines, %v at %d: it does not grow", heights[n], n, heights[n-1], n-1)
		}
	}
	for n := maxGrowLines + 1; n <= maxTestLines; n++ {
		if heights[n] != heights[maxGrowLines] {
			t.Fatalf("field %v high at %d lines, want %v as at %d lines", heights[n], n, heights[maxGrowLines], maxGrowLines)
		}
	}
	for n, hgt := range heights {
		if hgt != float32(int(hgt)) {
			t.Errorf("field is %v high at %d lines, want whole pixels", hgt, n)
		}
	}
}

func TestEscapeClosesAndFocusesEditor(t *testing.T) {
	h := newHarness(t, "alpha", false)
	h.open(editor.Find)
	h.key(0, ui.KeyEscape)
	h.tt.Frame()
	if _, ok := h.tt.AccessByRole(ui.RoleTextField, findLabel); ok {
		t.Fatal("Escape left the bar open")
	}
	if !h.st.Focused() {
		t.Fatal("Escape did not give the focus back to the editor")
	}
}

func TestInvalidRegexp(t *testing.T) {
	h := newHarness(t, "a(b", false)
	h.open(editor.Find)
	if err := h.tt.Click(useRegexp.Label); err != nil {
		t.Fatal(err)
	}
	h.tt.Type("(")
	h.tt.Frame()
	h.tt.Frame()
	if !h.field(findLabel).Invalid {
		t.Fatal("the query field is not invalid")
	}
	if _, ok := h.tt.AccessByRole(ui.RoleAlert, invalidRegexp+": missing closing )"); !ok {
		t.Fatalf("no error message: %q", h.tt.Texts())
	}
	if !h.tt.HasText(noResults) {
		t.Fatalf("status does not say %q: %q", noResults, h.tt.Texts())
	}
}

func TestStatusAndDisabledButtons(t *testing.T) {
	h := newHarness(t, "\nalpha beta alpha", false)
	h.open(editor.FindReplace)
	if !h.tt.HasText(noResults) {
		t.Fatalf("empty query: status %q, want %q", h.tt.Texts(), noResults)
	}
	for _, b := range []Button{PreviousMatch, NextMatch, ReplaceOne, ReplaceAll} {
		if info, ok := h.tt.AccessByRole(ui.RoleButton, b.Label); !ok || !info.Disabled {
			t.Errorf("%s = %+v without matches, want a disabled button", b.Label, info)
		}
	}
	h.tt.Type("alpha")
	h.tt.Frame()
	h.tt.Frame()
	if !h.tt.HasText("1 of 2") {
		t.Fatalf("status %q, want 1 of 2", h.tt.Texts())
	}
	for _, b := range []Button{PreviousMatch, NextMatch, ReplaceOne, ReplaceAll} {
		info, ok := h.tt.AccessByRole(ui.RoleButton, b.Label)
		if !ok || info.Disabled {
			t.Errorf("%s = %+v with matches, want an enabled button", b.Label, info)
		}
		if want := b.Label + " (" + b.Keys + ")"; info.Description != want {
			t.Errorf("%s tooltip %q, want %q", b.Label, info.Description, want)
		}
	}
	for _, b := range []Button{ReplaceOne, ReplaceAll} {
		info, _ := h.tt.AccessByRole(ui.RoleButton, b.Label)
		if info.Bounds.W != token.IconButtonSize.Px() {
			t.Errorf("%s is %v wide, want an icon-only button of %v", b.Label, info.Bounds.W, token.IconButtonSize.Px())
		}
	}
	h.tt.Type("zzz")
	h.tt.Frame()
	h.tt.Frame()
	if !h.tt.HasText(noResults) {
		t.Fatalf("no match: status %q, want %q", h.tt.Texts(), noResults)
	}
}

func TestReadOnlyDisablesReplace(t *testing.T) {
	h := newHarness(t, "alpha", true)
	h.open(editor.FindReplace)
	info, ok := h.tt.AccessByRole(ui.RoleDisclosure, toggleReplaceLabel)
	if !ok || !info.Disabled {
		t.Fatalf("toggle replace = %+v in a read-only editor, want disabled", info)
	}
	if _, ok := h.tt.AccessByRole(ui.RoleTextField, replaceLabel); ok {
		t.Fatal("a read-only editor shows the replace field")
	}
}

func TestToggleReplace(t *testing.T) {
	h := newHarness(t, "alpha", false)
	h.open(editor.Find)
	if err := h.tt.Click(toggleReplaceLabel); err != nil {
		t.Fatal(err)
	}
	h.tt.Frame()
	h.field(replaceLabel)
	if info, _ := h.tt.AccessByRole(ui.RoleDisclosure, toggleReplaceLabel); !info.Expanded {
		t.Fatal("toggle replace does not report it is expanded")
	}
}

func TestSeeding(t *testing.T) {
	h := newHarness(t, "alpha beta\ngamma\nalpha", false)
	h.st.Select(0, len("alpha"))
	h.open(editor.Find)
	if got := h.field(findLabel).Value; got != "alpha" {
		t.Fatalf("query %q from a single-line selection, want alpha", got)
	}
	if info, _ := h.tt.Access(FindInSelection.Label); info.Checked {
		t.Fatal("a single-line selection turned on Find in Selection")
	}
	h.key(0, ui.KeyEscape)

	m := newHarness(t, "alpha beta\ngamma\nalpha", false)
	m.st.Select(0, len("alpha beta\ngamma"))
	m.open(editor.Find)
	if info, _ := m.tt.Access(FindInSelection.Label); !info.Checked {
		t.Fatal("a multi-line selection did not turn on Find in Selection")
	}
	m.tt.Type("alpha")
	m.tt.Frame()
	if n, _ := m.matches(); n != 1 {
		t.Fatalf("%d matches in the selection, want 1", n)
	}
}

func TestToggleShortcuts(t *testing.T) {
	h := newHarness(t, "alpha", false)
	h.open(editor.Find)
	for _, tc := range []struct {
		key   ui.Key
		label string
	}{{ui.KeyC, matchCase.Label}, {ui.KeyW, wholeWord.Label}, {ui.KeyR, useRegexp.Label}, {ui.KeyL, FindInSelection.Label}} {
		h.key(ui.Alt|ui.Cmd, tc.key)
		if info, _ := h.tt.Access(tc.label); !info.Checked {
			t.Errorf("Alt+Cmd+%v did not turn on %s", tc.key, tc.label)
		}
	}
}
