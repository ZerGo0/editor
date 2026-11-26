package git

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type Worktree struct {
	Path     string
	HEAD     string
	Branch   string
	Bare     bool
	Detached bool
	Locked   bool
	Prunable bool
}

func (r *Repo) Worktrees(ctx context.Context) ([]Worktree, error) {
	out, err := r.read(ctx, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return nil, err
	}
	return parseWorktrees(out)
}

func parseWorktrees(data []byte) ([]Worktree, error) {
	var trees []Worktree
	var cur *Worktree
	finish := func() {
		if cur != nil {
			trees = append(trees, *cur)
			cur = nil
		}
	}
	for _, rec := range strings.Split(string(data), nulSeparator) {
		if rec == "" {
			finish()
			continue
		}
		key, value, _ := strings.Cut(rec, " ")
		if key == "worktree" {
			finish()
			cur = &Worktree{Path: value}
			continue
		}
		if cur == nil {
			return nil, fmt.Errorf("git worktree list: unexpected record %q", rec)
		}
		switch key {
		case "HEAD":
			cur.HEAD = value
		case "branch":
			cur.Branch = strings.TrimPrefix(value, headRefPrefix)
		case "bare":
			cur.Bare = true
		case "detached":
			cur.Detached = true
		case "locked":
			cur.Locked = true
		case "prunable":
			cur.Prunable = true
		}
	}
	finish()
	return trees, nil
}

func (r *Repo) AddWorktree(ctx context.Context, path, branch string, newBranch bool) error {
	if err := checkLocation(path); err != nil {
		return err
	}
	args := []string{"worktree", "add"}
	switch {
	case newBranch && branch == "":
		return errors.New("git worktree: new branch needs a name")
	case newBranch:
		if err := checkRef(branch); err != nil {
			return err
		}
		args = append(args, "-b", branch, path)
	case branch != "":
		if err := checkRef(branch); err != nil {
			return err
		}
		args = append(args, path, branch)
	default:
		args = append(args, path)
	}
	_, err := r.write(ctx, args...)
	return err
}

func (r *Repo) RemoveWorktree(ctx context.Context, path string, force bool) error {
	if err := checkLocation(path); err != nil {
		return err
	}
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	_, err := r.write(ctx, append(args, "--", path)...)
	return err
}
