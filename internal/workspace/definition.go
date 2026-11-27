package workspace

import (
	"fmt"
	"unicode"
	"unicode/utf8"

	"github.com/ZerGo0/editor/internal/search/finder"
	"github.com/ZerGo0/editor/internal/search/symbols"
	"github.com/egoist/mygo/ui/editor"
)

const workspaceSymbolPrefix = "#"

type definitionHit struct {
	path string
	line int
	col  int
}

func (w *Workspace) goToDefinition(st *editor.State) {
	pos := st.CursorPos()
	word := wordAround(st.Buffer.Line(pos.Line), pos.Col)
	if word == "" {
		return
	}
	var hits []definitionHit
	w.search.symbols.All(func(path string, s symbols.Symbol) bool {
		if s.Name == word {
			hits = append(hits, definitionHit{path, s.Line, s.Col})
		}
		return true
	})
	switch len(hits) {
	case 0:
		w.setNotice(fmt.Sprintf("No definition found for '%s'", word))
	case 1:
		w.OpenAt(hits[0].path, hits[0].line, hits[0].col)
	default:
		w.openSearch(finder.TabFiles, workspaceSymbolPrefix+word)
	}
}

func wordAround(line string, col int) string {
	col = max(0, min(col, len(line)))
	start, end := col, col
	for start > 0 {
		r, size := utf8.DecodeLastRuneInString(line[:start])
		if !isWordRune(r) {
			break
		}
		start -= size
	}
	for end < len(line) {
		r, size := utf8.DecodeRuneInString(line[end:])
		if !isWordRune(r) {
			break
		}
		end += size
	}
	return line[start:end]
}

func isWordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}
