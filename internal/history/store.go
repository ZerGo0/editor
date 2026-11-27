package history

import (
	"bufio"
	"bytes"
	"compress/flate"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ZerGo0/editor/internal/fsutil"
)

const (
	DefaultMaxAge             = 30 * 24 * time.Hour
	DefaultMaxTotalBytes      = 500 << 20
	DefaultMaxFileBytes       = 5 << 20
	DefaultMaxVersionsPerFile = 200
	MinFreeBytes              = 1 << 30
	PruneEvery                = 50

	blobsDir     = "blobs"
	indexDir     = "index"
	indexExt     = ".jsonl"
	dirPerm      = 0o755
	filePerm     = 0o644
	hashHexLen   = sha256.Size * 2
	maxIndexLine = 1 << 20
)

var (
	ErrLowDisk  = errors.New("history: not enough free disk space")
	ErrTooLarge = errors.New("history: file is too large to record")
	errBadHash  = errors.New("history: invalid version hash")
)

type Limits struct {
	MaxAge             time.Duration
	MaxTotalBytes      int64
	MaxFileBytes       int64
	MaxVersionsPerFile int
}

func (l Limits) withDefaults() Limits {
	if l.MaxAge <= 0 {
		l.MaxAge = DefaultMaxAge
	}
	if l.MaxTotalBytes <= 0 {
		l.MaxTotalBytes = DefaultMaxTotalBytes
	}
	if l.MaxFileBytes <= 0 {
		l.MaxFileBytes = DefaultMaxFileBytes
	}
	if l.MaxVersionsPerFile <= 0 {
		l.MaxVersionsPerFile = DefaultMaxVersionsPerFile
	}
	return l
}

type Version struct {
	Time  time.Time
	Hash  string
	Size  int64
	Label string
}

type entry struct {
	Path  string    `json:"path"`
	Time  time.Time `json:"time"`
	Hash  string    `json:"hash,omitempty"`
	Size  int64     `json:"size,omitempty"`
	Label string    `json:"label,omitempty"`
	From  string    `json:"from,omitempty"`
	drop  bool
}

func (e entry) isRename() bool {
	return e.Hash == "" && e.From != ""
}

type index struct {
	key     string
	entries []entry
}

func (x *index) last(path string) (entry, bool) {
	for i := len(x.entries) - 1; i >= 0; i-- {
		if x.entries[i].Path == path {
			return x.entries[i], true
		}
	}
	return entry{}, false
}

func (x *index) rename(from, to string) bool {
	changed := false
	for i := range x.entries {
		if p, ok := remapPath(x.entries[i].Path, from, to); ok {
			x.entries[i].Path = p
			changed = true
		}
	}
	return changed
}

func remapPath(p, from, to string) (string, bool) {
	if p == from {
		return to, true
	}
	if prefix := from + string(filepath.Separator); strings.HasPrefix(p, prefix) {
		return to + p[len(from):], true
	}
	return p, false
}

type Store struct {
	dir     string
	limits  Limits
	mu      sync.Mutex
	indexes map[string]*index
	records int
	now     func() time.Time
	free    func(dir string) (uint64, error)
}

func Open(dir string, limits Limits) (*Store, error) {
	s := &Store{
		dir:     dir,
		limits:  limits.withDefaults(),
		indexes: map[string]*index{},
		now:     time.Now,
		free:    fsutil.FreeBytes,
	}
	for _, sub := range []string{blobsDir, indexDir} {
		if err := os.MkdirAll(filepath.Join(dir, sub), dirPerm); err != nil {
			return nil, fmt.Errorf("history: create store: %w", err)
		}
	}
	_ = s.Prune()
	return s, nil
}

