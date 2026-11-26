package vscodeimport

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ZerGo0/editor/internal/fsutil"
	"github.com/ZerGo0/editor/internal/jsonc"
	"github.com/ZerGo0/editor/internal/keymap"
)

const (
	importComment = "// Imported from VS Code"
	entryIndent   = "\t"
	backupLayout  = "20060102-150405"
	backupSuffix  = ".backup-"
	filePerm      = 0o644
	arrayOpen     = '['
	arrayClose    = ']'
	trivia        = " \t\r\n"
)

func Append(data []byte, entries []keymap.Entry) ([]byte, error) {
	if len(entries) == 0 {
		return data, nil
	}
	block, err := render(entries)
	if err != nil {
		return nil, err
	}
	std := jsonc.Standard(data)
	trimmed := bytes.TrimRight(std, trivia)
	if len(bytes.TrimSpace(trimmed)) == 0 {
		out := bytes.Clone(bytes.TrimRight(data, trivia))
		if len(out) > 0 {
			out = append(out, '\n')
		}
		return append(out, string(arrayOpen)+block+"\n"+string(arrayClose)+"\n"...), nil
	}
	closeAt := len(trimmed) - 1
	if trimmed[closeAt] != arrayClose || bytes.TrimLeft(std, trivia)[0] != arrayOpen {
		return nil, errors.New("keybindings.json is not a JSON array")
	}
	last := bytes.LastIndexFunc(std[:closeAt], func(r rune) bool { return !strings.ContainsRune(trivia, r) })
	if last < 0 {
		return nil, errors.New("keybindings.json is not a JSON array")
	}
	insert := block
	if std[last] != arrayOpen {
		insert = "," + block
	}
	if !strings.ContainsRune(trivia, rune(data[last+1])) {
		insert += "\n"
	}
	out := make([]byte, 0, len(data)+len(insert))
	out = append(out, data[:last+1]...)
	out = append(out, insert...)
	return append(out, data[last+1:]...), nil
}

func render(entries []keymap.Entry) (string, error) {
	var b strings.Builder
	b.WriteString("\n" + entryIndent + importComment)
	for i, e := range entries {
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		enc.SetIndent(entryIndent, entryIndent)
		if err := enc.Encode(e); err != nil {
			return "", fmt.Errorf("entry %s: %w", e.Key, err)
		}
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString("\n" + entryIndent + strings.TrimRight(buf.String(), "\n"))
	}
	return b.String(), nil
}

func Merge(path string, data []byte, entries []keymap.Entry, now time.Time) (string, error) {
	out, err := Append(data, entries)
	if err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}
	if _, err := keymap.ParseEntries(out, path); err != nil {
		return "", err
	}
	backup := ""
	if len(bytes.TrimSpace(data)) > 0 {
		backup = path + backupSuffix + now.Format(backupLayout)
		if err := os.WriteFile(backup, data, filePerm); err != nil {
			return "", fmt.Errorf("back up %s: %w", path, err)
		}
	}
	if err := fsutil.WriteFileAtomic(path, out, filePerm); err != nil {
		return backup, fmt.Errorf("write %s: %w", path, err)
	}
	return backup, nil
}
