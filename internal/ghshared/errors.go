package ghshared

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ZerGo0/editor/internal/github"
)

const unauthorizedStatus = 401

func ErrorText(err error) string {
	var limit *github.RateLimitError
	var api *github.APIError
	switch {
	case errors.As(err, &limit):
		if limit.Reset.IsZero() {
			return "GitHub rate limit reached. Try again later."
		}
		return "GitHub rate limit reached. Try again after " + limit.Reset.Format("15:04") + "."
	case errors.As(err, &api):
		return fmt.Sprintf("GitHub answered %d: %s", api.Status, api.Message)
	}
	return strings.TrimPrefix(err.Error(), "github: ")
}

func IsUnauthorized(err error) bool {
	var api *github.APIError
	return errors.As(err, &api) && api.Status == unauthorizedStatus
}
