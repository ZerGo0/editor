package git

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	commitFormat = "%H%x00%P%x00%an%x00%ae%x00%at%x00%cn%x00%ce%x00%ct%x00%D%x00%s%x00%b"

	commitFieldCount = 11
	fieldOID         = 0
	fieldParents     = 1
	fieldAuthor      = 2
	fieldAuthorMail  = 3
	fieldAuthorTime  = 4
	fieldCommitter   = 5
	fieldCommitTime  = 7
	fieldRefs        = 8
	fieldSubject     = 9
	fieldBody        = 10

	noCommitsMarker = "does not have any commits yet"
	headArgMarker   = "'HEAD'"
	refSeparator    = ", "
)

type Commit struct {
	OID         string
	Parents     []string
	Author      string
	AuthorEmail string
	AuthorTime  time.Time
	Committer   string
	CommitTime  time.Time
	Subject     string
	Body        string
	Refs        []string
}

type LogOptions struct {
	Revs   []string
	Paths  []string
	Follow bool
	Max    int
	Skip   int
	Author string
	Grep   string
	All    bool
}

type ChangedFile struct {
	Path     string
	OrigPath string
	Status   byte
}

func (r *Repo) Log(ctx context.Context, o LogOptions) ([]Commit, error) {
	if err := checkRefs(o.Revs...); err != nil {
		return nil, err
	}
	if err := checkPaths(o.Paths); err != nil {
		return nil, err
	}
	if o.Follow && len(o.Paths) != 1 {
		return nil, errors.New("git log: follow needs exactly one path")
	}
	if o.Max < 0 || o.Skip < 0 {
		return nil, errors.New("git log: negative limit")
	}
	var args []string
	if o.All {
		args = append(args, "--all")
	}
	if o.Follow {
		args = append(args, "--follow")
	}
	if o.Max > 0 {
		args = append(args, "--max-count="+strconv.Itoa(o.Max))
	}
	if o.Skip > 0 {
		args = append(args, "--skip="+strconv.Itoa(o.Skip))
	}
	if o.Author != "" {
		args = append(args, "--author="+o.Author)
	}
	if o.Grep != "" {
		args = append(args, "--grep="+o.Grep)
	}
	args = append(args, o.Revs...)
	return r.logRaw(ctx, withPaths(args, o.Paths))
}

func (r *Repo) logRaw(ctx context.Context, args []string) ([]Commit, error) {
	base := []string{"log", "-z", "--no-show-signature", "--topo-order", "--encoding=UTF-8", "--format=" + commitFormat}
	out, err := r.read(ctx, append(base, args...)...)
	if err != nil {
		var ge *Error
		if errors.As(err, &ge) && (strings.Contains(ge.Stderr, noCommitsMarker) || r.unbornHead(ctx, ge)) {
			return nil, nil
		}
		return nil, err
	}
	fields := splitNUL(out)
	if len(fields)%commitFieldCount != 0 {
		return nil, fmt.Errorf("git log: unexpected field count %d", len(fields))
	}
	commits := make([]Commit, 0, len(fields)/commitFieldCount)
	for i := 0; i < len(fields); i += commitFieldCount {
		c, err := parseCommitFields(fields[i : i+commitFieldCount])
		if err != nil {
			return nil, err
		}
		commits = append(commits, c)
	}
	return commits, nil
}

func (r *Repo) unbornHead(ctx context.Context, ge *Error) bool {
	if !strings.Contains(ge.Stderr, headArgMarker) {
		return false
	}
	ok, err := r.hasHead(ctx)
	return err == nil && !ok
}

func parseCommitFields(f []string) (Commit, error) {
	if len(f) != commitFieldCount || f[fieldOID] == "" {
		return Commit{}, fmt.Errorf("git log: malformed commit record %q", f)
	}
	authored, err := parseUnix(f[fieldAuthorTime])
	if err != nil {
		return Commit{}, err
	}
	committed, err := parseUnix(f[fieldCommitTime])
	if err != nil {
		return Commit{}, err
	}
	c := Commit{
		OID:         f[fieldOID],
		Parents:     strings.Fields(f[fieldParents]),
		Author:      f[fieldAuthor],
		AuthorEmail: f[fieldAuthorMail],
		AuthorTime:  authored,
		Committer:   f[fieldCommitter],
		CommitTime:  committed,
		Subject:     f[fieldSubject],
		Body:        strings.TrimRight(f[fieldBody], "\n"),
	}
	if f[fieldRefs] != "" {
		c.Refs = strings.Split(f[fieldRefs], refSeparator)
	}
	return c, nil
}

func parseUnix(s string) (time.Time, error) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("git: malformed timestamp %q", s)
	}
	return time.Unix(n, 0), nil
}

func (r *Repo) CommitFiles(ctx context.Context, oid string) ([]ChangedFile, error) {
	if err := checkRef(oid); err != nil {
		return nil, err
	}
	base, err := r.diffBase(ctx, oid)
	if err != nil {
		return nil, err
	}
	out, err := r.read(ctx, "diff", "--name-status", "-z", "-M", "--no-ext-diff", base, oid, "--")
	if err != nil {
		return nil, err
	}
	return parseNameStatus(out)
}

func parseNameStatus(data []byte) ([]ChangedFile, error) {
	fields := splitNUL(data)
	var files []ChangedFile
	for i := 0; i < len(fields); i++ {
		code := fields[i]
		if code == "" {
			return nil, errors.New("git diff: empty status code")
		}
		f := ChangedFile{Status: code[0]}
		need := 1
		if f.Status == 'R' || f.Status == 'C' {
			need = 2
		}
		if i+need >= len(fields) {
			return nil, fmt.Errorf("git diff: truncated name-status entry %q", code)
		}
		if need == 2 {
			f.OrigPath, f.Path = fields[i+1], fields[i+2]
		} else {
			f.Path = fields[i+1]
		}
		i += need
		files = append(files, f)
	}
	return files, nil
}
