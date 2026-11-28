package github

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

var (
	ErrNoGH        = errors.New("github: gh CLI not found")
	ErrNotLoggedIn = errors.New("github: gh is not logged in")
)

func Token(ctx context.Context, host string) (string, error) {
	if override.token != "" {
		return override.token, nil
	}
	if host == "" || strings.HasPrefix(host, "-") {
		return "", fmt.Errorf("github: invalid host %q", host)
	}
	out, err := runCommand(ctx, "", ghBinary, "auth", "token", "--hostname", host)
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", ErrNoGH
		}
		return "", fmt.Errorf("%w for %s: %v", ErrNotLoggedIn, host, err)
	}
	token := strings.TrimSpace(string(out))
	if token == "" {
		return "", fmt.Errorf("%w for %s: empty token", ErrNotLoggedIn, host)
	}
	return token, nil
}
