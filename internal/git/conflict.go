package git

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	stageBase   = 1
	stageOurs   = 2
	stageTheirs = 3
	lsFilesTab  = "\t"
	lsFilesCols = 3
)

type ConflictFile struct {
	Path   string
	Base   bool
	Ours   bool
	Theirs bool
}

func (r *Repo) Conflicts(ctx context.Context) ([]ConflictFile, error) {
	out, err := r.read(ctx, "ls-files", "--unmerged", "-z")
	if err != nil {
		return nil, err
	}
	return parseConflicts(out)
}

func parseConflicts(data []byte) ([]ConflictFile, error) {
	var files []ConflictFile
	index := make(map[string]int)
	for _, rec := range splitNUL(data) {
		meta, path, ok := strings.Cut(rec, lsFilesTab)
		f := strings.Fields(meta)
		if !ok || path == "" || len(f) != lsFilesCols {
			return nil, fmt.Errorf("git ls-files: malformed record %q", rec)
		}
		stage, err := strconv.Atoi(f[2])
		if err != nil {
			return nil, fmt.Errorf("git ls-files: malformed stage in %q", rec)
		}
		i, seen := index[path]
		if !seen {
			i = len(files)
			index[path] = i
			files = append(files, ConflictFile{Path: path})
		}
		switch stage {
		case stageBase:
			files[i].Base = true
		case stageOurs:
			files[i].Ours = true
		case stageTheirs:
			files[i].Theirs = true
		default:
			return nil, fmt.Errorf("git ls-files: unexpected stage %d", stage)
		}
	}
	return files, nil
}

func (r *Repo) MergeVersions(ctx context.Context, path string) (base, ours, theirs []byte, err error) {
	stages := []*[]byte{&base, &ours, &theirs}
	for i, dst := range stages {
		data, err := r.Show(ctx, ":"+strconv.Itoa(stageBase+i), path)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, nil, nil, err
		}
		*dst = data
	}
	return base, ours, theirs, nil
}

func (r *Repo) MarkResolved(ctx context.Context, paths ...string) error {
	return r.StageFiles(ctx, paths...)
}

func (r *Repo) AcceptSide(ctx context.Context, path string, ours bool) error {
	if err := checkPath(path); err != nil {
		return err
	}
	conflicts, err := r.Conflicts(ctx)
	if err != nil {
		return err
	}
	var entry *ConflictFile
	for i := range conflicts {
		if conflicts[i].Path == path {
			entry = &conflicts[i]
		}
	}
	if entry == nil {
		return fmt.Errorf("git: %q has no conflict", path)
	}
	side, present := "--theirs", entry.Theirs
	if ours {
		side, present = "--ours", entry.Ours
	}
	if !present {
		_, err = r.write(ctx, "rm", "--quiet", "--", path)
		return err
	}
	if _, err = r.write(ctx, "checkout", side, "--", path); err != nil {
		return err
	}
	return r.StageFiles(ctx, path)
}
