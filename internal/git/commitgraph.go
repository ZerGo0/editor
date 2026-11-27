package git

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var commitGraphPaths = []string{"objects/info/commit-graph", "objects/info/commit-graphs/commit-graph-chain"}

func (r *Repo) EnsureCommitGraph(ctx context.Context) error {
	args := []string{"rev-parse"}
	for _, p := range commitGraphPaths {
		args = append(args, "--git-path", p)
	}
	out, err := r.readText(ctx, args...)
	if err != nil {
		return err
	}
	for _, p := range strings.Split(out, "\n") {
		if !filepath.IsAbs(p) {
			p = filepath.Join(r.Root, p)
		}
		if _, err := os.Stat(p); err == nil || !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	_, err = r.write(ctx, "commit-graph", "write", "--reachable", "--no-progress")
	return err
}
