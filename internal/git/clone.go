package git

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"unicode"
)

const (
	cloneSuffix      = ".git"
	cloneDefaultName = "repository"
)

var (
	ErrNoCloneURL  = errors.New("Enter a repository URL.")
	ErrBadCloneURL = errors.New("This is not a valid repository URL.")
)

func CheckCloneURL(u string) error {
	switch {
	case u == "":
		return ErrNoCloneURL
	case strings.HasPrefix(u, "-") || strings.IndexFunc(u, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0:
		return ErrBadCloneURL
	}
	return nil
}

func CloneDirName(u string) string {
	u = strings.TrimRight(u, "/")
	u = strings.TrimSuffix(u, cloneSuffix)
	if i := strings.LastIndexAny(u, "/:\\"); i >= 0 {
		u = u[i+1:]
	}
	if u == "" || u == "." || u == ".." {
		return cloneDefaultName
	}
	return u
}

func Clone(ctx context.Context, u, dest string, progress func(string)) error {
	if err := CheckCloneURL(u); err != nil {
		return err
	}
	if !filepath.IsAbs(dest) {
		return errors.New("git clone: destination must be an absolute path")
	}
	args := append(append([]string{"clone"}, progressArgs(progress)...), "--", u, dest)
	_, err := execGit(ctx, filepath.Dir(dest), call{progress: progress}, args...)
	return err
}
