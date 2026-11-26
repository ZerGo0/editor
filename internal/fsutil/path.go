package fsutil

import "path/filepath"

func Within(dir, path string) bool {
	r, err := filepath.Rel(dir, path)
	return err == nil && filepath.IsLocal(r)
}