func hashOf(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func validHash(h string) bool {
	if len(h) != hashHexLen {
		return false
	}
	_, err := hex.DecodeString(h)
	return err == nil
}

func (s *Store) blobPath(hash string) string {
	return filepath.Join(s.dir, blobsDir, hash[:2], hash)
}

func (s *Store) indexPath(key string) string {
	return filepath.Join(s.dir, indexDir, key+indexExt)
}

func (s *Store) index(root string) (*index, error) {
	return s.loadIndex(hashOf([]byte(root)))
}

func (s *Store) loadIndex(key string) (*index, error) {
	if x, ok := s.indexes[key]; ok {
		return x, nil
	}
	x := &index{key: key}
	f, err := os.Open(s.indexPath(key))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("history: read index: %w", err)
	}
	if err == nil {
		defer f.Close()
		sc := bufio.NewScanner(f)
		sc.Buffer(nil, maxIndexLine)
		for sc.Scan() {
			var e entry
			if json.Unmarshal(sc.Bytes(), &e) != nil || e.Path == "" {
				continue
			}
			switch {
			case e.isRename():
				x.rename(e.From, e.Path)
			case validHash(e.Hash):
				x.entries = append(x.entries, e)
			}
		}
		if err := sc.Err(); err != nil {
			return nil, fmt.Errorf("history: read index: %w", err)
		}
	}
	s.indexes[key] = x
	return x, nil
}

func (s *Store) appendEntry(key string, e entry) error {
	line, err := json.Marshal(e)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(s.indexPath(key), os.O_APPEND|os.O_CREATE|os.O_WRONLY, filePerm)
	if err != nil {
		return fmt.Errorf("history: append index: %w", err)
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		_ = f.Close()
		return fmt.Errorf("history: append index: %w", err)
	}
	return f.Close()
}

func (s *Store) writeBlob(hash string, content []byte) error {
	p := s.blobPath(hash)
	if _, err := os.Stat(p); err == nil {
		return nil
	}
	var buf bytes.Buffer
	w, err := flate.NewWriter(&buf, flate.DefaultCompression)
	if err != nil {
		return err
	}
	if _, err := w.Write(content); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), dirPerm); err != nil {
		return fmt.Errorf("history: write blob: %w", err)
	}
	if err := fsutil.WriteFileAtomic(p, buf.Bytes(), filePerm); err != nil {
		return fmt.Errorf("history: write blob: %w", err)
	}
	return nil
}

