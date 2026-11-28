package keymap

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ZerGo0/editor/internal/jsonc"
)

func describeEntries(data []byte, name string, err error) jsonc.Problem {
	f := jsonc.Describe(err)
	file := filepath.Base(name)
	if f.Syntax {
		return jsonc.At(data, f.Offset, file+": "+f.Sentence())
	}
	index, field, ok := entryField(f.Field)
	if !ok {
		return jsonc.At(data, f.Offset-1, fmt.Sprintf("%s: %s.", file, f.Message))
	}
	offset := f.Offset
	if offsets := jsonc.ElementOffsets(data); index < len(offsets) {
		offset = offsets[index]
	}
	text := fmt.Sprintf("%s, entry %d must be %s.", file, index+1, f.Expected)
	if field != "" {
		text = fmt.Sprintf("%s, entry %d: %q must be %s.", file, index+1, field, f.Expected)
	}
	return jsonc.At(data, offset, text)
}

func entryField(path string) (index int, field string, ok bool) {
	head, rest, _ := strings.Cut(path, ".")
	index, err := strconv.Atoi(head)
	return index, rest, err == nil
}

func entryLocator(data []byte) func(int) (int, int) {
	offsets := jsonc.ElementOffsets(data)
	return func(i int) (int, int) {
		if i >= len(offsets) {
			return jsonc.NoLine, 0
		}
		return jsonc.Position(data, offsets[i])
	}
}
