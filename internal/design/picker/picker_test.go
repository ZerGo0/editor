package picker

import (
	"strings"
	"testing"

	"github.com/ZerGo0/editor/internal/design/list"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	testWidth  = 900
	testHeight = 700
	dialogName = "Search Everywhere"
)

type harness struct {
	tab     int
	query   string
	results ResultsState
	rows    []Row
	keys    Keys
	clicked int
	closed  bool
	next    bool
	prev    bool
}

func newHarness() *harness {
	h := &harness{clicked: none}
	h.rows = []Row{
		{Kind: RowHeader, Title: "Files"},
		{Item: list.Item{Label: "a.go"}},
		{Item: list.Item{Label: "b.go"}},
		{Kind: RowMore, Title: "More files (3)"},
		{Kind: RowHeader, Title: "Actions"},
		{Item: list.Item{Label: "Save"}, Keys: "⌘S"},
	}
	return h
}

func (h *harness) view(c *ui.Context) {
	if Dialog(c, DialogOptions{Label: dialogName, ID: "test"}, func() {
		Header(c, func() { Tabs(c, "Scope", &h.tab, []string{"All", "Files"}) }, nil)
		h.keys = Query(c, dialogName, &h.query, QueryOptions{Placeholder: "Type to search"})
		switch {
		case h.keys.Down:
			h.results.Move(1)
		case h.keys.Up:
			h.results.Move(-1)
		}
		h.next, h.prev = h.next || h.keys.Next, h.prev || h.keys.Previous
		if i := Results(c, &h.results, h.rows, ResultsOptions{Label: "Results"}); i != none {
			h.clicked = i
		}
	}) {
		h.closed = true
	}
}

func TestResultsSkipHeadersAndKeepFocusInQuery(t *testing.T) {
	h := newHarness()
	tt := ui.NewTester(h.view, testWidth, testHeight)
	tt.Frame()
	if h.results.Selected != 1 {
		t.Fatalf("selected %d, want the first item (1)", h.results.Selected)
	}
	tt.Key(0, ui.KeyDown)
	tt.Key(0, ui.KeyDown)
	tt.Key(0, ui.KeyDown)
	if h.results.Selected != 5 {
		t.Fatalf("selected %d after three Down keys, want 5 (headers are skipped)", h.results.Selected)
	}
	tt.Key(0, ui.KeyDown)
	if h.results.Selected != 1 {
		t.Fatalf("selected %d after wrapping, want 1", h.results.Selected)
	}
	tt.Key(0, ui.KeyUp)
	if h.results.Selected != 5 {
		t.Fatalf("selected %d after Up from the first item, want 5", h.results.Selected)
	}
	tt.Type("ab")
	if h.query != "ab" {
		t.Fatalf("query %q: typing did not reach the query field", h.query)
	}
}

func TestRowClickAndKeys(t *testing.T) {
	h := newHarness()
	tt := ui.NewTester(h.view, testWidth, testHeight)
	tt.Click("b.go")
	if h.clicked != 2 {
		t.Fatalf("clicked %d, want 2", h.clicked)
	}
	for _, a := range tt.AccessAll() {
		if a.Name == "More files (3)" {
			return
		}
	}
	t.Fatal("the more row is not shown")
}

func TestTabAndShiftTabReachTheDialog(t *testing.T) {
	h := newHarness()
	tt := ui.NewTester(h.view, testWidth, testHeight)
	tt.Frame()
	tt.Key(0, ui.KeyTab)
	if !h.next {
		t.Fatal("Tab did not reach the query handler")
	}
	tt.Key(ui.Shift, ui.KeyTab)
	if !h.prev {
		t.Fatal("Shift+Tab did not reach the query handler")
	}
}

func TestTabsSelectOnClick(t *testing.T) {
	h := newHarness()
	tt := ui.NewTester(h.view, testWidth, testHeight)
	tt.Click("Files")
	if h.tab != 1 {
		t.Fatalf("tab %d, want 1", h.tab)
	}
	if _, ok := tt.AccessByRole(ui.RoleTab, "All"); !ok {
		t.Fatal("tabs have no tab role")
	}
}

func TestEscapeClosesDialog(t *testing.T) {
	h := newHarness()
	tt := ui.NewTester(h.view, testWidth, testHeight)
	if _, ok := tt.AccessByRole(ui.RoleDialog, dialogName); !ok {
		t.Fatal("dialog has no dialog role")
	}
	tt.Key(0, ui.KeyEscape)
	if !h.closed {
		t.Fatal("Escape does not close the dialog")
	}
}

func TestEmptyResults(t *testing.T) {
	var s ResultsState
	s.Selected = 4
	tt := ui.NewTester(func(c *ui.Context) {
		Results(c, &s, nil, ResultsOptions{Label: "Results", Empty: func(c *ui.Context) { ui.Text(c, "Nothing found") }})
	}, testWidth, testHeight)
	if _, ok := tt.Find("Nothing found"); !ok {
		t.Fatal("empty results show no message")
	}
	if s.Selected != none {
		t.Fatalf("selected %d on empty results", s.Selected)
	}
}

func TestPreviewShowsEditorMessageAndBusyStates(t *testing.T) {
	st := &editor.State{Buffer: editor.NewBuffer("package main\n"), ReadOnly: true}
	var shown *editor.State
	message, busy := "Select a file to preview it", false
	tt := ui.NewTester(func(c *ui.Context) { Preview(c, shown, PreviewOptions{Label: "Preview", Message: message, Busy: busy}) }, testWidth, testHeight)
	if !tt.HasText(message) {
		t.Fatal("an empty preview shows no message")
	}
	box, ok := tt.AccessByRole(ui.RoleGroup, "Preview")
	if !ok || box.Bounds.H > 220 {
		t.Fatalf("preview box %+v", box)
	}
	shown = st
	tt.Frame()
	found := false
	for _, a := range tt.AccessAll() {
		found = found || strings.Contains(a.Value, "package main")
	}
	if !found {
		t.Fatal("the preview does not hold its text")
	}
	shown, busy, message = nil, true, "Loading preview..."
	tt.Frame()
	if !tt.HasText(message) {
		t.Fatal("a busy preview shows no loading text")
	}
}

func TestDialogClosesWhenTheWindowDeactivates(t *testing.T) {
	h := newHarness()
	tt := ui.NewTester(h.view, testWidth, testHeight)
	tt.Frame()
	tt.SetWindowActive(false)
	tt.Frame()
	if !h.closed {
		t.Fatal("the dialog stays open in an inactive window")
	}
}

func TestDialogGivesTheFocusBackToTheOpener(t *testing.T) {
	const fieldLabel = "Notes"
	var text string
	h := newHarness()
	open := false
	tt := ui.NewTester(func(c *ui.Context) {
		ui.TextInputBase(c.Key("notes"), &text).Label(fieldLabel)
		if open {
			h.view(c)
			if h.closed {
				open = false
			}
		}
	}, testWidth, testHeight)
	tt.Frame()
	notes, _ := tt.Access(fieldLabel)
	tt.ClickAt(notes.Bounds.X+notes.Bounds.W/2, notes.Bounds.Y+notes.Bounds.H/2)
	tt.Frame()
	open = true
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	tt.Frame()
	if !tt.Focused(fieldLabel) {
		t.Fatal("Escape did not give the focus back to the field that had it")
	}
}
