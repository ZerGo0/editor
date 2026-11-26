package textsearch

import (
	"context"
	"io/fs"
	"path/filepath"
)

const (
	MaxWalkFiles = 500_000
	gitDirName   = ".git"
)

func WalkFiles(ctx context.Context, root string, skipDir func(rel string) bool) ([]string, error) {
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
			if d.Name() == gitDirName || skipDir != nil && skipDir(rel) {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		paths = append(paths, rel)
		if len(paths) >= MaxWalkFiles {
			return filepath.SkipAll
		}
		return nil
	})
	return paths, err
}
