package hexdump

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"time"
)

const waitLimit = 5 * time.Second

func testData(size int) []byte {
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i * 7)
	}
	return data
}

func loadPage(t *testing.T, s *Source, loaded chan struct{}, i int64) ([]byte, error) {
	t.Helper()
	deadline := time.After(waitLimit)
	for {
		data, err := s.Page(i)
		if !errors.Is(err, ErrLoading) {
			return data, err
		}
		select {
		case <-loaded:
		case <-deadline:
			t.Fatalf("page %d did not load", i)
		}
	}
}

func TestSourcePagesMatchTheFile(t *testing.T) {
	data := testData(PageSize*3 + 123)
	loaded := make(chan struct{}, 64)
	s := NewSource(bytes.NewReader(data), int64(len(data)), func() { loaded <- struct{}{} })
	if _, err := s.Page(0); !errors.Is(err, ErrLoading) {
		t.Fatalf("first request: %v, want ErrLoading", err)
	}
	for i := int64(0); i < PageCount(s.Size()); i++ {
		got, err := loadPage(t, s, loaded, i)
		if err != nil {
			t.Fatalf("page %d: %v", i, err)
		}
		start, end := PageBounds(i, s.Size())
		if !bytes.Equal(got, data[start:end]) {
			t.Errorf("page %d differs from the file", i)
		}
	}
}

func TestSourceRejectsPagesOutsideTheFile(t *testing.T) {
	s := NewSource(bytes.NewReader(make([]byte, 10)), 10, nil)
	for _, i := range []int64{-1, 1, 99} {
		if _, err := s.Page(i); !errors.Is(err, ErrPageRange) {
			t.Errorf("Page(%d) error = %v, want ErrPageRange", i, err)
		}
	}
}

func TestSourceKeepsFewPages(t *testing.T) {
	pages := maxCachedPages * 3
	data := testData(PageSize * pages)
	loaded := make(chan struct{}, 4*pages)
	s := NewSource(bytes.NewReader(data), int64(len(data)), func() { loaded <- struct{}{} })
	for i := int64(0); i < int64(pages); i++ {
		if _, err := loadPage(t, s, loaded, i); err != nil {
			t.Fatal(err)
		}
		s.mu.Lock()
		n := len(s.pages)
		s.mu.Unlock()
		if n > maxCachedPages {
			t.Fatalf("%d pages cached after page %d, limit %d", n, i, maxCachedPages)
		}
	}
}

type shrunkFile struct{ io.ReaderAt }

func TestSourceReportsShortReads(t *testing.T) {
	loaded := make(chan struct{}, 4)
	s := NewSource(shrunkFile{bytes.NewReader(make([]byte, 10))}, PageSize, func() { loaded <- struct{}{} })
	_, err := loadPage(t, s, loaded, 0)
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("error = %v, want unexpected EOF", err)
	}
}

func TestReadRange(t *testing.T) {
	data := testData(100)
	got, err := ReadRange(bytes.NewReader(data), 10, 20)
	if err != nil || !bytes.Equal(got, data[10:20]) {
		t.Fatalf("ReadRange = %v, %v", got, err)
	}
	if _, err := ReadRange(bytes.NewReader(data), 90, 110); err == nil {
		t.Fatal("ReadRange past the end did not fail")
	}
}
