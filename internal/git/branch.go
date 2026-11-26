package git

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	branchFormat = "%(refname)%00%(objectname)%00%(HEAD)%00%(upstream:short)%00%(upstream:track,nobracket)%00%(committerdate:unix)%00%(contents:subject)"
	tagFormat    = "%(refname)%00%(objectname)%00%(*objectname)%00%(creatordate:unix)%00%(contents:subject)"

	branchFieldCount = 7
	tagFieldCount    = 5
	headRefPrefix    = "refs/heads/"
	remoteRefPrefix  = "refs/remotes/"
	tagRefPrefix     = "refs/tags/"
	remoteHeadSuffix = "/HEAD"
	currentMarker    = "*"
	trackGone        = "gone"
	trackAhead       = "ahead "
	trackBehind      = "behind "
)

type Branch struct {
	Name     string
	Remote   bool
	Current  bool
	Upstream string
	Ahead    int
	Behind   int
	Gone     bool
	OID      string
	Subject  string
	Time     time.Time
}

type Tag struct {
	Name      string
	OID       string
	Annotated bool
	Subject   string
	Time      time.Time
}

type BranchComparison struct {
	AOnly []Commit
	BOnly []Commit
}

func (r *Repo) Branches(ctx context.Context) ([]Branch, error) {
	out, err := r.read(ctx, "for-each-ref", "--format="+branchFormat, "refs/heads", "refs/remotes")
	if err != nil {
		return nil, err
	}
	return parseBranches(string(out))
}

func parseBranches(text string) ([]Branch, error) {
	var branches []Branch
	for _, line := range strings.Split(text, "\n") {
		if line == "" {
			continue
		}
		f := strings.Split(line, nulSeparator)
		if len(f) != branchFieldCount {
			return nil, fmt.Errorf("git for-each-ref: malformed branch record %q", line)
		}
		b := Branch{OID: f[1], Current: f[2] == currentMarker, Upstream: f[3], Subject: f[6]}
		switch {
		case strings.HasPrefix(f[0], headRefPrefix):
			b.Name = strings.TrimPrefix(f[0], headRefPrefix)
		case strings.HasPrefix(f[0], remoteRefPrefix):
			b.Name = strings.TrimPrefix(f[0], remoteRefPrefix)
			b.Remote = true
			if strings.HasSuffix(b.Name, remoteHeadSuffix) {
				continue
			}
		default:
			return nil, fmt.Errorf("git for-each-ref: unexpected ref %q", f[0])
		}
		t, err := parseUnix(f[5])
		if err != nil {
			return nil, err
		}
		b.Time = t
		if err := parseTrack(&b, f[4]); err != nil {
			return nil, err
		}
		branches = append(branches, b)
	}
	return branches, nil
}

func parseTrack(b *Branch, track string) error {
	if track == "" {
		return nil
	}
	for _, part := range strings.Split(track, refSeparator) {
		var target *int
		var number string
		switch {
		case part == trackGone:
			b.Gone = true
			continue
		case strings.HasPrefix(part, trackAhead):
			target, number = &b.Ahead, strings.TrimPrefix(part, trackAhead)
		case strings.HasPrefix(part, trackBehind):
			target, number = &b.Behind, strings.TrimPrefix(part, trackBehind)
		default:
			return fmt.Errorf("git for-each-ref: malformed track %q", track)
		}
		n, err := strconv.Atoi(number)
		if err != nil {
			return fmt.Errorf("git for-each-ref: malformed track %q", track)
		}
		*target = n
	}
	return nil
}

func (r *Repo) Tags(ctx context.Context) ([]Tag, error) {
	out, err := r.read(ctx, "for-each-ref", "--format="+tagFormat, "refs/tags")
	if err != nil {
		return nil, err
	}
	return parseTags(string(out))
}

func parseTags(text string) ([]Tag, error) {
	var tags []Tag
	for _, line := range strings.Split(text, "\n") {
		if line == "" {
			continue
		}
		f := strings.Split(line, nulSeparator)
		if len(f) != tagFieldCount || !strings.HasPrefix(f[0], tagRefPrefix) {
			return nil, fmt.Errorf("git for-each-ref: malformed tag record %q", line)
		}
		t, err := parseUnix(f[3])
		if err != nil {
			return nil, err
		}
		tag := Tag{Name: strings.TrimPrefix(f[0], tagRefPrefix), OID: f[1], Subject: f[4], Time: t}
		if f[2] != "" {
			tag.OID, tag.Annotated = f[2], true
		}
		tags = append(tags, tag)
	}
	return tags, nil
}

func (r *Repo) CreateBranch(ctx context.Context, name, start string, checkout bool) error {
	if err := checkRef(name); err != nil {
		return err
	}
	args := []string{"branch", name}
	if checkout {
		args = []string{"switch", "--create", name}
	}
	if start != "" {
		if err := checkRef(start); err != nil {
			return err
		}
		args = append(args, start)
	}
	_, err := r.write(ctx, args...)
	return err
}

func (r *Repo) Checkout(ctx context.Context, ref string, force bool) error {
	if err := checkRef(ref); err != nil {
		return err
	}
	args := []string{"checkout"}
	if force {
		args = append(args, "--force")
	}
	_, err := r.write(ctx, append(args, ref, "--")...)
	return err
}

func (r *Repo) CheckoutNewTracking(ctx context.Context, remoteBranch string) error {
	if err := checkRef(remoteBranch); err != nil {
		return err
	}
	_, local, ok := strings.Cut(remoteBranch, "/")
	if !ok || local == "" {
		return fmt.Errorf("git: %q is not a remote branch name", remoteBranch)
	}
	_, err := r.write(ctx, "switch", "--track", "--create", local, remoteBranch)
	return err
}

func (r *Repo) RenameBranch(ctx context.Context, oldName, newName string) error {
	if err := checkRefs(oldName, newName); err != nil {
		return err
	}
	_, err := r.write(ctx, "branch", "--move", oldName, newName)
	return err
}

func (r *Repo) DeleteBranch(ctx context.Context, name string, force bool) error {
	if err := checkRef(name); err != nil {
		return err
	}
	args := []string{"branch", "--delete"}
	if force {
		args = append(args, "--force")
	}
	_, err := r.write(ctx, append(args, name)...)
	return err
}

func (r *Repo) CompareBranches(ctx context.Context, a, b string) (BranchComparison, error) {
	if err := checkRefs(a, b); err != nil {
		return BranchComparison{}, err
	}
	aOnly, err := r.Log(ctx, LogOptions{Revs: []string{b + ".." + a}})
	if err != nil {
		return BranchComparison{}, err
	}
	bOnly, err := r.Log(ctx, LogOptions{Revs: []string{a + ".." + b}})
	if err != nil {
		return BranchComparison{}, err
	}
	return BranchComparison{AOnly: aOnly, BOnly: bOnly}, nil
}
