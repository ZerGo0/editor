package git

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	shelfPatchFile  = "patch.diff"
	shelfMetaFile   = "meta.json"
	shelfDirMode    = 0o755
	shelfFileMode   = 0o644
	shelfIDTime     = "20060102T150405Z"
	shelfSlugLimit  = 40
	shelfIDLimit    = 128
	shelfDefaultTag = "shelf"
)

var (
	ErrNothingToShelve = errors.New("git: no changes to shelve")
	shelfIDPattern     = regexp.MustCompile("^[A-Za-z0-9][A-Za-z0-9._-]*$")
	shelfSlugStrip     = regexp.MustCompile("[^a-z0-9]+")
)

type Shelf struct {
	Dir string
}

type ShelfEntry struct {
	ID      string
	Name    string
	Created time.Time
	Base    string
}

type shelfMeta struct {
	Name    string
	Created time.Time
	Base    string
}

func validShelfID(id string) error {
	if len(id) > shelfIDLimit || !shelfIDPattern.MatchString(id) || strings.Contains(id, "..") {
		return fmt.Errorf("git shelf: invalid id %q", id)
	}
	return nil
}

func (s Shelf) entryDir(id string) (string, error) {
	if s.Dir == "" {
		return "", errors.New("git shelf: no directory configured")
	}
	if err := validShelfID(id); err != nil {
		return "", err
	}
	return filepath.Join(s.Dir, id), nil
}

func (s Shelf) Shelve(ctx context.Context, repo *Repo, name string, paths []string, keep bool) (ShelfEntry, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return ShelfEntry{}, errors.New("git shelf: empty name")
	}
	if err := checkPaths(paths); err != nil {
		return ShelfEntry{}, err
	}
	born, err := repo.hasHead(ctx)
	if err != nil {
		return ShelfEntry{}, err
	}
	if !born {
		return ShelfEntry{}, errors.New("git shelf: repository has no commits")
	}
	base, err := repo.readText(ctx, "rev-parse", "HEAD")
	if err != nil {
		return ShelfEntry{}, err
	}
	args := append([]string{"diff", "--binary", "--no-renames"}, diffFlags[:3]...)
	patch, err := repo.read(ctx, withPaths(append(args, "HEAD"), paths)...)
	if err != nil {
		return ShelfEntry{}, err
	}
	if len(patch) == 0 {
		return ShelfEntry{}, ErrNothingToShelve
	}
	entry, err := s.create(name, base, patch)
	if err != nil {
		return ShelfEntry{}, err
	}
	if !keep {
		if err := repo.revertToHead(ctx, paths); err != nil {
			return entry, err
		}
	}
	return entry, nil
}

func (s Shelf) create(name, base string, patch []byte) (ShelfEntry, error) {
	if err := os.MkdirAll(s.Dir, shelfDirMode); err != nil {
		return ShelfEntry{}, err
	}
	created := time.Now().UTC()
	prefix := created.Format(shelfIDTime) + "-" + shelfSlug(name)
	id := prefix
	for n := 2; ; n++ {
		err := os.Mkdir(filepath.Join(s.Dir, id), shelfDirMode)
		if err == nil {
			break
		}
		if !errors.Is(err, fs.ErrExist) {
			return ShelfEntry{}, err
		}
		id = prefix + "-" + strconv.Itoa(n)
	}
	dir := filepath.Join(s.Dir, id)
	meta, err := json.MarshalIndent(shelfMeta{Name: name, Created: created, Base: base}, "", "  ")
	if err == nil {
		err = os.WriteFile(filepath.Join(dir, shelfPatchFile), patch, shelfFileMode)
	}
	if err == nil {
		err = os.WriteFile(filepath.Join(dir, shelfMetaFile), meta, shelfFileMode)
	}
	if err != nil {
		_ = os.RemoveAll(dir)
		return ShelfEntry{}, err
	}
	return ShelfEntry{ID: id, Name: name, Created: created, Base: base}, nil
}

func shelfSlug(name string) string {
	slug := strings.Trim(shelfSlugStrip.ReplaceAllString(strings.ToLower(name), "-"), "-")
	if len(slug) > shelfSlugLimit {
		slug = strings.Trim(slug[:shelfSlugLimit], "-")
	}
	if slug == "" {
		return shelfDefaultTag
	}
	return slug
}

func (r *Repo) revertToHead(ctx context.Context, paths []string) error {
	out, err := r.read(ctx, withPaths([]string{"diff", "--name-status", "-z", "--no-renames", "--no-ext-diff", "HEAD"}, paths)...)
	if err != nil {
		return err
	}
	changes, err := parseNameStatus(out)
	if err != nil {
		return err
	}
	var added, restored []string
	for _, c := range changes {
		if c.Status == 'A' {
			added = append(added, c.Path)
		} else {
			restored = append(restored, c.Path)
		}
	}
	if len(added) > 0 {
		if _, err := r.write(ctx, withPaths([]string{"rm", "--force", "--quiet"}, added)...); err != nil {
			return err
		}
	}
	if len(restored) > 0 {
		_, err = r.write(ctx, withPaths([]string{"restore", "--source=HEAD", "--staged", "--worktree"}, restored)...)
	}
	return err
}

func (s Shelf) List() ([]ShelfEntry, error) {
	if s.Dir == "" {
		return nil, errors.New("git shelf: no directory configured")
	}
	dirs, err := os.ReadDir(s.Dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var entries []ShelfEntry
	for _, d := range dirs {
		if !d.IsDir() || validShelfID(d.Name()) != nil {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.Dir, d.Name(), shelfMetaFile))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		var m shelfMeta
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("git shelf: bad metadata for %q: %w", d.Name(), err)
		}
		entries = append(entries, ShelfEntry{ID: d.Name(), Name: m.Name, Created: m.Created, Base: m.Base})
	}
	sort.Slice(entries, func(i, j int) bool {
		if !entries[i].Created.Equal(entries[j].Created) {
			return entries[i].Created.After(entries[j].Created)
		}
		return entries[i].ID > entries[j].ID
	})
	return entries, nil
}

func (s Shelf) Unshelve(ctx context.Context, repo *Repo, id string, remove bool) (OpResult, error) {
	dir, err := s.entryDir(id)
	if err != nil {
		return OpResult{}, err
	}
	patch := filepath.Join(dir, shelfPatchFile)
	if _, err := os.Stat(patch); errors.Is(err, fs.ErrNotExist) {
		return OpResult{}, ErrNotFound
	} else if err != nil {
		return OpResult{}, err
	}
	_, err = repo.write(ctx, "apply", "--3way", "--whitespace=nowarn", patch)
	if err != nil {
		conflicts, cErr := repo.Conflicts(ctx)
		if cErr == nil && len(conflicts) > 0 {
			return OpResult{Stopped: true, Output: err.Error()}, nil
		}
		if _, plainErr := repo.write(ctx, "apply", "--whitespace=nowarn", patch); plainErr != nil {
			return OpResult{}, plainErr
		}
	}
	if remove {
		return OpResult{}, s.Delete(id)
	}
	return OpResult{}, nil
}

func (s Shelf) Delete(id string) error {
	dir, err := s.entryDir(id)
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(dir, shelfMetaFile)); errors.Is(err, fs.ErrNotExist) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	return os.RemoveAll(dir)
}
