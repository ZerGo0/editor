package problems

import (
	"encoding/json"
	"errors"
	"path"
	"strings"

	"github.com/egoist/mygo/ui/editor"
)

const (
	jsonLanguage = "JSON"
	utf8BOM      = "\ufeff"
	vscodeDir    = ".vscode"

	unexpectedEndMessage = "unexpected end of JSON input"
)

var jsoncExtensions = map[string]bool{".jsonc": true, ".json5": true}

var jsoncFilenames = map[string]bool{
	"settings.json": true, "keybindings.json": true, "tasks.json": true, "launch.json": true,
	"extensions.json": true, "tsconfig.json": true, "jsconfig.json": true, "devcontainer.json": true,
}

func isJSONC(filename string) bool {
	slashed := strings.ReplaceAll(filename, "\\", "/")
	base := strings.ToLower(path.Base(slashed))
	if jsoncExtensions[path.Ext(base)] || jsoncFilenames[base] {
		return true
	}
	for _, dir := range strings.Split(path.Dir(slashed), "/") {
		if dir == vscodeDir {
			return true
		}
	}
	return false
}

func checkJSON(lang *editor.Language, filename, text string) []Problem {
	if lang == nil || lang.Name != jsonLanguage || isJSONC(filename) || strings.TrimSpace(text) == "" {
		return nil
	}
	body := strings.TrimPrefix(text, utf8BOM)
	bomLen := len(text) - len(body)
	var raw json.RawMessage
	err := json.Unmarshal([]byte(body), &raw)
	if err == nil {
		return nil
	}
	offset := len(body)
	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		offset = int(min(max(syntaxErr.Offset-1, 0), int64(len(body))))
		if syntaxErr.Error() == unexpectedEndMessage {
			offset = len(body)
		}
	}
	offset += bomLen
	before := text[:offset]
	line := strings.Count(before, "\n")
	col := offset - (strings.LastIndexByte(before, '\n') + 1)
	end := col
	if offset < len(text) && text[offset] != '\n' && text[offset] != '\r' {
		end++
	}
	return []Problem{{Line: line, Col: col, EndCol: end, Severity: Error, Message: err.Error(), Source: sourceJSON}}
}
