package gitui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/ZerGo0/editor/internal/git"
)

const (
	maxDiffBytes = 20 << 20
	binaryProbe  = 8000
	headRev      = "HEAD"
	indexRev     = ""
)

var (
	errBinary   = errors.New("This file is binary and has no text diff.")
	errTooLarge = fmt.Errorf("This file is larger than %d MB and is not compared.", maxDiffBytes>>20)
)

type diffKind int

const (
	diffWorkTree diffKind = iota
	diffStaged
	diffUntracked
	diffHead
	diffCommit
	diffCompare
)

type diffSource struct {
	kind    diffKind
	path    string
	oldPath string
	rev     string
	parent  string
}

func (s diffSource) usesWorkTree() bool {
	switch s.kind {
	case diffWorkTree, diffUntracked, diffHead, diffCompare:
		return true
	}
	return false
}

func (s diffSource) id() string {
	return fmt.Sprintf("%s%d:%s:%s:%s", diffViewPrefix, s.kind, s.rev, s.oldPath, s.path)
}

func (s diffSource) title() string {
	name, _ := splitPath(s.path)
	switch s.kind {
	case diffStaged:
		return name + " (Staged)"
	case diffUntracked:
		return name + " (New)"
	case diffHead:
		return name + " (HEAD)"
	case diffCommit:
		return name + " (" + shortHash(s.rev) + ")"
	case diffCompare:
		return name + " (" + shortHash(s.rev) + " vs Local)"
	}
	return name + " (Changes)"
}

func (s diffSource) old() string {
	if s.oldPath != "" {
		return s.oldPath
	}
	return s.path
}

func loadDiffTexts(ctx context.Context, r repo, s diffSource, abs string, readDisk bool) (string, string, error) {
	var oldData, newData []byte
	var err error
	show := func(rev, path string) ([]byte, error) {
		data, err := r.Show(ctx, rev, path)
		if errors.Is(err, git.ErrNotFound) {
			return nil, nil
		}
		return data, err
	}
	disk := func() ([]byte, error) {
		if !readDisk {
			return nil, nil
		}
		return readFileLimited(abs)
	}
	switch s.kind {
	case diffWorkTree:
		if oldData, err = show(indexRev, s.path); err == nil {
			newData, err = disk()
		}
	case diffStaged:
		if oldData, err = show(headRev, s.old()); err == nil {
			newData, err = show(indexRev, s.path)
		}
	case diffUntracked:
		newData, err = disk()
	case diffHead:
		if oldData, err = show(headRev, s.old()); err == nil {
			newData, err = disk()
		}
	case diffCommit:
		if s.parent != "" {
			oldData, err = show(s.parent, s.old())
		}
		if err == nil {
			newData, err = show(s.rev, s.path)
		}
	case diffCompare:
		if oldData, err = show(s.rev, s.old()); err == nil {
			newData, err = disk()
		}
	}
	if err != nil {
		return "", "", err
	}
	if err := checkText(oldData); err != nil {
		return "", "", err
	}
	if err := checkText(newData); err != nil {
		return "", "", err
	}
	return string(oldData), string(newData), nil
}

func readFileLimited(abs string) ([]byte, error) {
	info, err := os.Stat(abs)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if info.Size() > maxDiffBytes {
		return nil, errTooLarge
	}
	data, err := os.ReadFile(abs)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return data, err
}

func checkText(data []byte) error {
	if len(data) > maxDiffBytes {
		return errTooLarge
	}
	if bytes.IndexByte(data[:min(len(data), binaryProbe)], 0) >= 0 {
		return errBinary
	}
	return nil
}
