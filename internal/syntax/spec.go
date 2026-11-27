package syntax

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/egoist/mygo/ui/editor"
)

const (
	modeComment    = 1
	modeString     = 32
	modeMask       = 0xff
	depthShift     = 8
	depthMask      = 0xff
	maxOptionalLen = 6
	operatorBytes  = "+-*/%=<>!&|^~?:"
	punctBytes     = "()[]{},;."
)

type blockComment struct {
	open, close string
	nested      bool
}

type stringSpec struct {
	open, close string
	escape      byte
	multiline   bool
	optional    bool
}

type spec struct {
	lineComments []string
	blocks       []blockComment
	strings      []stringSpec
	words        map[string]editor.TokenKind
	prefixes     map[byte]editor.TokenKind
	foldCase     bool
	identStart   string
	identChars   string
	numberChars  string
	textEscape   byte
	capitalTypes bool
	calls        bool
	parenCalls   bool
	noNumbers    bool
}

type scan struct {
	*spec
	line  string
	toks  []editor.Token
	base  int
	pos   int
	mode  int
	depth int
}

func (s *spec) Tokenize(line string, state uint64, tokens []editor.Token) ([]editor.Token, uint64) {
	t := scan{spec: s, line: line, toks: tokens, base: len(tokens)}
	t.mode = s.validMode(int(state & modeMask))
	t.depth = int(state >> depthShift & depthMask)
	for i := 0; i < len(line); {
		switch {
		case t.mode >= modeString:
			i = t.inString(i)
		case t.mode >= modeComment:
			i = t.inComment(i)
		default:
			i = t.code(i)
		}
	}
	if t.mode >= modeString && !s.strings[t.mode-modeString].multiline {
		t.mode = 0
	}
	return t.toks, uint64(t.mode) | uint64(t.depth)<<depthShift
}

func (s *spec) validMode(mode int) int {
	switch {
	case mode >= modeString && mode-modeString < len(s.strings):
		return mode
	case mode >= modeComment && mode < modeString && mode-modeComment < len(s.blocks):
		return mode
	}
	return 0
}

func (t *scan) emit(end int, kind editor.TokenKind) int {
	if end <= t.pos {
		return end
	}
	t.pos = end
	if n := len(t.toks); n > t.base && t.toks[n-1].Kind == kind {
		t.toks[n-1].End = end
		return end
	}
	t.toks = append(t.toks, editor.Token{End: end, Kind: kind})
	return end
}

func (t *scan) inComment(i int) int {
	b := t.blocks[t.mode-modeComment]
	line := t.line
	j := i
	for j < len(line) {
		switch {
		case b.nested && strings.HasPrefix(line[j:], b.open):
			t.depth++
			j += len(b.open)
		case strings.HasPrefix(line[j:], b.close):
			j += len(b.close)
			if t.depth == 0 {
				t.mode = 0
				return t.emit(j, editor.TokenComment)
			}
			t.depth--
		default:
			j++
		}
	}
	return t.emit(len(line), editor.TokenComment)
}

func stringEnd(line string, from int, st stringSpec) (int, bool) {
	for j := from; j < len(line); {
		if st.escape != 0 && line[j] == st.escape {
			j += 2
			continue
		}
		if strings.HasPrefix(line[j:], st.close) {
			return j + len(st.close), true
		}
		j++
	}
	return len(line), false
}

func (t *scan) inString(i int) int {
	end, closed := stringEnd(t.line, i, t.strings[t.mode-modeString])
	if closed {
		t.mode = 0
	}
	return t.emit(end, editor.TokenString)
}

func letterAt(line string, i int) int {
	c := line[i]
	if c < utf8.RuneSelf {
		if isASCIILetter(c) {
			return 1
		}
		return 0
	}
	r, n := utf8.DecodeRuneInString(line[i:])
	if unicode.IsLetter(r) {
		return n
	}
	return 0
}

func (s *spec) identEnd(line string, i int) int {
	return identRunEnd(line, i, func(c byte) bool {
		return isDigit(c) || isASCIILetter(c) || strings.IndexByte(s.identChars, c) >= 0
	})
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func (t *scan) code(i int) int {
	line, c := t.line, t.line[i]
	rest := line[i:]
	if t.textEscape != 0 && c == t.textEscape && i+1 < len(line) && letterAt(line, i+1) == 0 {
		_, n := utf8.DecodeRuneInString(line[i+1:])
		return t.emit(i+1+n, editor.TokenText)
	}
	for k, b := range t.blocks {
		if strings.HasPrefix(rest, b.open) {
			t.mode, t.depth = modeComment+k, 0
			return t.emit(i+len(b.open), editor.TokenComment)
		}
	}
	for _, lc := range t.lineComments {
		if strings.HasPrefix(rest, lc) {
			return t.emit(len(line), editor.TokenComment)
		}
	}
	for k, st := range t.strings {
		if !strings.HasPrefix(rest, st.open) {
			continue
		}
		if !st.optional {
			t.mode = modeString + k
			return t.emit(i+len(st.open), editor.TokenString)
		}
		end, closed := stringEnd(line, i+len(st.open), st)
		if closed && end-i <= maxOptionalLen && !strings.ContainsAny(line[i:end], " \t") {
			return t.emit(end, editor.TokenString)
		}
	}
	if kind, ok := t.prefixes[c]; ok {
		j := i + 1
		for j < len(line) && line[j] == c {
			j++
		}
		if j < len(line) {
			if w := letterAt(line, j); w > 0 {
				return t.emit(t.identEnd(line, j+w), kind)
			}
		}
	}
	if w := letterAt(line, i); w > 0 || strings.IndexByte(t.identStart, c) >= 0 {
		if w == 0 {
			w = 1
		}
		end := t.identEnd(line, i+w)
		return t.emit(end, t.wordKind(line[i:end], end))
	}
	if isDigit(c) {
		end := t.numberEnd(i)
		if t.noNumbers {
			return t.emit(end, editor.TokenText)
		}
		return t.emit(end, editor.TokenNumber)
	}
	_, n := utf8.DecodeRuneInString(rest)
	switch {
	case strings.IndexByte(punctBytes, c) >= 0:
		return t.emit(i+n, editor.TokenPunctuation)
	case strings.IndexByte(operatorBytes, c) >= 0:
		return t.emit(i+n, editor.TokenOperator)
	}
	return t.emit(i+n, editor.TokenText)
}

func (t *scan) numberEnd(i int) int {
	line := t.line
	j := i + 1
	for j < len(line) {
		c := line[j]
		switch {
		case isDigit(c) || c == '_' || letterAt(line, j) == 1 && c < utf8.RuneSelf:
		case c == '.' && j+1 < len(line) && isDigit(line[j+1]):
		case strings.IndexByte(t.numberChars, c) >= 0:
		default:
			return j
		}
		j++
	}
	return j
}

func (t *scan) wordKind(word string, end int) editor.TokenKind {
	key := word
	if t.foldCase {
		key = strings.ToLower(word)
	}
	if kind, ok := t.words[key]; ok {
		return kind
	}
	switch {
	case t.calls && end < len(t.line) && t.line[end] == '(':
		return editor.TokenFunction
	case t.parenCalls && t.pos > 0 && t.line[t.pos-1] == '(':
		return editor.TokenFunction
	case t.capitalTypes:
		if r, _ := utf8.DecodeRuneInString(word); unicode.IsUpper(r) {
			return editor.TokenType
		}
	}
	return editor.TokenText
}
