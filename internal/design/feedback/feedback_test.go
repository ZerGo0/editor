package feedback

import (
	"errors"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestEmptyShowsCodeAndActions(t *testing.T) {
	retried, copied := false, false
	tt := ui.NewTester(func(c *ui.Context) {
		Empty(c, EmptyOptions{Title: "Sign in to GitHub", Message: "Run this command", Code: "gh auth login", Action: "Check Again", OnAction: func() { retried = true },
			Link: "Copy Command", OnLink: func() { copied = true }})
	}, 400, 400)
	if !tt.HasText("gh auth login") {
		t.Fatalf("texts %q", tt.Texts())
	}
	if err := tt.Click("Check Again"); err != nil || !retried {
		t.Fatalf("Check Again: %v %v", err, retried)
	}
	if err := tt.Click("Copy Command"); err != nil || !copied {
		t.Fatalf("Copy Command: %v %v", err, copied)
	}
}

func TestErrorOffersRetryOnlyWithHandler(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) { Error(c, "Cannot load", errors.New("boom"), nil) }, 400, 300)
	if _, ok := tt.Access("Retry"); ok {
		t.Fatal("Retry shown without a handler")
	}
	if !tt.HasText("boom") {
		t.Fatalf("texts %q", tt.Texts())
	}
}
