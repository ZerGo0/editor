package references

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/egoist/mygo/ui/editor"
)

var skippedKinds = map[editor.TokenKind]bool{
	editor.TokenComment: true,
	editor.TokenString:  true,
	editor.TokenRegexp:  true,
}

var memberOperators = []string{"?.", "->", "::", "."}

func memberAccess(line string, start int) (op int, ok bool) {
	before := strings.TrimRight(line[:start], " \t")
	for _, operator := range memberOperators {
		if !strings.HasSuffix(before, operator) {
			continue
		}
		op = len(before) - len(operator)
		head := before[:op]
		switch operator {
		case ".":
			return op, !strings.HasSuffix(head, ".")
		case "::":
			r, _ := utf8.DecodeLastRuneInString(head)
			return op, isWordRune(r) || r == '>'
		}
		return op, true
	}
	return 0, false
}

func isWordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func wholeWord(line string, start, end int) bool {
	if start > 0 {
		if r, _ := utf8.DecodeLastRuneInString(line[:start]); isWordRune(r) {
			return false
		}
	}
	if end < len(line) {
		if r, _ := utf8.DecodeRuneInString(line[end:]); isWordRune(r) {
			return false
		}
	}
	return true
}

func splitLines(text string) []string {
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSuffix(l, "\r")
	}
	return lines
}

func lineTokens(lang *editor.Language, lines []string, wanted map[int]bool) map[int][]editor.Token {
	if lang == nil || lang.Tokenizer == nil {
		return nil
	}
	out := make(map[int][]editor.Token, len(wanted))
	var buf []editor.Token
	var state uint64
	last := 0
	for line := range wanted {
		last = max(last, line)
	}
	for i := 0; i <= last && i < len(lines); i++ {
		buf, state = lang.Tokenizer.Tokenize(lines[i], state, buf[:0])
		if wanted[i] {
			out[i] = append([]editor.Token(nil), buf...)
		}
	}
	return out
}

func inCodeAt(tokens []editor.Token, col int) bool {
	for _, t := range tokens {
		if col < t.End {
			return !skippedKinds[t.Kind]
		}
	}
	return true
}
