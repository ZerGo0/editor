package syntax

import (
	"strings"

	"github.com/egoist/mygo/ui/editor"
)

const (
	rstDirectiveOpen  = ".. "
	rstDirectiveClose = "::"
	rstLiteralMark    = "``"
	rstUnderlineChars = "=-~^\"'`#*+:._<>"
	rstUnderlineMin   = 3
)

var latex = &editor.Language{
	Name: "LaTeX", Extensions: []string{".tex", ".sty", ".cls", ".ltx", ".bib"},
	LineComment: "%", Brackets: brackets, AutoClose: autoClose(), TabSize: 2,
	IncreaseIndent: re(`\\begin\{`),
	DecreaseIndent: re(`^\s*\\end\{`),
	Tokenizer: &spec{
		lineComments: percent, prefixes: map[byte]editor.TokenKind{'\\': editor.TokenKeyword},
		textEscape: '\\', noNumbers: true,
	},
}

var restructuredText = &editor.Language{
	Name: "reStructuredText", Extensions: []string{".rst", ".rest"},
	LineComment: ".. ", TabSize: 3,
	Tokenizer: editor.TokenizerFunc(tokenizeRST),
}

func tokenizeRST(line string, state uint64, tokens []editor.Token) ([]editor.Token, uint64) {
	t := scan{line: line, toks: tokens, base: len(tokens)}
	trimmed := strings.TrimLeft(line, " \t")
	indent := len(line) - len(trimmed)
	t.emit(indent, editor.TokenText)
	switch {
	case strings.HasPrefix(trimmed, rstDirectiveOpen):
		name, _, found := strings.Cut(trimmed[len(rstDirectiveOpen):], rstDirectiveClose)
		if found && name != "" && !strings.ContainsAny(name, " \t") {
			t.emit(indent+len(rstDirectiveOpen)+len(name)+len(rstDirectiveClose), editor.TokenMeta)
			t.emit(len(line), editor.TokenText)
		} else {
			t.emit(len(line), editor.TokenComment)
		}
	case isUnderline(trimmed):
		t.emit(len(line), editor.TokenHeading)
	default:
		for i := indent; i < len(line); {
			open := strings.Index(line[i:], rstLiteralMark)
			if open < 0 {
				break
			}
			closeAt := strings.Index(line[i+open+len(rstLiteralMark):], rstLiteralMark)
			if closeAt < 0 {
				break
			}
			end := i + open + len(rstLiteralMark) + closeAt + len(rstLiteralMark)
			t.emit(i+open, editor.TokenText)
			t.emit(end, editor.TokenCode)
			i = end
		}
		t.emit(len(line), editor.TokenText)
	}
	return t.toks, 0
}

func isUnderline(s string) bool {
	if len(s) < rstUnderlineMin || strings.IndexByte(rstUnderlineChars, s[0]) < 0 {
		return false
	}
	return strings.Count(s, s[:1]) == len(s)
}
