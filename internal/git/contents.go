package git

import (
	"context"
	"errors"
	"strings"
)

var missingObjectMarkers = []string{
	"does not exist",
	"exists on disk, but not in",
	"invalid object name",
	"bad revision",
	"unknown revision",
	"Not a valid object name",
	"is in the index, but not at stage",
}

func (r *Repo) Show(ctx context.Context, rev, path string) ([]byte, error) {
	if err := checkPath(path); err != nil {
		return nil, err
	}
	if rev != "" {
		if err := checkRef(rev); err != nil {
			return nil, err
		}
	}
	out, err := r.read(ctx, "show", rev+":"+path)
	if err != nil {
		var ge *Error
		if errors.As(err, &ge) && containsAny(ge.Stderr, missingObjectMarkers) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return out, nil
}

func containsAny(s string, markers []string) bool {
	for _, m := range markers {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}
