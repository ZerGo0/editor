package filetree

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path"
	"strings"
)

const (
	maxIgnoredEntries = 200_000
	notRepoExitCode   = 128
	notRepoMessage    = "not a git repository"
)

type Ignored struct {
	paths map[string]bool
	dirs  map[string]bool
}

func ParseIgnored(out []byte) Ignored {
	s := Ignored{paths: map[string]bool{}, dirs: map[string]bool{}}
	for i, entry := range bytes.Split(out, []byte{0}) {
		if i >= maxIgnoredEntries {
			break
		}
		p := string(entry)
		if p == "" || strings.HasPrefix(p, "../") || path.IsAbs(p) {
			continue
		}
		if dir, ok := strings.CutSuffix(p, "/"); ok {
			s.dirs[path.Clean(dir)] = true
			continue
		}
		s.paths[path.Clean(p)] = true
	}
	return s
}

func (s Ignored) Has(rel string) bool {
	if rel == "" || rel == "." {
		return false
	}
	if s.paths[rel] || s.dirs[rel] {
		return true
	}
	for dir := path.Dir(rel); dir != "." && dir != "/"; dir = path.Dir(dir) {
		if s.dirs[dir] {
			return true
		}
	}
	return false
}

func (s Ignored) Len() int {
	return len(s.paths) + len(s.dirs)
}

func LoadIgnored(ctx context.Context, root string) (Ignored, error) {
	cmd := exec.CommandContext(ctx, "git", "ls-files", "--others", "--ignored", "--exclude-standard", "--directory", "-z")
	cmd.Dir = root
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	var exit *exec.ExitError
	switch {
	case errors.As(err, &exit) && exit.ExitCode() == notRepoExitCode && strings.Contains(stderr.String(), notRepoMessage):
		return Ignored{}, nil
	case err != nil:
		return Ignored{}, fmt.Errorf("list ignored files in %s: %w: %s", root, err, strings.TrimSpace(stderr.String()))
	}
	return ParseIgnored(out), nil
}
