package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type WhitespaceMode int

const (
	WhitespaceNone WhitespaceMode = iota
	WhitespaceTrim
	WhitespaceIgnore
	WhitespaceIgnoreBlank
)

const (
	flagIgnoreSpaceAtEOL = "--ignore-space-at-eol"
	flagIgnoreAllSpace   = "-w"
	flagIgnoreBlankLines = "--ignore-blank-lines"
	flagNoIndex          = "--no-index"
	flagEndOfOptions     = "--"
	noIndexTempPattern   = "editor-diff-"
	noIndexOldName       = "old"
	noIndexNewName       = "new"
	noIndexFileMode      = 0o600
	exitCodeDifferences  = 1
)

var whitespaceModes = []struct {
	mode  WhitespaceMode
	key   string
	flags []string
}{
	{WhitespaceNone, "none", nil},
	{WhitespaceTrim, "trim", []string{flagIgnoreSpaceAtEOL}},
	{WhitespaceIgnore, "ignore", []string{flagIgnoreAllSpace}},
	{WhitespaceIgnoreBlank, "ignoreBlankLines", []string{flagIgnoreAllSpace, flagIgnoreBlankLines}},
}

func (m WhitespaceMode) Flags() []string {
	for _, e := range whitespaceModes {
		if e.mode == m {
			return append([]string(nil), e.flags...)
		}
	}
	return nil
}

func (m WhitespaceMode) Key() string {
	for _, e := range whitespaceModes {
		if e.mode == m {
			return e.key
		}
	}
	return whitespaceModes[0].key
}

func ParseWhitespaceMode(key string) (WhitespaceMode, bool) {
	for _, e := range whitespaceModes {
		if e.key == key {
			return e.mode, true
		}
	}
	return WhitespaceNone, false
}

func DiffTexts(ctx context.Context, old, new string, m WhitespaceMode) ([]byte, error) {
	dir, err := os.MkdirTemp("", noIndexTempPattern)
	if err != nil {
		return nil, fmt.Errorf("git diff: temp dir: %w", err)
	}
	defer os.RemoveAll(dir)
	for name, text := range map[string]string{noIndexOldName: old, noIndexNewName: new} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), noIndexFileMode); err != nil {
			return nil, fmt.Errorf("git diff: write %s: %w", name, err)
		}
	}
	args := append([]string{"diff"}, diffFlags...)
	args = append(args, m.Flags()...)
	args = append(args, flagNoIndex, flagEndOfOptions, noIndexOldName, noIndexNewName)
	out, err := execGit(ctx, dir, readOnly, args...)
	var ge *Error
	if errors.As(err, &ge) && ge.ExitCode == exitCodeDifferences && len(out) > 0 {
		return out, nil
	}
	return out, err
}
