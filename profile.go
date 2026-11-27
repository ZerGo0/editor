package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
)

const (
	devAppName    = "Editor Dev"
	bundleExecDir = ".app/Contents/MacOS/"
)

var installRoots = []string{"/Applications"}

func applyProfile() {
	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
	}
	home, _ := os.UserHomeDir()
	if err != nil || !installed(exe, home) {
		mygo.App.SetName(devAppName)
	}
}

func installed(exe, home string) bool {
	if !strings.Contains(exe, bundleExecDir) {
		return false
	}
	roots := installRoots
	if home != "" {
		roots = append(roots[:len(roots):len(roots)], filepath.Join(home, "Applications"))
	}
	for _, root := range roots {
		if strings.HasPrefix(exe, root+string(filepath.Separator)) {
			return true
		}
	}
	return false
}
