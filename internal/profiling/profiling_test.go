package profiling

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCaptureWritesProfilesAndKeepsTheNewest(t *testing.T) {
	root := t.TempDir()
	start := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	var last string
	for i := range Keep + 2 {
		dir, err := Capture(root, start.Add(time.Duration(i)*time.Second), time.Millisecond)
		if err != nil {
			t.Fatal(err)
		}
		last = dir
	}
	for _, name := range []string{GoroutinesFile, HeapFile, CPUFile} {
		info, err := os.Stat(filepath.Join(last, name))
		if err != nil || info.Size() == 0 {
			t.Fatalf("%s: %v", name, err)
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != Keep {
		t.Fatalf("kept %d captures, want %d", len(entries), Keep)
	}
	if entries[len(entries)-1].Name() != filepath.Base(last) {
		t.Fatalf("newest kept %s, want %s", entries[len(entries)-1].Name(), filepath.Base(last))
	}
}
