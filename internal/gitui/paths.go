package gitui

import (
	"path"
	"path/filepath"
	"strings"
)

type pathMap struct {
	root     string
	repoRoot string
	prefix   string
}

func newPathMap(root, repoRoot string) pathMap {
	real := root
	if r, err := filepath.EvalSymlinks(root); err == nil {
		real = r
	}
	p := pathMap{root: root, repoRoot: repoRoot}
	if rel, err := filepath.Rel(repoRoot, real); err == nil && rel != "." && filepath.IsLocal(rel) {
		p.prefix = filepath.ToSlash(rel)
	}
	return p
}

func (p pathMap) rel(abs string) (string, bool) {
	if abs == "" {
		return "", false
	}
	if r, err := filepath.Rel(p.root, abs); err == nil && r != "." && filepath.IsLocal(r) {
		return path.Join(p.prefix, filepath.ToSlash(r)), true
	}
	if r, err := filepath.Rel(p.repoRoot, abs); err == nil && r != "." && filepath.IsLocal(r) {
		return filepath.ToSlash(r), true
	}
	return "", false
}

func (p pathMap) abs(rel string) string {
	if p.prefix == "" {
		return filepath.Join(p.root, filepath.FromSlash(rel))
	}
	if r, ok := strings.CutPrefix(rel, p.prefix+"/"); ok {
		return filepath.Join(p.root, filepath.FromSlash(r))
	}
	return filepath.Join(p.repoRoot, filepath.FromSlash(rel))
}