func (s *Store) Record(root, path string, content []byte, label string) error {
	if int64(len(content)) > s.limits.MaxFileBytes {
		return ErrTooLarge
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	free, err := s.free(s.dir)
	if err != nil {
		return fmt.Errorf("history: check free space: %w", err)
	}
	if free < MinFreeBytes {
		return ErrLowDisk
	}
	x, err := s.index(root)
	if err != nil {
		return err
	}
	hash := hashOf(content)
	if last, ok := x.last(path); ok && last.Hash == hash && label == "" {
		return nil
	}
	if err := s.writeBlob(hash, content); err != nil {
		return err
	}
	e := entry{Path: path, Time: s.now().UTC(), Hash: hash, Size: int64(len(content)), Label: label}
	if err := s.appendEntry(x.key, e); err != nil {
		return err
	}
	x.entries = append(x.entries, e)
	s.records++
	if s.records%PruneEvery == 0 {
		return s.prune()
	}
	return nil
}

func (s *Store) Versions(root, path string) ([]Version, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, err := s.index(root)
	if err != nil {
		return nil, err
	}
	var out []Version
	for i := len(x.entries) - 1; i >= 0; i-- {
		if e := x.entries[i]; e.Path == path {
			out = append(out, Version{Time: e.Time, Hash: e.Hash, Size: e.Size, Label: e.Label})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Time.After(out[j].Time) })
	return out, nil
}

func (s *Store) Read(hash string) ([]byte, error) {
	if !validHash(hash) {
		return nil, errBadHash
	}
	f, err := os.Open(s.blobPath(hash))
	if err != nil {
		return nil, fmt.Errorf("history: read version: %w", err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(flate.NewReader(f), s.limits.MaxFileBytes+1))
	if err != nil {
		return nil, fmt.Errorf("history: read version: %w", err)
	}
	if int64(len(data)) > s.limits.MaxFileBytes {
		return nil, fmt.Errorf("history: read version: %w", ErrTooLarge)
	}
	return data, nil
}

func (s *Store) Renamed(root, from, to string) error {
	if from == to {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	x, err := s.index(root)
	if err != nil {
		return err
	}
	if !x.rename(from, to) {
		return nil
	}
	return s.appendEntry(x.key, entry{Path: to, From: from, Time: s.now().UTC()})
}

func (s *Store) Prune() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.prune()
}

func (s *Store) prune() error {
	files, err := os.ReadDir(filepath.Join(s.dir, indexDir))
	if err != nil {
		return fmt.Errorf("history: prune: %w", err)
	}
	cached := map[string]bool{}
	for k := range s.indexes {
		cached[k] = true
	}
	var all []*index
	for _, f := range files {
		key, ok := strings.CutSuffix(f.Name(), indexExt)
		if !ok {
			continue
		}
		x, err := s.loadIndex(key)
		if err != nil {
			return err
		}
		all = append(all, x)
	}
	s.dropExpired(all)
	refs := s.dropOverSize(all)
	if err := s.commit(all); err != nil {
		return err
	}
	for k := range s.indexes {
		if !cached[k] {
			delete(s.indexes, k)
		}
	}
	return s.removeUnreferenced(refs)
}

func (s *Store) dropExpired(all []*index) {
	cutoff := s.now().Add(-s.limits.MaxAge)
	for _, x := range all {
		kept := map[string]int{}
		for i := len(x.entries) - 1; i >= 0; i-- {
			e := &x.entries[i]
			if e.Label != "" {
				continue
			}
			kept[e.Path]++
			if e.Time.Before(cutoff) || kept[e.Path] > s.limits.MaxVersionsPerFile {
				e.drop = true
			}
		}
	}
}

func (s *Store) dropOverSize(all []*index) map[string]int {
	type ref struct {
		x *index
		i int
	}
	var live []ref
	sizes := map[string]int64{}
	refs := map[string]int{}
	for _, x := range all {
		for i := range x.entries {
			e := &x.entries[i]
			if e.drop {
				continue
			}
			size, ok := sizes[e.Hash]
			if !ok {
				st, err := os.Stat(s.blobPath(e.Hash))
				if err != nil {
					e.drop = true
					continue
				}
				size = st.Size()
				sizes[e.Hash] = size
			}
			refs[e.Hash]++
			live = append(live, ref{x, i})
		}
	}
	var total int64
	for h := range refs {
		total += sizes[h]
	}
	sort.SliceStable(live, func(a, b int) bool {
		return live[a].x.entries[live[a].i].Time.Before(live[b].x.entries[live[b].i].Time)
	})
	for _, r := range live {
		if total <= s.limits.MaxTotalBytes {
			break
		}
		e := &r.x.entries[r.i]
		e.drop = true
		if refs[e.Hash]--; refs[e.Hash] == 0 {
			total -= sizes[e.Hash]
			delete(refs, e.Hash)
		}
	}
	return refs
}

func (s *Store) commit(all []*index) error {
	for _, x := range all {
		kept := x.entries[:0:0]
		for _, e := range x.entries {
			if !e.drop {
				kept = append(kept, e)
			}
		}
		if len(kept) == len(x.entries) {
			continue
		}
		x.entries = kept
		if len(kept) == 0 {
			if err := os.Remove(s.indexPath(x.key)); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("history: prune: %w", err)
			}
			continue
		}
		err := fsutil.WriteAtomic(s.indexPath(x.key), filePerm, func(w io.Writer) error {
			enc := json.NewEncoder(w)
			for _, e := range kept {
				if err := enc.Encode(e); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("history: prune: %w", err)
		}
	}
	return nil
}

func (s *Store) removeUnreferenced(refs map[string]int) error {
	root := filepath.Join(s.dir, blobsDir)
	return filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !validHash(d.Name()) || refs[d.Name()] > 0 {
			return err
		}
		if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("history: prune: %w", err)
		}
		return nil
	})
}
