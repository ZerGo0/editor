package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const loginShellTimeout = 3 * time.Second

var wellKnownBinDirs = []string{"/opt/homebrew/bin", "/usr/local/bin"}

func extendPath() {
	addPath(wellKnownBinDirs)
	go func() {
		shell := os.Getenv("SHELL")
		if shell == "" || !filepath.IsAbs(shell) {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), loginShellTimeout)
		defer cancel()
		out, err := exec.CommandContext(ctx, shell, "-l", "-c", "printf %s \"$PATH\"").Output()
		if err != nil {
			return
		}
		addPath(strings.Split(strings.TrimSpace(string(out)), string(os.PathListSeparator)))
	}()
}

func addPath(dirs []string) {
	current := filepath.SplitList(os.Getenv("PATH"))
	for _, d := range dirs {
		if filepath.IsAbs(d) && !slices.Contains(current, d) {
			current = append(current, d)
		}
	}
	os.Setenv("PATH", strings.Join(current, string(os.PathListSeparator)))
}
