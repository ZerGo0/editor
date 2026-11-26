package vscodeimport

import (
	"os"
	"path/filepath"
)

const (
	appSupportDir   = "Library/Application Support"
	userDir         = "User"
	keybindingsFile = "keybindings.json"
	settingsFile    = "settings.json"
)

var products = []string{"Code", "Code - Insiders", "VSCodium"}

type Source struct {
	Product string
	Dir     string
}

func (s Source) Keybindings() string {
	return filepath.Join(s.Dir, keybindingsFile)
}

func (s Source) Settings() string {
	return filepath.Join(s.Dir, settingsFile)
}

func Sources(home string) []Source {
	out := make([]Source, len(products))
	for i, p := range products {
		out[i] = Source{Product: p, Dir: filepath.Join(home, appSupportDir, p, userDir)}
	}
	return out
}

func Find(home string) (Source, bool) {
	for _, s := range Sources(home) {
		if info, err := os.Stat(s.Keybindings()); err == nil && info.Mode().IsRegular() {
			return s, true
		}
	}
	return Source{}, false
}
