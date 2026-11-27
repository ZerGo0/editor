package fsutil

import (
	"os"
	"path/filepath"
)

const HomePrefix = "~"

func Within(dir, path string) bool {
	r, err := filepath.Rel(dir, path)
	return err == nil && filepath.IsLocal(r)
}

func DisplayPath(path string) string {
	if home, err := os.UserHomeDir(); err == nil && Within(home, path) {
		r, _ := filepath.Rel(home, path)
		return filepath.Join(HomePrefix, r)
	}
	return path
}
