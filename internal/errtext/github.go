package errtext

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/ZerGo0/editor/internal/github"
	"github.com/ZerGo0/editor/internal/plural"
)

func gitHub(err error) (string, bool) {
	var limit *github.RateLimitError
	var api *github.APIError
	switch {
	case errors.As(err, &limit):
		return rateLimitText(limit.Reset, time.Now()), true
	case errors.As(err, &api):
		return apiText(api), true
	}
	return "", false
}

func rateLimitText(reset, now time.Time) string {
	const base = "GitHub rate limit reached. "
	if reset.IsZero() {
		return base + "Try again later."
	}
	minutes := int((reset.Sub(now) + time.Minute - 1) / time.Minute)
	if minutes <= 1 {
		return base + "Try again in a minute."
	}
	return base + "Try again in " + plural.Count(minutes, "minute", "minutes") + "."
}

func apiText(api *github.APIError) string {
	switch {
	case api.Status == http.StatusUnauthorized:
		return "GitHub did not accept your sign-in. Sign in again with gh auth login."
	case api.Status == http.StatusForbidden:
		return "GitHub denied access. Check that your account can access this repository."
	case api.Status == http.StatusNotFound:
		return "GitHub cannot find it. Check that it exists and that your account can access it."
	case api.Status >= http.StatusInternalServerError:
		return "GitHub is not responding right now. Try again later."
	case strings.TrimSpace(api.Message) != "":
		return "GitHub rejected the request: " + sentence(api.Message)
	}
	return "GitHub rejected the request."
}
