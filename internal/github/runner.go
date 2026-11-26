package github

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

const ghBinary = "gh"

type Runner func(ctx context.Context, dir, name string, args ...string) ([]byte, error)

var runCommand Runner = execRunner

func execRunner(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return out, fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return out, err
	}
	return out, nil
}

func Checkout(ctx context.Context, dir string, number int) error {
	if dir == "" {
		return errors.New("github: checkout: empty directory")
	}
	if err := checkNumber(number); err != nil {
		return err
	}
	if _, err := runCommand(ctx, dir, ghBinary, "pr", "checkout", strconv.Itoa(number)); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return ErrNoGH
		}
		return fmt.Errorf("github: checkout pull request %d: %w", number, err)
	}
	return nil
}
