package ghshared_test

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"testing"

	"github.com/ZerGo0/editor/internal/ghshared"
	"github.com/ZerGo0/editor/internal/github"
)

func TestErrorText(t *testing.T) {
	offline := fmt.Errorf("prui: GET http://127.0.0.1:1/user: %w", &url.Error{Op: "Get", URL: "http://127.0.0.1:1/user",
		Err: &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connect: connection refused")}})
	cases := map[string]error{
		"GitHub rate limit reached. Try again later.":                                      &github.RateLimitError{},
		"GitHub cannot find it. Check that it exists and that your account can access it.": &github.APIError{Status: 404, Message: "Not Found"},
		"GitHub is not responding right now. Try again later.":                             &github.APIError{Status: 502, Message: "Bad Gateway"},
		"Cannot reach GitHub. Check your connection.":                                      offline,
		"Plain.": errors.New("github: plain"),
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
