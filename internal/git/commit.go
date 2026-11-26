package git

import (
	"context"
	"errors"
	"strings"
)

var ErrEmptyMessage = errors.New("git: empty commit message")

type CommitOptions struct {
	Message    string
	Amend      bool
	SignOff    bool
	AllowEmpty bool
	NoVerify   bool
	Author     string
	Paths      []string
}

func (r *Repo) Commit(ctx context.Context, o CommitOptions) (string, error) {
	if strings.TrimSpace(o.Message) == "" && !o.Amend {
		return "", ErrEmptyMessage
	}
	if err := checkPaths(o.Paths); err != nil {
		return "", err
	}
	args := []string{"commit"}
	var c call
	if strings.TrimSpace(o.Message) == "" {
		args = append(args, "--no-edit")
	} else {
		args = append(args, "-F", "-")
		c.stdin = strings.NewReader(o.Message)
	}
	if o.Amend {
		args = append(args, "--amend")
	}
	if o.SignOff {
		args = append(args, "--signoff")
	}
	if o.AllowEmpty {
		args = append(args, "--allow-empty")
	}
	if o.NoVerify {
		args = append(args, "--no-verify")
	}
	if o.Author != "" {
		args = append(args, "--author="+o.Author)
	}
	if len(o.Paths) > 0 {
		args = withPaths(append(args, "--only"), o.Paths)
	}
	if _, err := r.run(ctx, c, args...); err != nil {
		return "", err
	}
	return r.readText(ctx, "rev-parse", "HEAD")
}

func (r *Repo) LastCommitMessage(ctx context.Context) (string, error) {
	out, err := r.readText(ctx, "log", "-1", "--no-show-signature", "--encoding=UTF-8", "--format=%B")
	if err != nil {
		var ge *Error
		if errors.As(err, &ge) && strings.Contains(ge.Stderr, noCommitsMarker) {
			return "", ErrNotFound
		}
		return "", err
	}
	return strings.TrimRight(out, "\n"), nil
}
