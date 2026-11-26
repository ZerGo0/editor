package git

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	stashFormat       = "%H%x1f%ct%x1f%gd%x1f%gs"
	stashFieldSep     = "\x1f"
	stashFieldCount   = 4
	stashRefFormat    = "stash@{%d}"
	nothingToStash    = "No local changes to save"
	untrackedParent   = "^3"
	firstParentSuffix = "^1"
)

var stashRefPattern = regexp.MustCompile("^stash@\\{(\\d+)\\}$")

type Stash struct {
	Index   int
	Ref     string
	Message string
	OID     string
	Time    time.Time
}

func stashRef(index int) (string, error) {
	if index < 0 {
		return "", fmt.Errorf("git stash: invalid index %d", index)
	}
	return fmt.Sprintf(stashRefFormat, index), nil
}

func (r *Repo) Stashes(ctx context.Context) ([]Stash, error) {
	out, err := r.read(ctx, "stash", "list", "-z", "--format="+stashFormat)
	if err != nil {
		return nil, err
	}
	var stashes []Stash
	for _, rec := range splitNUL(out) {
		f := strings.Split(rec, stashFieldSep)
		if len(f) != stashFieldCount {
			return nil, fmt.Errorf("git stash list: malformed record %q", rec)
		}
		m := stashRefPattern.FindStringSubmatch(f[2])
		if m == nil {
			return nil, fmt.Errorf("git stash list: malformed ref %q", f[2])
		}
		index, _ := strconv.Atoi(m[1])
		t, err := parseUnix(f[1])
		if err != nil {
			return nil, err
		}
		stashes = append(stashes, Stash{Index: index, Ref: f[2], Message: f[3], OID: f[0], Time: t})
	}
	return stashes, nil
}

func (r *Repo) StashPush(ctx context.Context, message string, includeUntracked, keepIndex bool, paths ...string) (bool, error) {
	if err := checkPaths(paths); err != nil {
		return false, err
	}
	args := []string{"stash", "push"}
	if includeUntracked {
		args = append(args, "--include-untracked")
	}
	if keepIndex {
		args = append(args, "--keep-index")
	}
	if message != "" {
		args = append(args, "--message", message)
	}
	if len(paths) > 0 {
		args = withPaths(args, paths)
	}
	out, err := r.run(ctx, call{env: []string{"GIT_LITERAL_PATHSPECS=0"}}, args...)
	if err != nil {
		return false, err
	}
	return !strings.Contains(string(out), nothingToStash), nil
}

func (r *Repo) StashApply(ctx context.Context, index int, pop bool) (OpResult, error) {
	ref, err := stashRef(index)
	if err != nil {
		return OpResult{}, err
	}
	verb := "apply"
	if pop {
		verb = "pop"
	}
	out, err := r.write(ctx, "stash", verb, ref)
	return r.finishOp(ctx, out, err)
}

func (r *Repo) StashDrop(ctx context.Context, index int) error {
	ref, err := stashRef(index)
	if err != nil {
		return err
	}
	_, err = r.write(ctx, "stash", "drop", ref)
	return err
}

func (r *Repo) StashShow(ctx context.Context, index int) ([]FileDiff, error) {
	ref, err := stashRef(index)
	if err != nil {
		return nil, err
	}
	diffs, err := r.diff(ctx, []string{ref + firstParentSuffix, ref}, nil)
	if err != nil {
		return nil, err
	}
	if _, err := r.read(ctx, "rev-parse", "--verify", "--quiet", ref+untrackedParent); err != nil {
		if exitCodeIs(err, 1) {
			return diffs, nil
		}
		return nil, err
	}
	empty, err := r.emptyTree(ctx)
	if err != nil {
		return nil, err
	}
	untracked, err := r.diff(ctx, []string{empty, ref + untrackedParent}, nil)
	if err != nil {
		return nil, err
	}
	return append(diffs, untracked...), nil
}
