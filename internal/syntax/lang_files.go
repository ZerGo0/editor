package syntax

import (
	"strings"

	"github.com/egoist/mygo/ui/editor"
)

var (
	goModDirectives = set("module go toolchain godebug require replace exclude retract use ignore tool")
	gettextKeywords = set("msgctxt msgid msgid_plural msgstr")
	gettextComments = map[byte]editor.TokenKind{':': editor.TokenMeta, ',': editor.TokenKeyword, '|': editor.TokenMeta}
)

const (
	goSumHashPrefix  = "h1:"
	goModVersionRune = 'v'
	gettextMarker    = '#'
)

type goModLexer struct{}

func (goModLexer) Tokenize(line string, state uint64, tokens []editor.Token) ([]editor.Token, uint64) {
	w := newWriter(tokens)
	first := true
	for i := 0; i < len(line); {
		c := line[i]
		switch {
		case c == ' ' || c == '\t' || c == '\r':
			i = w.emit(i+1, editor.TokenText)
		case strings.HasPrefix(line[i:], "//"):
			i = w.emit(len(line), editor.TokenComment)
		case c == '(' || c == ')':
			i = w.emit(i+1, editor.TokenPunctuation)
		case strings.HasPrefix(line[i:], "=>"):
			i = w.emit(i+2, editor.TokenOperator)
		case c == '"' || c == '`':
			end := len(line)
			if k := strings.IndexByte(line[i+1:], c); k >= 0 {
				end = i + k + 2
			}
			i = w.emit(end, editor.TokenString)
		default:
			j := i
			for j < len(line) && strings.IndexByte(" \t\r()", line[j]) < 0 {
				j++
			}
			i = w.emit(j, goModFieldKind(line[i:j], first))
			first = false
		}
	}
	return w.toks, 0
}

func goModFieldKind(field string, first bool) editor.TokenKind {
	switch {
	case first && goModDirectives[field]:
		return editor.TokenKeyword
	case strings.HasPrefix(field, goSumHashPrefix):
		return editor.TokenString
	case len(field) > 1 && field[0] == goModVersionRune && isDigit(field[1]) || field == "=>":
		return editor.TokenNumber
	case len(field) > 0 && isDigit(field[0]):
		return editor.TokenNumber
	}
	return editor.TokenVariable
}

type gettextLexer struct{}

func (gettextLexer) Tokenize(line string, state uint64, tokens []editor.Token) ([]editor.Token, uint64) {
	w := newWriter(tokens)
	trimmed := strings.TrimLeft(line, " \t")
	indent := len(line) - len(trimmed)
	w.emit(indent, editor.TokenText)
	if trimmed == "" {
		return w.toks, 0
	}
	if trimmed[0] == gettextMarker {
		kind := editor.TokenComment
		if len(trimmed) > 1 {
			if k, ok := gettextComments[trimmed[1]]; ok {
				kind = k
			}
		}
		w.emit(len(line), kind)
		return w.toks, 0
	}
	i := indent
	if trimmed[0] != '"' {
		j := i
		for j < len(line) && line[j] != ' ' && line[j] != '\t' && line[j] != '[' {
			j++
		}
		kind := editor.TokenText
		if gettextKeywords[line[i:j]] {
			kind = editor.TokenKeyword
		}
		w.emit(j, kind)
		i = j
		if i < len(line) && line[i] == '[' {
			end := strings.IndexByte(line[i:], ']')
			if end > 0 {
				w.emit(i+end+1, editor.TokenNumber)
				i += end + 1
			}
		}
	}
	gettextString(&w, line, i)
	return w.toks, 0
}

func gettextString(w *writer, line string, i int) {
	for i < len(line) {
		c := line[i]
		switch {
		case c == '\\' && i+1 < len(line):
			w.emit(i, editor.TokenString)
			i = w.emit(i+2, editor.TokenEscape)
		case c == '%' && i+1 < len(line) && (isIdentPartByte(line[i+1]) || line[i+1] == '%' || line[i+1] == '('):
			w.emit(i, editor.TokenString)
			end := i + 2
			if line[i+1] == '(' {
				if close := strings.IndexByte(line[i:], ')'); close > 0 {
					end = i + close + 2
				}
			}
			i = w.emit(min(end, len(line)), editor.TokenEscape)
		case c == '{' && strings.IndexByte(line[i:], '}') > 0:
			w.emit(i, editor.TokenString)
			i = w.emit(i+strings.IndexByte(line[i:], '}')+1, editor.TokenEscape)
		case c == '"':
			i = w.emit(i+1, editor.TokenString)
		case c == ' ' || c == '\t':
			i = w.emit(i+1, editor.TokenText)
		default:
			i = w.emit(i+1, editor.TokenString)
		}
	}
}
