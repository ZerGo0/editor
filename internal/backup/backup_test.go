package backup

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type text string

func (t text) WriteTo(w io.Writer) (int64, error) {
	n, err := io.WriteString(w, string(t))
	return int64(n), err
}

func write(t *testing.T, s *Store, seq uint64, path, body string) {
	t.Helper()
	if err := s.Write(seq, Meta{Path: path}, text(body), int64(len(body))); err != nil {
		t.Fatal(err)
	}
}

func TestWriteReadAndReopen(t *testing.T) {
	dir := t.TempDir()
	s := Open(dir, DefaultLimit)
	stamp := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := s.Write(1, Meta{Path: "/p/a.go", Language: "Go", DiskSize: 3, DiskModTime: stamp}, text("draft"), 5); err != nil {
		t.Fatal(err)
	}
	write(t, s, 2, "untitled:Untitled-1", "notes")
	again := Open(dir, DefaultLimit)
	got, m, err := again.Read("/p/a.go")
	if err != nil || got != "draft" || m.Language != "Go" || m.DiskSize != 3 || !m.DiskModTime.Equal(stamp) {
		t.Fatalf("Read = %q %+v %v", got, m, err)
	}
	if entries := again.Entries(); len(entries) != 2 || entries[0].Path != "/p/a.go" {
		t.Fatalf("entries = %+v", entries)
	}
}

func TestStaleWritesAndDeletesAreIgnored(t *testing.T) {
	s := Open(t.TempDir(), DefaultLimit)
	write(t, s, 3, "/a", "new")
	write(t, s, 2, "/a", "old")
	if got, _, _ := s.Read("/a"); got != "new" {
		t.Fatalf("stale write won: %q", got)
	}
	if err := s.Delete(4, "/a"); err != nil {
		t.Fatal(err)
	}
	write(t, s, 4, "/a", "late")
	if s.Has("/a") {
		t.Fatal("a write older than the delete brought the backup back")
	}
	write(t, s, 5, "/a", "again")
	if !s.Has("/a") {
		t.Fatal("a newer write after the delete was dropped")
	}
}

func TestLimit(t *testing.T) {
	s := Open(t.TempDir(), 10)
	write(t, s, 1, "/a", "123456")
	if err := s.Write(2, Meta{Path: "/b"}, text("12345"), 5); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("err = %v, want ErrTooLarge", err)
	}
	write(t, s, 3, "/a", "1234567890")
}

func TestClearAndCorruptFiles(t *testing.T) {
	dir := t.TempDir()
	s := Open(dir, DefaultLimit)
	write(t, s, 1, "/a", "x")
	if err := os.WriteFile(filepath.Join(dir, "bad.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := len(Open(dir, DefaultLimit).Entries()); got != 1 {
		t.Fatalf("entries with a corrupt meta = %d", got)
	}
	if err := s.Clear(2); err != nil {
		t.Fatal(err)
	}
	names, _ := os.ReadDir(dir)
	for _, n := range names {
		if !strings.HasPrefix(n.Name(), "bad") {
			t.Fatalf("left %s after Clear", n.Name())
		}
	}
	if err := os.WriteFile(filepath.Join(dir, key("/t")+metaExt), []byte(`{"path": "/t", "size": 9}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Open(dir, DefaultLimit).Read("/t"); err == nil {
		t.Fatal("read a backup whose text is missing")
	}
}
