package findwidget

import (
	"encoding/json"
	"slices"
	"strconv"
	"testing"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	widthDrag = 40
	splitText = "Cat cat\ncat"
)

func sharedHarness(t *testing.T, sh *Shared) *harness {
	t.Helper()
	st := &editor.State{Buffer: editor.NewBuffer(splitText)}
	st.RenderFindBar = ForEditor(st, sh)
	return &harness{t: t, st: st, tt: editorTester(st)}
}

func (h *harness) replaceQuery(text string) {
	h.key(ui.Cmd, ui.KeyA)
	h.tt.Type(text)
	h.tt.Frame()
}

func TestOptionsAreSharedAcrossEditors(t *testing.T) {
	sh := &Shared{}
	a, b := &editor.State{Buffer: editor.NewBuffer(splitText)}, &editor.State{Buffer: editor.NewBuffer(splitText)}
	a.RenderFindBar, b.RenderFindBar = ForEditor(a, sh), ForEditor(b, sh)
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Row(c).Fill().Children(func() {
			editor.View(c, a).Grow(1).Basis(0)
			editor.View(c, b).Grow(1).Basis(0)
		})
	}, testW, testH)
	tt.Frame()
	a.Run(editor.Find)
	tt.Frame()
	tt.Type("cat")
	tt.Frame()
	tt.Key(ToggleMods, ui.KeyC)
	tt.Frame()
	tt.Frame()
	if !sh.MatchCase {
		t.Fatal("Match Case in one editor did not reach the shared options")
	}
	if n, _ := a.FindMatches(); n != 2 {
		t.Fatalf("%d matches in the first editor with Match Case, want 2", n)
	}
	b.Find("cat")
	tt.Frame()
	tt.Frame()
	if n, _ := b.FindMatches(); n != 2 {
		t.Fatalf("%d matches in the second editor, want 2: Match Case is not shared", n)
	}
	sh.MatchCase = false
	tt.Frame()
	if n, _ := b.FindMatches(); n != 3 {
		t.Fatalf("%d matches after turning Match Case off, want 3", n)
	}
}

func TestQueryHistoryBrowsesWithUpAndDown(t *testing.T) {
	sh := &Shared{}
	h := sharedHarness(t, sh)
	h.open(editor.Find)
	for _, q := range []string{"one", "two"} {
		h.replaceQuery(q)
		h.key(0, ui.KeyEnter)
	}
	h.replaceQuery("dra")
	for i, step := range []struct {
		key  ui.Key
		want string
	}{{ui.KeyUp, "two"}, {ui.KeyUp, "one"}, {ui.KeyUp, "one"}, {ui.KeyDown, "two"}, {ui.KeyDown, "dra"}} {
		h.key(0, step.key)
		h.tt.Frame()
		if got := h.field(findLabel).Value; got != step.want {
			t.Fatalf("step %d: query %q, want %q", i, got, step.want)
		}
	}
	if got := sh.History.Entries(); !slices.Equal(got, []string{"one", "two"}) {
		t.Fatalf("history %q, want the submitted queries", got)
	}
}

func TestEscapeRemembersTheQuery(t *testing.T) {
	sh := &Shared{}
	h := sharedHarness(t, sh)
	h.open(editor.Find)
	h.tt.Type("cat")
	h.tt.Frame()
	h.key(0, ui.KeyEscape)
	if _, ok := h.tt.AccessByRole(ui.RoleTextField, findLabel); ok {
		t.Fatal("Escape left the bar open")
	}
	if got := sh.History.Entries(); !slices.Equal(got, []string{"cat"}) {
		t.Fatalf("history %q after Escape, want the query", got)
	}
}

func TestLeftEdgeResizesTheWidget(t *testing.T) {
	sh := &Shared{}
	h := sharedHarness(t, sh)
	h.open(editor.Find)
	before := h.field(findLabel).Bounds.W
	info, ok := h.tt.AccessByRole(ui.RoleSplitter, resizeLabel)
	if !ok {
		t.Fatal("the find widget has no resize handle")
	}
	x, y := info.Bounds.X+info.Bounds.W/2, info.Bounds.Y+info.Bounds.H/2
	h.tt.Press(x, y)
	h.tt.Frame()
	h.tt.Move(x-widthDrag, y)
	h.tt.Frame()
	h.tt.Release(x-widthDrag, y)
	h.tt.Frame()
	h.tt.Frame()
	if want := int(token.FindInputWidth.Px()) + widthDrag; sh.Width != want {
		t.Fatalf("width %d after dragging the left edge, want %d", sh.Width, want)
	}
	if got := h.field(findLabel).Bounds.W; got != before+widthDrag {
		t.Fatalf("query field %v wide, want %v", got, before+widthDrag)
	}
	h.key(0, ui.KeyEnter)
	if sh.Width != 0 {
		t.Fatalf("width %d after reset, want the default", sh.Width)
	}
}

func TestHistoryIsBoundedAndDeduplicated(t *testing.T) {
	var h History
	for i := range historyLimit + 5 {
		h.Add("q" + strconv.Itoa(i))
	}
	h.Add("q10")
	h.Add("  ")
	got := h.Entries()
	if len(got) != historyLimit || got[len(got)-1] != "q10" || slices.Index(got, "q10") != len(got)-1 || got[0] != "q5" {
		t.Fatalf("history %q, want %d entries, oldest first, q10 moved to the end", got, historyLimit)
	}
	data, err := json.Marshal(Shared{MatchCase: true, History: h})
	if err != nil {
		t.Fatal(err)
	}
	var back Shared
	if err := json.Unmarshal(data, &back); err != nil || !back.MatchCase || !slices.Equal(back.History.Entries(), got) {
		t.Fatalf("round trip %+v (%v)", back, err)
	}
	if data, _ := json.Marshal(Shared{}); string(data) != "{}" {
		t.Fatalf("empty state saved as %s", data)
	}
}
