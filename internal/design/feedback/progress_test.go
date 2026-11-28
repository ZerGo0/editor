package feedback

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestTaskShowsLabelAndCancels(t *testing.T) {
	cancelled := false
	tt := ui.NewTester(func(c *ui.Context) {
		Task(c, "Fetch: Receiving objects 45%", TaskOptions{Value: 0.4, Cancel: "Cancel Fetch", OnCancel: func() { cancelled = true }})
	}, 300, 100)
	if !tt.HasText("Fetch: Receiving objects 45%") {
		t.Fatalf("texts %q", tt.Texts())
	}
	if err := tt.Click("Cancel Fetch"); err != nil || !cancelled {
		t.Fatalf("Cancel Fetch: %v %v", err, cancelled)
	}
}

func TestTaskWithoutCancelHasNoButton(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) { Task(c, "Commit...", TaskOptions{Value: Indeterminate}) }, 300, 100)
	if _, ok := tt.Access("Cancel"); ok {
		t.Fatal("a cancel button shows without a handler")
	}
}
