package shellcmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/ZerGo0/editor/internal/fsutil"
)

const (
	Name       = "editor"
	marker     = "# Installed by the Editor app."
	scriptPerm = 0o755
	dirPerm    = 0o755
)

var ErrForeign = errors.New("another program already uses this path")

func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "bin", Name), nil
}

func Install(executable string) (string, error) {
	path, err := Path()
	if err != nil {
		return "", err
	}
	if err := checkOwned(path); err != nil {
		return path, err
	}
	if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
		return path, err
	}
	return path, fsutil.WriteFileAtomic(path, []byte(script(executable)), scriptPerm)
}

func Uninstall() (string, error) {
	path, err := Path()
	if err != nil {
		return "", err
	}
	if err := checkOwned(path); err != nil {
		return path, err
	}
	err = os.Remove(path)
	if errors.Is(err, fs.ErrNotExist) {
		return path, nil
	}
	return path, err
}

func Installed() bool {
	path, err := Path()
	if err != nil {
		return false
	}
	data, err := os.ReadFile(path)
	return err == nil && strings.Contains(string(data), marker)
}

func checkOwned(path string) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !strings.Contains(string(data), marker) {
		return ErrForeign
	}
	return nil
}

func script(executable string) string {
	return fmt.Sprintf("#!/bin/sh\n%s\nnohup %s \"$@\" >/dev/null 2>&1 &\n", marker, quote(executable))
}

func quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
