package files

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ZerGo0/editor/internal/syntax"
	"github.com/egoist/mygo/ui/editor"
)

const (
	maxScanFileSize = 1 << 20
	binarySniffSize = 8000
)

type Visitor interface {
	Update(path string, lang *editor.Language, text string)
	Remove(path string)
}

type Scanner struct {
	Root     string
	Visitors []Visitor
	MaxFiles int
	mu       sync.Mutex
	mtimes   map[string]time.Time
}

func (s *Scanner) Scan(ctx context.Context, rels []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.mtimes == nil {
		s.mtimes = map[string]time.Time{}
	}
	if s.MaxFiles > 0 && len(rels) > s.MaxFiles {
		rels = rels[:s.MaxFiles]
	}
	seen := make(map[string]bool, len(rels))
	for _, rel := range rels {
		if ctx.Err() != nil {
			return
		}
		lang := syntax.ForName(rel)
		if lang == nil {
			continue
		}
		abs := filepath.Join(s.Root, filepath.FromSlash(rel))
		seen[abs] = true
		info, err := os.Stat(abs)
		if err != nil || info.Size() > maxScanFileSize || s.mtimes[abs].Equal(info.ModTime()) {
			continue
		}
		data, err := os.ReadFile(abs)
		if err != nil || bytes.IndexByte(data[:min(len(data), binarySniffSize)], 0) >= 0 {
			continue
		}
		text := string(data)
		for _, v := range s.Visitors {
			v.Update(abs, lang, text)
		}
		s.mtimes[abs] = info.ModTime()
	}
	for p := range s.mtimes {
		if !seen[p] {
			delete(s.mtimes, p)
			for _, v := range s.Visitors {
				v.Remove(p)
			}
		}
	}
}
