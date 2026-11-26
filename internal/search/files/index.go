package files

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os/exec"
	"path/filepath"
	"slices"
	"sync"
)

const (
	MaxFiles = 500_000
	gitDir   = ".git"
)

var errTooMany = errors.New("too many files")

type Index struct {
	Root  string
	mu    sync.RWMutex
	paths []string
	ready bool
}

func (x *Index) Paths() ([]string, bool) {
	x.mu.RLock()
	defer x.mu.RUnlock()
	return x.paths, x.ready
}

func (x *Index) Refresh(ctx context.Context, excluded func(rel string) bool) error {
	paths, err := gitFiles(ctx, x.Root)
	if err != nil {
		paths, err = walk(ctx, x.Root, excluded)
	}
	if err != nil && !errors.Is(err, errTooMany) {
		return err
	}
	paths = slices.DeleteFunc(paths, excluded)
	slices.Sort(paths)
	x.mu.Lock()
	x.paths, x.ready = paths, true
	x.mu.Unlock()
	return nil
}

func gitFiles(ctx context.Context, root string) ([]string, error) {
	listed, err := git(ctx, root, "--cached", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	deleted, err := git(ctx, root, "--deleted")
	if err != nil {
		return nil, err
	}
	gone := make(map[string]bool, len(deleted))
	for _, p := range deleted {
		gone[p] = true
	}
	paths := slices.DeleteFunc(listed, func(p string) bool { return gone[p] })
	return slices.Compact(slices.Sorted(slices.Values(paths))), nil
}

func git(ctx context.Context, root string, args ...string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", root, "ls-files", "-z"}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var paths []string
	for p := range bytes.SplitSeq(out, []byte{0}) {
		if len(p) > 0 {
			paths = append(paths, string(p))
		}
		if len(paths) >= MaxFiles {
			break
		}
	}
	return paths, nil
}

func walk(ctx context.Context, root string, excluded func(string) bool) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		rel, err := filepath.Rel(root, p)
		if err != nil || rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if d.Name() == gitDir || excluded != nil && excluded(rel) {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() && d.Type()&fs.ModeSymlink == 0 {
			return nil
		}
		paths = append(paths, rel)
		if len(paths) >= MaxFiles {
			return errTooMany
		}
		return nil
	})
	return paths, err
}
