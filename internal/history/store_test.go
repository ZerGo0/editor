package history

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	testRoot = "/proj"
	testFile = "/proj/a.txt"
	testGB   = 1 << 30
)

func openTest(t *testing.T, limits Limits) (*Store, *time.Time) {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "history"), limits)
	if err != nil {
		t.Fatal(err)
	}
	clock := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return clock }
	s.free = func(string) (uint64, error) { return 10 * testGB, nil }
	return s, &clock
}

func reopen(t *testing.T, s *Store) *Store {
	t.Helper()
	again, err := Open(s.dir, Limits{MaxAge: 100 * 365 * 24 * time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	again.now = s.now
	return again
}

func mustRecord(t *testing.T, s *Store, path, content, label string) {
	t.Helper()
	if err := s.Record(testRoot, path, []byte(content), label); err != nil {
		t.Fatal(err)
	}
}

func TestRecordAndRead(t *testing.T) {
	s, clock := openTest(t, Limits{})
	mustRecord(t, s, testFile, "one", "")
	*clock = clock.Add(time.Minute)
	mustRecord(t, s, testFile, "two", "")
	vs, err := s.Versions(testRoot, testFile)
	if err != nil || len(vs) != 2 {
		t.Fatalf("versions %v, err %v", vs, err)
	}
	if !vs[0].Time.After(vs[1].Time) || vs[0].Size != 3 {
		t.Errorf("not newest first: %+v", vs)
	}
	got, err := s.Read(vs[0].Hash)
	if err != nil || string(got) != "two" {
		t.Errorf("read %q, err %v", got, err)
	}
}

func TestSameContentTwiceRecordedOnce(t *testing.T) {
	s, _ := openTest(t, Limits{})
	mustRecord(t, s, testFile, "same", "")
	mustRecord(t, s, testFile, "same", "")
	if vs, _ := s.Versions(testRoot, testFile); len(vs) != 1 {
		t.Errorf("want 1 version, got %d", len(vs))
	}
	mustRecord(t, s, testFile, "same", "Release")
	vs, _ := s.Versions(testRoot, testFile)
	if len(vs) != 2 || vs[0].Label != "Release" {
		t.Errorf("label not recorded: %+v", vs)
	}
}

func TestPersistsAcrossOpen(t *testing.T) {
	s, _ := openTest(t, Limits{})
	mustRecord(t, s, testFile, "kept", "L")
	again := reopen(t, s)
	vs, err := again.Versions(testRoot, testFile)
	if err != nil || len(vs) != 1 || vs[0].Label != "L" {
		t.Errorf("versions %+v, err %v", vs, err)
	}
}

func TestProjectsAreSeparate(t *testing.T) {
	s, _ := openTest(t, Limits{})
	mustRecord(t, s, testFile, "x", "")
	if vs, _ := s.Versions("/other", testFile); len(vs) != 0 {
		t.Errorf("other project sees %v", vs)
	}
}

func TestTooLarge(t *testing.T) {
	s, _ := openTest(t, Limits{MaxFileBytes: 4})
	if err := s.Record(testRoot, testFile, []byte("12345"), ""); !errors.Is(err, ErrTooLarge) {
		t.Errorf("err %v", err)
	}
	if vs, _ := s.Versions(testRoot, testFile); len(vs) != 0 {
		t.Error("recorded a large file")
	}
}

func TestLowDisk(t *testing.T) {
	s, _ := openTest(t, Limits{})
	s.free = func(string) (uint64, error) { return MinFreeBytes - 1, nil }
	if err := s.Record(testRoot, testFile, []byte("x"), ""); !errors.Is(err, ErrLowDisk) {
		t.Errorf("err %v", err)
	}
}

func TestReadRejectsBadHash(t *testing.T) {
	s, _ := openTest(t, Limits{})
	for _, h := range []string{"", "../../etc/passwd", strings.Repeat("z", hashHexLen)} {
		if _, err := s.Read(h); err == nil {
			t.Errorf("accepted %q", h)
		}
	}
	if _, err := s.Read(hashOf([]byte("missing"))); err == nil {
		t.Error("missing blob read")
	}
}

func TestMalformedIndexLinesSkipped(t *testing.T) {
	s, _ := openTest(t, Limits{})
	mustRecord(t, s, testFile, "ok", "")
	x, _ := s.index(testRoot)
	f, err := os.OpenFile(s.indexPath(x.key), os.O_APPEND|os.O_WRONLY, filePerm)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("not json\n{\"path\":\"/proj/a.txt\",\"hash\":\"zz\"}\n{\"path\"")
	f.Close()
	again := reopen(t, s)
	if vs, err := again.Versions(testRoot, testFile); err != nil || len(vs) != 1 {
		t.Errorf("versions %v, err %v", vs, err)
	}
}

func TestPruneByAge(t *testing.T) {
	s, clock := openTest(t, Limits{MaxAge: time.Hour})
	mustRecord(t, s, testFile, "old", "")
	mustRecord(t, s, testFile, "named", "Keep")
	*clock = clock.Add(2 * time.Hour)
	mustRecord(t, s, testFile, "new", "")
	if err := s.Prune(); err != nil {
		t.Fatal(err)
	}
	vs, _ := s.Versions(testRoot, testFile)
	if len(vs) != 2 || vs[0].Size != 3 || vs[1].Label != "Keep" {
		t.Errorf("after prune %+v", vs)
	}
	if _, err := s.Read(hashOf([]byte("old"))); err == nil {
		t.Error("unreferenced blob not removed")
	}
}

func TestPruneByCount(t *testing.T) {
	s, clock := openTest(t, Limits{MaxVersionsPerFile: 2})
	for _, c := range []string{"1", "2", "3", "4"} {
		*clock = clock.Add(time.Second)
		mustRecord(t, s, testFile, c, "")
	}
	mustRecord(t, s, "/proj/b.txt", "b", "")
	if err := s.Prune(); err != nil {
		t.Fatal(err)
	}
	vs, _ := s.Versions(testRoot, testFile)
	if len(vs) != 2 || vs[0].Size != 1 {
		t.Fatalf("versions %+v", vs)
	}
	got, _ := s.Read(vs[1].Hash)
	if string(got) != "3" {
		t.Errorf("kept %q, want 3", got)
	}
	if vs, _ := s.Versions(testRoot, "/proj/b.txt"); len(vs) != 1 {
		t.Error("other file pruned")
	}
}

func TestPruneByTotalSize(t *testing.T) {
	s, clock := openTest(t, Limits{})
	for i := range 4 {
		*clock = clock.Add(time.Second)
		mustRecord(t, s, testFile, strings.Repeat(string(rune('a'+i)), 2000)+string(rune(i)), "")
	}
	st, _ := os.Stat(s.blobPath(hashOf([]byte(strings.Repeat("a", 2000) + "\x00"))))
	s.limits.MaxTotalBytes = st.Size() * 2
	if err := s.Prune(); err != nil {
		t.Fatal(err)
	}
	vs, _ := s.Versions(testRoot, testFile)
	if len(vs) != 2 {
		t.Fatalf("versions %d, want 2", len(vs))
	}
	if _, err := s.Read(vs[0].Hash); err != nil {
		t.Errorf("newest lost: %v", err)
	}
}

func TestPruneEveryRecords(t *testing.T) {
	s, _ := openTest(t, Limits{MaxVersionsPerFile: 1})
	for i := range PruneEvery {
		mustRecord(t, s, testFile, strings.Repeat("x", i+1), "")
	}
	if vs, _ := s.Versions(testRoot, testFile); len(vs) != 1 {
		t.Errorf("not pruned automatically: %d", len(vs))
	}
}

func TestPruneDropsEntriesWithMissingBlob(t *testing.T) {
	s, _ := openTest(t, Limits{})
	mustRecord(t, s, testFile, "gone", "")
	os.Remove(s.blobPath(hashOf([]byte("gone"))))
	if err := s.Prune(); err != nil {
		t.Fatal(err)
	}
	if vs, _ := s.Versions(testRoot, testFile); len(vs) != 0 {
		t.Errorf("versions %v", vs)
	}
}

func TestRenamed(t *testing.T) {
	s, _ := openTest(t, Limits{})
	mustRecord(t, s, testFile, "v1", "")
	mustRecord(t, s, "/proj/dir/c.txt", "c", "")
	if err := s.Renamed(testRoot, testFile, "/proj/b.txt"); err != nil {
		t.Fatal(err)
	}
	if err := s.Renamed(testRoot, "/proj/dir", "/proj/moved"); err != nil {
		t.Fatal(err)
	}
	check := func(st *Store) {
		t.Helper()
		if vs, _ := st.Versions(testRoot, "/proj/b.txt"); len(vs) != 1 {
			t.Errorf("renamed file versions %v", vs)
		}
		if vs, _ := st.Versions(testRoot, "/proj/moved/c.txt"); len(vs) != 1 {
			t.Errorf("moved dir versions %v", vs)
		}
		if vs, _ := st.Versions(testRoot, testFile); len(vs) != 0 {
			t.Errorf("old path versions %v", vs)
		}
	}
	check(s)
	again := reopen(t, s)
	check(again)
}

func TestConcurrentRecords(t *testing.T) {
	s, _ := openTest(t, Limits{})
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mustRecord(t, s, testFile, strings.Repeat("y", i+1), "")
			_, _ = s.Versions(testRoot, testFile)
		}()
	}
	wg.Wait()
	if vs, _ := s.Versions(testRoot, testFile); len(vs) != 20 {
		t.Errorf("versions %d", len(vs))
	}
}

func TestBlobsAreCompressed(t *testing.T) {
	s, _ := openTest(t, Limits{})
	content := bytes.Repeat([]byte("abc"), 10000)
	if err := s.Record(testRoot, testFile, content, ""); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(s.blobPath(hashOf(content)))
	if err != nil || st.Size() >= int64(len(content))/10 {
		t.Errorf("blob size %v, err %v", st, err)
	}
}
