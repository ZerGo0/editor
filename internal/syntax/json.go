package syntax

import (
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui/editor"
)

const jsonCommentState = 1

var jsonConstants = set("true false null NaN Infinity")

type jsonLexer struct{}

func (jsonLexer) Tokenize(line string, state uint64, tokens []editor.Token) ([]editor.Token, uint64) {
	w := newWriter(tokens)
	i := 0
	if state&jsonCommentState != 0 {
		end := strings.Index(line, "*/")
		if end < 0 {
			w.emit(len(line), editor.TokenComment)
			return w.toks, jsonCommentState
		}
		i = w.emit(end+2, editor.TokenComment)
	}
	for i < len(line) {
		c := line[i]
		switch {
		case c == ' ' || c == '\t' || c == '\r':
			i = w.emit(i+1, editor.TokenText)
		case strings.HasPrefix(line[i:], "//"):
			i = w.emit(len(line), editor.TokenComment)
		case strings.HasPrefix(line[i:], "/*"):
			end := strings.Index(line[i+2:], "*/")
			if end < 0 {
				w.emit(len(line), editor.TokenComment)
				return w.toks, jsonCommentState
			}
			i = w.emit(i+end+4, editor.TokenComment)
		case c == '"' || c == '\'':
			i = jsonString(&w, line, i)
		case isDigit(c) || (c == '-' || c == '+' || c == '.') && i+1 < len(line) && (isDigit(line[i+1]) || line[i+1] == '.'):
			j := i + 1
			for j < len(line) && (isDigit(line[j]) || strings.IndexByte(".eE+-xX_abcdefABCDEF", line[j]) >= 0) {
				j++
			}
			i = w.emit(j, editor.TokenNumber)
		case isIdentStartByte(c) && c != '$' || c == '$':
			j := scanIdent(line, i)
			if j == i {
				_, n := utf8.DecodeRuneInString(line[i:])
				j = i + n
			}
			kind := editor.TokenVariable
			switch {
			case jsonConstants[line[i:j]]:
				kind = editor.TokenConstant
			case colonAhead(line, j):
				kind = editor.TokenProperty
			}
			i = w.emit(j, kind)
		case strings.IndexByte("{}[],:", c) >= 0:
			i = w.emit(i+1, editor.TokenPunctuation)
		default:
			_, n := utf8.DecodeRuneInString(line[i:])
			i = w.emit(i+n, editor.TokenText)
		}
	}
	return w.toks, 0
}

func colonAhead(line string, i int) bool {
	rest := strings.TrimLeft(line[i:], " \t")
	return strings.HasPrefix(rest, ":")
}

func jsonString(w *writer, line string, i int) int {
	q := line[i]
	j := i + 1
	for j < len(line) {
		switch line[j] {
		case '\\':
			end := min(len(line), escapeEnd(line, j))
			w.emit(j, editor.TokenString)
			j = end
			w.emit(j, editor.TokenEscape)
			continue
		case q:
			kind := editor.TokenString
			if colonAhead(line, j+1) {
				kind = editor.TokenProperty
			}
			return w.emit(j+1, kind)
		}
		j++
	}
	return w.emit(j, editor.TokenString)
}
