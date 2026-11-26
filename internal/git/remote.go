package git

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

const (
	remoteFetchSuffix = " (fetch)"
	remotePushSuffix  = " (push)"
	upstreamRef       = "@{upstream}"
)

type Remote struct {
	Name     string
	FetchURL string
	PushURL  string
}

type PullOptions struct {
	Rebase   bool
	FFOnly   bool
	Progress func(line string)
}

type PushOptions struct {
	Remote      string
	Branch      string
	SetUpstream bool
	Force       bool
	Tags        bool
	Progress    func(line string)
}

func (r *Repo) Remotes(ctx context.Context) ([]Remote, error) {
	out, err := r.readText(ctx, "remote", "-v")
	if err != nil {
		return nil, err
	}
	return parseRemotes(out)
}

func parseRemotes(text string) ([]Remote, error) {
	var remotes []Remote
	index := make(map[string]int)
	for _, line := range strings.Split(text, "\n") {
		if line == "" {
			continue
		}
		name, rest, ok := strings.Cut(line, "\t")
		if !ok {
			return nil, fmt.Errorf("git remote: malformed line %q", line)
		}
		i, seen := index[name]
		if !seen {
			i = len(remotes)
			index[name] = i
			remotes = append(remotes, Remote{Name: name})
		}
		switch {
		case strings.HasSuffix(rest, remoteFetchSuffix):
			remotes[i].FetchURL = strings.TrimSuffix(rest, remoteFetchSuffix)
		case strings.HasSuffix(rest, remotePushSuffix):
			remotes[i].PushURL = strings.TrimSuffix(rest, remotePushSuffix)
		default:
			return nil, fmt.Errorf("git remote: malformed line %q", line)
		}
	}
	return remotes, nil
}

func progressArgs(progress func(string)) []string {
	if progress == nil {
		return nil
	}
	return []string{progressFlag}
}

func (r *Repo) Fetch(ctx context.Context, remote string, prune bool, progress func(line string)) error {
	args := append([]string{"fetch"}, progressArgs(progress)...)
	if prune {
		args = append(args, "--prune")
	}
	if remote == "" {
		args = append(args, "--all")
	} else {
		if err := checkRef(remote); err != nil {
			return err
		}
		args = append(args, remote)
	}
	_, err := r.run(ctx, call{progress: progress}, args...)
	return err
}

func (r *Repo) Pull(ctx context.Context, o PullOptions) (OpResult, error) {
	if o.Rebase && o.FFOnly {
		return OpResult{}, errors.New("git pull: rebase and fast-forward only are exclusive")
	}
	args := append([]string{"pull", "--no-edit"}, progressArgs(o.Progress)...)
	switch {
	case o.Rebase:
		args = append(args, "--rebase")
	case o.FFOnly:
		args = append(args, "--ff-only")
	default:
		args = append(args, "--no-rebase")
	}
	out, err := r.run(ctx, call{progress: o.Progress}, args...)
	return r.finishOp(ctx, out, err)
}

func (r *Repo) Push(ctx context.Context, o PushOptions) error {
	args := append([]string{"push"}, progressArgs(o.Progress)...)
	if o.SetUpstream {
		args = append(args, "--set-upstream")
	}
	if o.Force {
		args = append(args, "--force-with-lease")
	}
	if o.Tags {
		args = append(args, "--tags")
	}
	branch := o.Branch
	if branch == "" && o.Remote != "" && o.SetUpstream {
		branch = "HEAD"
	}
	if o.Remote != "" {
		if err := checkRef(o.Remote); err != nil {
			return err
		}
		args = append(args, o.Remote)
		if branch != "" {
			if err := checkRef(branch); err != nil {
				return err
			}
			args = append(args, branch)
		}
	}
	_, err := r.run(ctx, call{progress: o.Progress}, args...)
	return err
}

func (r *Repo) hasUpstream(ctx context.Context) (bool, error) {
	_, err := r.read(ctx, "rev-parse", "--verify", "--quiet", upstreamRef)
	if err == nil {
		return true, nil
	}
	if exitCodeIs(err, 1) {
		return false, nil
	}
	return false, err
}

func (r *Repo) OutgoingCommits(ctx context.Context) ([]Commit, error) {
	ok, err := r.hasUpstream(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return r.logRaw(ctx, []string{"HEAD", "--not", "--remotes", "--"})
	}
	return r.logRaw(ctx, []string{upstreamRef + "..HEAD", "--"})
}

func (r *Repo) IncomingCommits(ctx context.Context) ([]Commit, error) {
	ok, err := r.hasUpstream(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNoUpstream
	}
	return r.logRaw(ctx, []string{"HEAD.." + upstreamRef, "--"})
}
