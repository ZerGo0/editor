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

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/telemetry"
	"go.opentelemetry.io/otel/attribute"
)

const (
	DefaultMaxFileSize int64 = 10 << 20
	DefaultLimit             = 2000
	binarySniffBytes         = 8000
	maxDefaultWorkers        = 4
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
	o = o.withDefaults()
	ctx, span := telemetry.Start(ctx, telemetry.SpanTextSearch)
	defer span.End()
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	results, counts := startScan(runCtx, m, o)
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
	st.Files = int(counts.scanned.Load())
	st.Skipped = int(counts.skipped.Load())
	span.SetAttributes(attribute.Int("files", st.Files), attribute.Int("matched", st.Matched), attribute.Bool("truncated", st.Truncated))
	return st, ctx.Err()
}

func (o Options) withDefaults() Options {
	if o.MaxFileSize <= 0 {
		o.MaxFileSize = DefaultMaxFileSize
	}
	if o.Limit <= 0 {
		o.Limit = DefaultLimit
	}
	if o.Workers <= 0 {
		o.Workers = max(1, min(runtime.GOMAXPROCS(0)-1, maxDefaultWorkers))
	}
	return o
}

type scanCounts struct {
	scanned, skipped atomic.Int64
}

func startScan(runCtx context.Context, m *Matcher, o Options) (<-chan FileResult, *scanCounts) {
	paths := make(chan string)
	results := make(chan FileResult)
	counts := &scanCounts{}

	applog.Go(func() {
		defer close(paths)
		for _, p := range o.Files {
			select {
			case paths <- p:
			case <-runCtx.Done():
				return
			}
		}
	})

	var wg sync.WaitGroup
	for range o.Workers {
		wg.Add(1)
		applog.Go(func() {
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
					counts.skipped.Add(1)
					continue
				}
				counts.scanned.Add(1)
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
		})
	}
	applog.Go(func() {
		wg.Wait()
		close(results)
	})
	return results, counts
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
