package gitui

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/ZerGo0/editor/internal/git"
)

const (
	gitCommand     = "git"
	commandDelay   = 3 * time.Second
	nameStatusZ    = "\x00"
	originHead     = "refs/remotes/origin/HEAD"
	remoteRefs     = "refs/remotes/"
	localRefs      = "refs/heads/"
	fallbackBranch = "master"
)

var defaultBranchRefs = []string{remoteRefs + "origin/main", localRefs + "main", remoteRefs + "origin/master"}

func checkName(s string) error {
	if s == "" || strings.HasPrefix(s, "-") || strings.ContainsAny(s, "\x00\n ") {
		return fmt.Errorf("invalid name %q", s)
	}
	return nil
}

func checkURL(s string) error {
	if s == "" || strings.HasPrefix(s, "-") || strings.ContainsAny(s, "\x00\n") {
		return fmt.Errorf("invalid URL %q", s)
	}
	return nil
}

func gitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, gitCommand, args...)
	cmd.Dir = dir
	cmd.WaitDelay = commandDelay
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "LC_ALL=C")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return "", fmt.Errorf("git %s: %w", args[0], err)
		}
		return "", fmt.Errorf("git %s: %s", args[0], msg)
	}
	return string(out), nil
}

func runGit(ctx context.Context, dir string, args ...string) error {
	_, err := gitOutput(ctx, dir, args...)
	return err
}

func (r *cliRepo) CreateTag(ctx context.Context, name, rev, message string) error {
	if err := checkName(name); err != nil {
		return err
	}
	if err := checkName(rev); err != nil {
		return err
	}
	if strings.TrimSpace(message) == "" {
		return runGit(ctx, r.Root, "tag", name, rev)
	}
	return runGit(ctx, r.Root, "tag", "-a", "-m", message, name, rev)
}

func (r *cliRepo) DeleteTag(ctx context.Context, name string) error {
	if err := checkName(name); err != nil {
		return err
	}
	return runGit(ctx, r.Root, "tag", "-d", name)
}

func (r *cliRepo) PushRef(ctx context.Context, remote, refspec string) error {
	if err := checkName(remote); err != nil {
		return err
	}
	if err := checkName(refspec); err != nil {
		return err
	}
	return runGit(ctx, r.Root, "push", remote, refspec)
}

func (r *cliRepo) AddRemote(ctx context.Context, name, url string) error {
	if err := checkName(name); err != nil {
		return err
	}
	if err := checkURL(url); err != nil {
		return err
	}
	return runGit(ctx, r.Root, "remote", "add", name, url)
}

func (r *cliRepo) SetRemoteURL(ctx context.Context, name, url string) error {
	if err := checkName(name); err != nil {
		return err
	}
	if err := checkURL(url); err != nil {
		return err
	}
	return runGit(ctx, r.Root, "remote", "set-url", name, url)
}

func (r *cliRepo) RenameRemote(ctx context.Context, oldName, newName string) error {
	if err := checkName(oldName); err != nil {
		return err
	}
	if err := checkName(newName); err != nil {
		return err
	}
	return runGit(ctx, r.Root, "remote", "rename", oldName, newName)
}

func (r *cliRepo) RemoveRemote(ctx context.Context, name string) error {
	if err := checkName(name); err != nil {
		return err
	}
	return runGit(ctx, r.Root, "remote", "remove", name)
}

func (r *cliRepo) BranchesContaining(ctx context.Context, oid string) ([]string, error) {
	if err := checkName(oid); err != nil {
		return nil, err
	}
	out, err := gitOutput(ctx, r.Root, "for-each-ref", "--contains", oid, "--format=%(refname:short)", "refs/heads", "refs/remotes")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, line := range strings.Split(out, "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasSuffix(line, "/HEAD") {
			names = append(names, line)
		}
	}
	return names, nil
}

func (r *cliRepo) ChangedSince(ctx context.Context, rev string) ([]git.ChangedFile, error) {
	if err := checkName(rev); err != nil {
		return nil, err
	}
	out, err := gitOutput(ctx, r.Root, "diff", "--name-status", "-z", "-M", "--no-ext-diff", rev, "--")
	if err != nil {
		return nil, err
	}
	return parseNameStatus(out)
}

func (r *cliRepo) MergeBase(ctx context.Context, a, b string) (string, error) {
	if err := checkName(a); err != nil {
		return "", err
	}
	if err := checkName(b); err != nil {
		return "", err
	}
	out, err := gitOutput(ctx, r.Root, "merge-base", "--end-of-options", a, b)
	return strings.TrimSpace(out), err
}

func (r *cliRepo) DefaultBranch(ctx context.Context) string {
	if out, err := gitOutput(ctx, r.Root, "symbolic-ref", originHead); err == nil {
		if ref := strings.TrimSpace(out); ref != "" && r.hasRef(ctx, ref) {
			return strings.TrimPrefix(ref, remoteRefs)
		}
	}
	for _, name := range defaultBranchRefs {
		if r.hasRef(ctx, name) {
			return strings.TrimPrefix(strings.TrimPrefix(name, remoteRefs), localRefs)
		}
	}
	return fallbackBranch
}

func (r *cliRepo) hasRef(ctx context.Context, ref string) bool {
	return runGit(ctx, r.Root, "show-ref", "--verify", "--quiet", ref) == nil
}

func parseNameStatus(out string) ([]git.ChangedFile, error) {
	fields := strings.Split(strings.TrimSuffix(out, nameStatusZ), nameStatusZ)
	var files []git.ChangedFile
	for i := 0; i < len(fields); {
		code := fields[i]
		if code == "" {
			break
		}
		i++
		f := git.ChangedFile{Status: code[0]}
		if code[0] == 'R' || code[0] == 'C' {
			if i+1 >= len(fields) {
				return nil, fmt.Errorf("git diff: truncated rename entry")
			}
			f.OrigPath, f.Path = fields[i], fields[i+1]
			i += 2
		} else {
			if i >= len(fields) {
				return nil, fmt.Errorf("git diff: truncated entry")
			}
			f.Path = fields[i]
			i++
		}
		files = append(files, f)
	}
	return files, nil
}
