package ghshared

import (
	"errors"

	"github.com/ZerGo0/editor/internal/errtext"
	"github.com/ZerGo0/editor/internal/github"
)

const (
	unauthorizedStatus = 401
	offlineText        = "Cannot reach GitHub. Check your connection."
)

func ErrorText(err error) string {
	if errtext.IsNetwork(err) {
		return offlineText
	}
	return errtext.Text(err)
}

func IsUnauthorized(err error) bool {
	var api *github.APIError
	return errors.As(err, &api) && api.Status == unauthorizedStatus
}
