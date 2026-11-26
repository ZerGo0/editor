package dialog

import (
	"testing"

	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/egoist/mygo/ui"
)

const (
	testWidth  = 800
	testHeight = 600
)

func TestModalOpensAndCloses(t *testing.T) {
	open := true
	submitted := false
	tt := ui.NewTester(func(c *ui.Context) {
		Modal(c, &open, "Submit Review", Options{Hint: "Send your verdict."}, func() {
			Actions(c, func() {
				action.Button(c, "Cancel", action.ButtonOptions{})
				if action.Button(c, "Submit", action.ButtonOptions{Variant: action.Primary}).Clicked() {
					submitted = true
				}
			})
		})
	}, testWidth, testHeight)
	if _, ok := tt.Access("Submit Review"); !ok {
		t.Fatal("dialog is not labelled with its title")
	}
	if err := tt.Click("Submit"); err != nil || !submitted {
		t.Fatalf("Submit click: %v, submitted %v", err, submitted)
	}
	tt.Key(0, ui.KeyEscape)
	if open {
		t.Fatal("Escape does not close the dialog")
	}
}
