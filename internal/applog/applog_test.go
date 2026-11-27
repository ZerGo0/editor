package applog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriterRotatesAndKeepsBackups(t *testing.T) {
	const max, backups, lines = 100, 2, 20
	path := filepath.Join(t.TempDir(), "logs", FileName)
	w, err := Open(path, max, backups)
	if err != nil {
		t.Fatal(err)
	}
	line := strings.Repeat("x", 39) + "\n"
	for range lines {
		if _, err := w.Write([]byte(line)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
		info, _ := e.Info()
		if info.Size() > max {
			t.Errorf("%s is %d bytes, more than %d", e.Name(), info.Size(), max)
		}
	}
	want := []string{"editor.1.log", "editor.2.log", "editor.log"}
	if strings.Join(names, " ") != strings.Join(want, " ") {
		t.Fatalf("files = %v, want %v", names, want)
	}
}

func TestWriterAppendsAfterReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	for _, s := range []string{"one\n", "two\n"} {
		w, err := Open(path, MaxSize, MaxBackups)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(s)); err != nil {
			t.Fatal(err)
		}
		w.Close()
	}
	got, _ := os.ReadFile(path)
	if string(got) != "one\ntwo\n" {
		t.Fatalf("log = %q", got)
	}
}
