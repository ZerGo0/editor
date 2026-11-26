package textsearch

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

const (
	DefaultMaxFileSize int64 = 10 << 20
	DefaultLimit             = 2000
	binarySniffBytes         = 8000
)

type FileResult struct {
	Path    string
	Matches []Match
}

type Stats struct {
	Files     int
	Matched   int
	Skipped   int
	Truncated bool
}

type Options struct {
	Root        string
	Files       []string
	Overlay     func(path string) (text string, ok bool)
	MaxFileSize int64
	Limit       int
	Workers     int
}

func SortResults(rs []FileResult) {
	sort.Slice(rs, func(i, j int) bool { return rs[i].Path < rs[j].Path })
}

func Search(ctx context.Context, m *Matcher, o Options, emit func(FileResult)) (Stats, error) {
	if m == nil {
		return Stats{}, fmt.Errorf("textsearch: nil matcher")
	}
	if o.MaxFileSize <= 0 {
		o.MaxFileSize = DefaultMaxFileSize
	}
	if o.Limit <= 0 {
		o.Limit = DefaultLimit
	}
	if o.Workers <= 0 {
		o.Workers = max(1, runtime.GOMAXPROCS(0)-1)
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	paths := make(chan string)
	results := make(chan FileResult)
	var scanned, skipped atomic.Int64

	go func() {
		defer close(paths)
		for _, p := range o.Files {
			select {
			case paths <- p:
			case <-runCtx.Done():
				return
			}
		}
	}()

	var wg sync.WaitGroup
	for range o.Workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range paths {
				if runCtx.Err() != nil {
					return
				}
				if !m.Wants(relPath(o.Root, p)) {
					continue
				}
				text, ok := loadText(p, o)
				if !ok {
					skipped.Add(1)
					continue
				}
				scanned.Add(1)
				matches := scanText(runCtx, m, text, o.Limit+1)
				if len(matches) == 0 {
					continue
				}
				select {
				case results <- FileResult{Path: p, Matches: matches}:
				case <-runCtx.Done():
					return
				}
			}
		}()
	}
	go func() {
		wg.Wait()
		close(results)
	}()

	var st Stats
	total := 0
	for r := range results {
		if st.Truncated {
			continue
		}
		if room := o.Limit - total; len(r.Matches) > room {
			r.Matches = r.Matches[:room]
			st.Truncated = true
			cancel()
		}
		if len(r.Matches) == 0 {
			continue
		}
		total += len(r.Matches)
		st.Matched++
		emit(r)
	}
	st.Files = int(scanned.Load())
	st.Skipped = int(skipped.Load())
	return st, ctx.Err()
}

func relPath(root, p string) string {
	if root != "" {
		if rel, err := filepath.Rel(root, p); err == nil {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(p)
}

func loadText(p string, o Options) (string, bool) {
	if o.Overlay != nil {
		if text, ok := o.Overlay(p); ok {
			return text, true
		}
	}
	text, _, err := readText(p, o.MaxFileSize)
	return text, err == nil
}

func readText(p string, maxSize int64) (string, os.FileInfo, error) {
	info, err := os.Stat(p)
	if err != nil {
		return "", nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxSize {
		return "", nil, fmt.Errorf("textsearch: %s is not a regular file within %d bytes", p, maxSize)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return "", nil, err
	}
	if bytes.IndexByte(data[:min(len(data), binarySniffBytes)], 0) >= 0 {
		return "", nil, fmt.Errorf("textsearch: %s is a binary file", p)
	}
	return string(data), info, nil
}

func scanText(ctx context.Context, m *Matcher, text string, limit int) []Match {
	if m.multiline && strings.Contains(text, "\r\n") {
		text = strings.ReplaceAll(text, "\r\n", "\n")
	}
	var out []Match
	var rc runeCounter
	m.each(ctx, text, limit, func(h hit) bool {
		out = append(out, buildMatch(text, h, &rc))
		return len(out) < limit
	})
	return out
}
