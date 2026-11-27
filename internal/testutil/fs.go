package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

const (
	dirPerm  = 0o755
	filePerm = 0o644
)

type content interface{ ~string | ~[]byte }

func WriteFile[V content](t testing.TB, path string, data V) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), filePerm); err != nil {
		t.Fatal(err)
	}
}

func WriteFiles[V content](t testing.TB, root string, files map[string]V) []string {
	t.Helper()
	paths := make([]string, 0, len(files))
	for name, data := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		WriteFile(t, path, data)
		paths = append(paths, path)
	}
	return paths
}

func ReadFile(t testing.TB, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func ReadText(t testing.TB, path string) string {
	t.Helper()
	return string(ReadFile(t, path))
}

func TempProject[V content](t testing.TB, files map[string]V) (root string, paths []string) {
	t.Helper()
	root = t.TempDir()
	return root, WriteFiles(t, root, files)
}
