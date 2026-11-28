package fsutil

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteAtomicKeepsAReadOnlyFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "ro.txt")
	if err := os.WriteFile(p, []byte("old"), 0o444); err != nil {
		t.Fatal(err)
	}
	err := WriteFileAtomic(p, []byte("new"), 0o644)
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("err = %v, want a permission error", err)
	}
	if data, _ := os.ReadFile(p); string(data) != "old" {
		t.Fatalf("content = %q, want the old content", data)
	}
}

func TestWriteAtomicErrorNamesTheTargetNotTheTempFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "gone", "e.txt")
	err := WriteFileAtomic(p, []byte("x"), 0o644)
	var pe *fs.PathError
	if !errors.As(err, &pe) || pe.Path != p || pe.Op != saveOp || !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("err = %#v, want a save error for %s", err, p)
	}
	if strings.Contains(err.Error(), ".tmp") {
		t.Fatalf("err %q names the temp file", err)
	}
}
