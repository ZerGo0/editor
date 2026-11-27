package ghshared_test

import (
	"errors"
	"testing"
	"time"

	"github.com/ZerGo0/editor/internal/ghshared"
	"github.com/ZerGo0/editor/internal/github"
)

func TestErrorText(t *testing.T) {
	reset := time.Date(2026, 1, 1, 15, 4, 0, 0, time.Local)
	cases := map[string]error{
		"GitHub rate limit reached. Try again after 15:04.": &github.RateLimitError{Reset: reset},
		"GitHub rate limit reached. Try again later.":       &github.RateLimitError{},
		"GitHub answered 404: Not Found":                    &github.APIError{Status: 404, Message: "Not Found"},
		"plain":                                             errors.New("github: plain"),
	}
	for want, err := range cases {
		if got := ghshared.ErrorText(err); got != want {
			t.Errorf("%q, want %q", got, want)
		}
	}
}

func TestIsUnauthorized(t *testing.T) {
	if !ghshared.IsUnauthorized(&github.APIError{Status: 401}) {
		t.Error("a 401 answer must count as unauthorized")
	}
	if ghshared.IsUnauthorized(&github.APIError{Status: 403}) || ghshared.IsUnauthorized(errors.New("boom")) || ghshared.IsUnauthorized(nil) {
		t.Error("only a 401 answer counts as unauthorized")
	}
}
