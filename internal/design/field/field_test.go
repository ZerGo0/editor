package field

import (
	"math"
	"testing"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const (
	testWidth  = 400
	testHeight = 200
)

func TestTextInputError(t *testing.T) {
	value := "main.go/"
	tt := ui.NewTester(func(c *ui.Context) {
		TextInput(c, "File name", &value, InputOptions{Error: "Name cannot end with a slash."})
	}, testWidth, testHeight)
	info, ok := tt.AccessByRole(ui.RoleTextField, "File name")
	if !ok || !info.Invalid {
		t.Fatalf("input = %+v, want an invalid text field", info)
	}
	if _, ok := tt.AccessByRole(ui.RoleAlert, "Name cannot end with a slash."); !ok {
		t.Fatal("error message is not announced")
	}
}

func TestSearchInputEscapeClears(t *testing.T) {
	query := "design"
	tt := ui.NewTester(func(c *ui.Context) {
		SearchInput(c, "Search", &query, SearchOptions{AutoFocus: true})
	}, testWidth, testHeight)
	if _, ok := tt.Access("Clear Search"); !ok {
		t.Fatal("search input has no clear button")
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if query != "" {
		t.Fatalf("query = %q after Escape, want empty", query)
	}
}

func TestTextAreaFontSize(t *testing.T) {
	const bigFont = 16
	var small, big string
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Children(func() {
			TextArea(c, "Small", &small, AreaOptions{})
			TextArea(c, "Big", &big, AreaOptions{FontSize: bigFont})
		})
	}, testWidth, testHeight)
	s, okSmall := tt.AccessByRole(ui.RoleTextField, "Small")
	b, okBig := tt.AccessByRole(ui.RoleTextField, "Big")
	if !okSmall || !okBig || b.Bounds.H <= s.Bounds.H {
		t.Fatalf("text area heights: default %v, %d px font %v; want the larger font to give a taller line", s.Bounds.H, bigFont, b.Bounds.H)
	}
}

func TestToggleMeetsHitTarget(t *testing.T) {
	var q string
	on := false
	tt := ui.NewTester(func(c *ui.Context) {
		TextInput(c, "Find", &q, InputOptions{Toggles: []Toggle{{Icon: icons.CaseSensitive, Label: "Match Case", On: &on}}})
	}, testWidth, testHeight)
	b, ok := tt.AccessByRole(ui.RoleToggleButton, "Match Case")
	if !ok || b.Bounds.W < token.HitTargetMin.Px() || b.Bounds.H < token.HitTargetMin.Px() {
		t.Fatalf("toggle bounds %v; want at least %v px", b.Bounds, token.HitTargetMin.Px())
	}
}

func TestMultilineTrailingControlsCentreOnFirstLine(t *testing.T) {
	var one, many string
	on := false
	toggles := []Toggle{{Icon: icons.CaseSensitive, Label: "Match Case", On: &on}}
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Children(func() {
			TextInput(c, "One", &one, InputOptions{Toggles: toggles, ID: "one"})
			TextArea(c, "Many", &many, AreaOptions{MinLines: 3, Toggles: toggles, Actions: []Action{{Icon: icons.Close, Label: "Clear"}}, ID: "many"})
		})
	}, testWidth, testHeight)
	var single, multi, clear ui.AccessInfo
	for _, a := range tt.AccessAll() {
		switch {
		case a.Name == "Match Case" && single.Name == "":
			single = a
		case a.Name == "Match Case":
			multi = a
		case a.Name == "Clear":
			clear = a
		}
	}
	oneIn, okOne := tt.AccessByRole(ui.RoleTextField, "One")
	manyIn, okMany := tt.AccessByRole(ui.RoleTextField, "Many")
	if !okOne || !okMany || multi.Name == "" || clear.Name == "" {
		t.Fatal("a field, toggle or action is missing")
	}
	want := single.Bounds.Y + single.Bounds.H/2 - oneIn.Bounds.Y
	for _, b := range []ui.Rect{multi.Bounds, clear.Bounds} {
		if got := b.Y + b.H/2 - manyIn.Bounds.Y; got != want {
			t.Fatalf("multi-line control centre at +%v; the first line centre is at +%v", got, want)
		}
	}
	if y := manyIn.Bounds.Y + token.PadInputY.Px(); y != float32(math.Round(float64(y))) {
		t.Fatalf("first line top %v is not on a whole px", y)
	}
}

func TestTrailingActionRunsUnlessDisabled(t *testing.T) {
	var q string
	runs := 0
	disabled := true
	tt := ui.NewTester(func(c *ui.Context) {
		TextInput(c, "Message", &q, InputOptions{Actions: []Action{{Icon: icons.Close, Label: "Send", Disabled: disabled, Run: func() { runs++ }}}})
	}, testWidth, testHeight)
	tt.Click("Send")
	if runs != 0 {
		t.Fatal("a disabled action ran")
	}
	disabled = false
	tt.Advance(0)
	tt.Click("Send")
	if runs != 1 {
		t.Fatalf("action ran %d times; want 1", runs)
	}
}
