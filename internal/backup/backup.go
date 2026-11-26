package backup

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/ZerGo0/editor/internal/fsutil"
)

const (
	DefaultLimit = 256 << 20
	metaExt      = ".json"
	textExt      = ".txt"
	keyBytes     = 16
	filePerm     = 0o600
	dirPerm      = 0o700
)

var (
	ErrTooLarge = errors.New("backups are over their size limit")
	errNoPath   = errors.New("backup without a path")
	errSize     = errors.New("backup text does not match its size")
)

type Meta struct {
	Path        string    `json:"path"`
	Language    string    `json:"language,omitempty"`
	DiskSize    int64     `json:"diskSize,omitempty"`
	DiskModTime time.Time `json:"diskModTime,omitzero"`
	Size        int64     `json:"size"`
}

type Store struct {
	dir   string
	limit int64
	mu    sync.Mutex
	seq   map[string]uint64
	metas map[string]Meta
}

func Open(dir string, limit int64) *Store {
	s := &Store{dir: dir, limit: limit, seq: map[string]uint64{}, metas: map[string]Meta{}}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			log.Printf("backups %s: %v", dir, err)
		}
		return s
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != metaExt {
			continue
		}
		m, err := readMeta(filepath.Join(dir, e.Name()))
		if err != nil {
			log.Printf("backup %s: %v", e.Name(), err)
			continue
		}
		s.metas[m.Path] = m
	}
	return s
}

func readMeta(path string) (Meta, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Meta{}, err
	}
	var m Meta
	if err := json.Unmarshal(data, &m); err != nil {
		return Meta{}, err
	}
	if m.Path == "" {
		return Meta{}, errNoPath
	}
	return m, nil
}

func key(path string) string {
	sum := sha256.Sum256([]byte(path))
	return hex.EncodeToString(sum[:keyBytes])
}

func (s *Store) Entries() []Meta {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Meta, 0, len(s.metas))
	for _, m := range s.metas {
		out = append(out, m)
	}
	slices.SortFunc(out, func(a, b Meta) int { return strings.Compare(a.Path, b.Path) })
	return out
}

func (s *Store) Has(path string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.metas[path]
	return ok
}

func (s *Store) total() int64 {
	var n int64
	for _, m := range s.metas {
		n += m.Size
	}
	return n
}

func (s *Store) Write(seq uint64, m Meta, text io.WriterTo, size int64) error {
	if m.Path == "" {
		return errNoPath
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if seq <= s.seq[m.Path] {
		return nil
	}
	if s.total()-s.metas[m.Path].Size+size > s.limit {
		return ErrTooLarge
	}
	if err := os.MkdirAll(s.dir, dirPerm); err != nil {
		return err
	}
	k := key(m.Path)
	var written int64
	err := fsutil.WriteAtomic(filepath.Join(s.dir, k+textExt), filePerm, func(w io.Writer) error {
		n, err := text.WriteTo(w)
		written = n
		return err
	})
	if err != nil {
		return fmt.Errorf("write backup of %s: %w", m.Path, err)
	}
	m.Size = written
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(filepath.Join(s.dir, k+metaExt), data, filePerm); err != nil {
		return fmt.Errorf("write backup of %s: %w", m.Path, err)
	}
	s.seq[m.Path], s.metas[m.Path] = seq, m
	return nil
}

func (s *Store) Delete(seq uint64, path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if seq <= s.seq[path] {
		return nil
	}
	s.seq[path] = seq
	if _, ok := s.metas[path]; !ok {
		return nil
	}
	delete(s.metas, path)
	k := key(path)
	return errors.Join(remove(filepath.Join(s.dir, k+metaExt)), remove(filepath.Join(s.dir, k+textExt)))
}

func remove(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (s *Store) Read(path string) (string, Meta, error) {
	s.mu.Lock()
	m, ok := s.metas[path]
	s.mu.Unlock()
	if !ok {
		return "", Meta{}, os.ErrNotExist
	}
	data, err := os.ReadFile(filepath.Join(s.dir, key(path)+textExt))
	if err != nil {
		return "", m, fmt.Errorf("read backup of %s: %w", path, err)
	}
	if int64(len(data)) != m.Size {
		return "", m, fmt.Errorf("read backup of %s: %w", path, errSize)
	}
	return string(data), m, nil
}

func (s *Store) Clear(seq uint64) error {
	var errs []error
	for _, m := range s.Entries() {
		errs = append(errs, s.Delete(seq, m.Path))
	}
	return errors.Join(errs...)
}
