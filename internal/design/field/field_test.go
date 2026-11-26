package field

import (
	"testing"

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
