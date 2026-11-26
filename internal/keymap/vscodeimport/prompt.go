package vscodeimport

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ZerGo0/editor/internal/jsonc"
	"github.com/ZerGo0/editor/internal/keymap"
)

const (
	PromptStateFile = "vscode-keybindings-import"
	stateDirPerm    = 0o755
)

type Choice string

const (
	ChoiceImport Choice = "import"
	ChoiceNotNow Choice = "not-now"
)

func ShouldOffer(home, statePath, keybindingsPath string) (Source, bool) {
	if _, err := os.Stat(statePath); !errors.Is(err, os.ErrNotExist) {
		return Source{}, false
	}
	src, ok := Find(home)
	if !ok || !isEmpty(keybindingsPath) {
		return Source{}, false
	}
	return src, true
}

func isEmpty(path string) bool {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return true
	}
	if err != nil {
		return false
	}
	if len(bytes.TrimSpace(jsonc.Standard(data))) == 0 {
		return true
	}
	entries, err := keymap.ParseEntries(data, path)
	return err == nil && len(entries) == 0
}

func Remember(statePath string, c Choice) error {
	if err := os.MkdirAll(filepath.Dir(statePath), stateDirPerm); err != nil {
		return fmt.Errorf("save the import choice: %w", err)
	}
	if err := os.WriteFile(statePath, []byte(string(c)+"\n"), filePerm); err != nil {
		return fmt.Errorf("save the import choice: %w", err)
	}
	return nil
}
