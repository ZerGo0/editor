package syntax

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/egoist/mygo/ui/editor"
)

const (
	goBlock = 1 + iota
	goStruct
	goIface
	goParams
	goParen
	goVarGroup
	goTypeGroup
)

const (
	goEntryBits  = 3
	goEntryMask  = 1<<goEntryBits - 1
	goEntries    = 6
	goStackBits  = goEntryBits * goEntries
	goModeBits   = 2
	goModeMask   = 1<<goModeBits - 1
	goModeCode   = 0
	goModeBlock  = 1
	goModeRaw    = 2
	goNoTypeRun  = -1
	goVerbFlags  = "+-# 0"
	goVerbLetter = "vTtbcdoOqxXUeEfFgGsp"
)

var (
	goKeywords  = set("break case chan const continue default defer else fallthrough for func go goto if import interface map package range return select struct switch type var")
	goTypeNames = set("any bool byte comparable complex64 complex128 error float32 float64 int int8 int16 int32 int64 rune string uint uint8 uint16 uint32 uint64 uintptr")
	goConstants = set("true false nil iota")
	goDirective = []string{"//go:", "//nolint", "//lint:", "//export ", "//line "}
)

type golang struct{}

type goScan struct {
	writer
	line      string
	stack     uint64
	mode      int
	prevWord  string
	prevDot   bool
	afterFunc bool
	declNames bool
	declType  bool
	caseLine  bool
	typeRun   int
}

func (golang) Tokenize(line string, state uint64, tokens []editor.Token) ([]editor.Token, uint64) {
	s := goScan{writer: newWriter(tokens), line: line, mode: int(state & goModeMask), stack: state >> goModeBits, typeRun: goNoTypeRun}
	switch s.top() {
	case goVarGroup, goTypeGroup, goStruct:
		s.declNames = s.mode == goModeCode
	}
	for i := 0; i < len(line); {
		switch s.mode {
		case goModeBlock:
			i = s.blockComment(i)
		case goModeRaw:
			i = s.rawString(i)
		default:
			i = s.code(i)
		}
	}
	return s.toks, uint64(s.mode) | s.stack<<goModeBits
}

func (s *goScan) top() uint64 { return s.stack & goEntryMask }

func (s *goScan) push(k uint64) { s.stack = (s.stack<<goEntryBits | k) & (1<<goStackBits - 1) }

func (s *goScan) pop() { s.stack >>= goEntryBits }

func (s *goScan) blockComment(i int) int {
	end, closed := blockCommentEnd(s.line, i)
	if closed {
		s.mode = goModeCode
	}
	return s.emit(end, editor.TokenComment)
}

func (s *goScan) simple(end int, kind editor.TokenKind) int {
	s.prevWord, s.prevDot = "", false
	return s.emit(end, kind)
}

func (s *goScan) code(i int) int {
	line := s.line
	c := line[i]
	switch {
	case c == ' ' || c == '\t' || c == '\r':
		return s.emit(i+1, editor.TokenText)
	case c == '/' && i+1 < len(line) && line[i+1] == '/':
		kind := editor.TokenComment
		for _, d := range goDirective {
			if strings.HasPrefix(line[i:], d) {
				kind = editor.TokenMeta
			}
		}
		return s.emit(len(line), kind)
	case c == '/' && i+1 < len(line) && line[i+1] == '*':
		s.mode = goModeBlock
		return s.blockComment(i + 2)
	case c == '"':
		return s.quoted(i)
	case c == '\'':
		return s.rune(i)
	case c == '`':
		return s.rawStart(i)
	case isDigit(c) || c == '.' && i+1 < len(line) && isDigit(line[i+1]):
		s.endTypeRun()
		return s.simple(goNumberEnd(line, i), editor.TokenNumber)
	case identStartAt(line, i) && c != '$':
		return s.word(i)
	case c >= utf8.RuneSelf:
		_, n := utf8.DecodeRuneInString(line[i:])
		return s.simple(i+n, editor.TokenText)
	}
	return s.punctuation(i)
}

func goNumberEnd(line string, i int) int {
	j := i
	if line[j] == '0' && j+1 < len(line) && strings.IndexByte("xXbBoO", line[j+1]) >= 0 {
		j += 2
	}
	for j < len(line) {
		c := line[j]
		switch {
		case isDigit(c) || c == '_' || c == '.' || c == 'i':
		case isIdentPartByte(c) && (c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F' || c == 'x' || c == 'p' || c == 'P'):
		case (c == '+' || c == '-') && (line[j-1] == 'e' || line[j-1] == 'E' || line[j-1] == 'p' || line[j-1] == 'P'):
		default:
			return j
		}
		j++
	}
	return j
}

func (s *goScan) endTypeRun() {
	if s.typeRun != goNoTypeRun && s.typeRun == 0 {
		s.typeRun = goNoTypeRun
	}
}

func (s *goScan) quoted(i int) int {
	line := s.line
	s.endTypeRun()
	j := i + 1
	for j < len(line) {
		switch line[j] {
		case '\\':
			s.emit(j, editor.TokenString)
			j = s.emit(min(len(line), escapeEnd(line, j)), editor.TokenEscape)
			continue
		case '%':
			if end := goVerbEnd(line, j); end > 0 {
				s.emit(j, editor.TokenString)
				j = s.emit(end, editor.TokenEscape)
				continue
			}
		case '"':
			return s.simple(j+1, editor.TokenString)
		}
		j++
	}
	return s.simple(j, editor.TokenString)
}

func goVerbEnd(line string, i int) int {
	j := i + 1
	if j < len(line) && line[j] == '%' {
		return j + 1
	}
	for j < len(line) && strings.IndexByte(goVerbFlags, line[j]) >= 0 {
		j++
	}
	for j < len(line) && (isDigit(line[j]) || line[j] == '*') {
		j++
	}
	if j < len(line) && line[j] == '.' {
		j++
		for j < len(line) && (isDigit(line[j]) || line[j] == '*') {
			j++
		}
	}
	if j < len(line) && strings.IndexByte(goVerbLetter, line[j]) >= 0 {
		return j + 1
	}
	return -1
}

func (s *goScan) rune(i int) int {
	line := s.line
	s.endTypeRun()
	j := i + 1
	if j < len(line) && line[j] == '\\' {
		s.emit(j, editor.TokenString)
		j = s.emit(min(len(line), escapeEnd(line, j)), editor.TokenEscape)
	} else if j < len(line) {
		_, n := utf8.DecodeRuneInString(line[j:])
		j += n
	}
	if j < len(line) && line[j] == '\'' {
		j++
	}
	return s.simple(j, editor.TokenString)
}

func (s *goScan) rawStart(i int) int {
	line := s.line
	s.endTypeRun()
	end := strings.IndexByte(line[i+1:], '`')
	if end < 0 {
		s.mode = goModeRaw
		return s.simple(len(line), editor.TokenString)
	}
	end += i + 1
	if looksLikeStructTag(line[i+1 : end]) {
		s.emit(i+1, editor.TokenString)
		s.structTag(i+1, end)
		return s.simple(end+1, editor.TokenString)
	}
	return s.simple(end+1, editor.TokenString)
}

func looksLikeStructTag(tag string) bool {
	tag = strings.TrimSpace(tag)
	colon := strings.Index(tag, ":\"")
	if colon <= 0 || !strings.HasSuffix(tag, "\"") {
		return false
	}
	for _, r := range tag[:colon] {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-' {
			return false
		}
	}
	return true
}

func (s *goScan) structTag(from, to int) {
	line := s.line
	for i := from; i < to; {
		for i < to && line[i] == ' ' {
			i++
		}
		colon := strings.IndexByte(line[i:to], ':')
		if colon < 0 {
			break
		}
		s.emit(i, editor.TokenString)
		s.emit(i+colon, editor.TokenAttribute)
		s.emit(i+colon+1, editor.TokenOperator)
		i += colon + 1
		if i < to && line[i] == '"' {
			end := strings.IndexByte(line[i+1:to], '"')
			if end < 0 {
				break
			}
			s.emit(i+1, editor.TokenString)
			s.emit(i+1+end, editor.TokenString)
			i += end + 2
		}
	}
}

func (s *goScan) rawString(i int) int {
	end := strings.IndexByte(s.line[i:], '`')
	if end < 0 {
		return s.emit(len(s.line), editor.TokenString)
	}
	s.mode = goModeCode
	return s.simple(i+end+1, editor.TokenString)
}

func (s *goScan) punctuation(i int) int {
	line := s.line
	c := line[i]
	if s.typeRun != goNoTypeRun && s.stopsTypeRun(c, i) {
		s.typeRun = goNoTypeRun
	}
	switch c {
	case '{':
		return s.openBrace(i)
	case '}':
		for k := s.top(); k == goParams || k == goParen || k == goVarGroup || k == goTypeGroup; k = s.top() {
			s.pop()
		}
		if k := s.top(); k == goBlock || k == goStruct || k == goIface {
			s.pop()
		}
		return s.simple(i+1, editor.TokenPunctuation)
	case '(':
		return s.openParen(i)
	case ')':
		return s.closeParen(i)
	case '[':
		if s.typeRun != goNoTypeRun {
			s.typeRun++
		} else if i+1 < len(line) && line[i+1] == ']' || s.afterFunc || s.declType {
			s.typeRun = 1
		}
		return s.simple(i+1, editor.TokenPunctuation)
	case ']':
		if s.typeRun > 0 {
			s.typeRun--
			if s.typeRun == 0 {
				s.typeRun = 0
			}
		}
		return s.simple(i+1, editor.TokenPunctuation)
	case ',', ';':
		return s.simple(i+1, editor.TokenPunctuation)
	case '.':
		if strings.HasPrefix(line[i:], "...") {
			return s.simple(i+3, editor.TokenOperator)
		}
		s.emit(i+1, editor.TokenPunctuation)
		s.prevDot = true
		s.prevWord = ""
		return i + 1
	case ':':
		if i+1 < len(line) && line[i+1] == '=' {
			s.declNames = false
			return s.simple(i+2, editor.TokenOperator)
		}
	case '=':
		if s.typeRun != goNoTypeRun {
			s.typeRun = goNoTypeRun
		}
		if s.declType && i+1 < len(line) && line[i+1] != '=' {
			s.declType = false
			s.typeRun = 0
		}
		s.declNames = false
	}
	if strings.HasPrefix(line[i:], "<-") {
		return s.simple(i+2, editor.TokenOperator)
	}
	return s.simple(i+opLenGo(line[i:]), editor.TokenOperator)
}

func opLenGo(s string) int {
	for _, o := range goOperators {
		if strings.HasPrefix(s, o) {
			return len(o)
		}
	}
	return 1
}

var goOperators = []string{"&^=", "<<=", ">>=", "&&", "||", "==", "!=", "<=", ">=", "<<", ">>", "&^", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=", "++", "--", ":="}

func (s *goScan) stopsTypeRun(c byte, i int) bool {
	if s.typeRun > 0 {
		return false
	}
	switch c {
	case ',', ')', '=', ';', '}', ':':
		return true
	case '{':
		return s.prevWord != "struct" && s.prevWord != "interface"
	}
	return false
}

func (s *goScan) openBrace(i int) int {
	kind := uint64(goBlock)
	switch s.prevWord {
	case "struct":
		kind = goStruct
	case "interface":
		kind = goIface
	}
	s.push(kind)
	s.afterFunc, s.declType, s.declNames = false, false, false
	s.typeRun = goNoTypeRun
	return s.simple(i+1, editor.TokenPunctuation)
}

func (s *goScan) openParen(i int) int {
	line := s.line
	switch {
	case s.afterFunc || s.top() == goIface:
		s.afterFunc = false
		s.push(goParams)
	case s.prevDot || strings.HasSuffix(line[:i], ".") || s.prevWord == "":
		s.push(goParen)
	case s.prevWord == "var" || s.prevWord == "const":
		s.push(goVarGroup)
	case s.prevWord == "type":
		s.push(goTypeGroup)
	default:
		s.push(goParen)
	}
	if s.top() == goParen && i > 0 && line[i-1] == '.' {
		s.typeRun = 0
	} else if s.typeRun != goNoTypeRun && s.top() == goParen {
		s.typeRun++
	}
	s.typeRun = min(s.typeRun, 8)
	if s.top() == goParams || s.top() == goVarGroup || s.top() == goTypeGroup {
		s.typeRun = goNoTypeRun
	}
	return s.simple(i+1, editor.TokenPunctuation)
}

func (s *goScan) closeParen(i int) int {
	closed := s.top()
	if closed == goParams || closed == goParen || closed == goVarGroup || closed == goTypeGroup {
		s.pop()
	}
	if s.typeRun > 0 {
		s.typeRun--
	}
	if closed == goParams && s.methodNameAhead(i+1) {
		s.afterFunc = true
	} else if closed == goParams {
		s.typeRun = 0
	} else if closed == goParen && s.typeRun == 0 {
		s.typeRun = goNoTypeRun
	}
	return s.simple(i+1, editor.TokenPunctuation)
}

func (s *goScan) methodNameAhead(i int) bool {
	rest := strings.TrimLeft(s.line[i:], " \t")
	if rest == "" || !isIdentStartByte(rest[0]) {
		return false
	}
	end := 0
	for end < len(rest) && isIdentPartByte(rest[end]) {
		end++
	}
	return end < len(rest) && (rest[end] == '(' || rest[end] == '[') && !goTypeNames[rest[:end]] && rest[:end] != "func" && rest[:end] != "map" && rest[:end] != "chan"
}

func isTypeStart(c, c2 byte) bool {
	return isIdentStartByte(c) || c == '*' || c == '[' || c == '<' && c2 == '-' || c == '.' && c2 == '.' || c == '('
}

func (s *goScan) word(i int) int {
	line := s.line
	end := scanIdent(line, i)
	w := line[i:end]
	nx, nx2 := nextBytes(s.line, end)
	prev, prevDot := s.prevWord, s.prevDot
	s.prevWord, s.prevDot = w, false
	top := s.top()
	if prevDot {
		s.prevWord = ""
		kind := editor.TokenProperty
		switch {
		case s.typeRun != goNoTypeRun:
			kind = editor.TokenType
		case nx == '(':
			kind = editor.TokenFunction
		}
		return s.emit(end, kind)
	}
	if goKeywords[w] {
		s.keyword(w)
		return s.emit(end, editor.TokenKeyword)
	}
	if prev == "package" {
		return s.emit(end, editor.TokenType)
	}
	if s.typeRun != goNoTypeRun {
		kind := editor.TokenType
		if nx == '.' {
			s.typeRun = 0
		}
		if w == "_" {
			kind = editor.TokenVariable
		}
		return s.emit(end, kind)
	}
	switch {
	case goConstants[w]:
		return s.emit(end, editor.TokenConstant)
	case prev == "type" || s.afterFunc && (nx == '(' || nx == '['):
		kind := editor.TokenType
		if prev != "type" {
			kind = editor.TokenFunction
		} else {
			s.declType = false
			s.typeRun = 0
			if nx == '=' {
				s.typeRun = goNoTypeRun
				s.declType = true
			}
		}
		return s.emit(end, kind)
	case s.declNames || top == goParams:
		if kind, ok := s.declaration(w, end, nx, nx2, top); ok {
			return s.emit(end, kind)
		}
	}
	switch {
	case top == goIface:
		kind := editor.TokenType
		if nx == '(' {
			kind = editor.TokenFunction
		}
		return s.emit(end, kind)
	case nx == '(':
		if goTypeNames[w] {
			return s.emit(end, editor.TokenType)
		}
		return s.emit(end, editor.TokenFunction)
	case goTypeNames[w]:
		return s.emit(end, editor.TokenType)
	case nx == '{' && end < len(line) && line[end] == '{' && prev != "range" && prev != "if" && prev != "for" && prev != "switch":
		return s.emit(end, editor.TokenType)
	case nx == ':' && nx2 != '=' && !s.caseLine && (top == goBlock || top == goParen):
		return s.emit(end, editor.TokenProperty)
	}
	return s.emit(end, goIdentKind(w))
}

func goIdentKind(w string) editor.TokenKind {
	if len(w) > 1 && w == strings.ToUpper(w) {
		return editor.TokenConstant
	}
	return editor.TokenVariable
}

func (s *goScan) declaration(w string, end int, nx, nx2 byte, top uint64) (editor.TokenKind, bool) {
	name := editor.TokenVariable
	switch top {
	case goStruct:
		name = editor.TokenProperty
	case goTypeGroup:
		name = editor.TokenType
	}
	switch {
	case isTypeStart(nx, nx2) && nx != '(' && !(nx == '[' && end < len(s.line) && s.line[end] == '['):
		s.declNames = false
		s.typeRun = 0
		return name, true
	case nx == ',':
		return name, true
	case top == goParams:
		if goTypeNames[w] {
			return editor.TokenType, true
		}
		if nx == ')' || nx == 0 {
			r, _ := utf8.DecodeRuneInString(w)
			if unicode.IsUpper(r) {
				return editor.TokenType, true
			}
		}
		return editor.TokenVariable, true
	case top == goStruct && (nx == 0 || nx == '`' || nx == '/' || nx == '.'):
		s.typeRun = 0
		if nx == '.' {
			return editor.TokenType, true
		}
		return editor.TokenType, true
	case s.declNames && (nx == '=' || nx == 0 || nx == ':'):
		s.declNames = false
		return name, true
	}
	return 0, false
}

func (s *goScan) keyword(w string) {
	switch w {
	case "func":
		s.afterFunc = true
	case "type":
		s.declType = true
	case "var", "const":
		s.declNames = true
	case "case":
		s.caseLine = true
	case "map", "chan":
		if s.typeRun == goNoTypeRun {
			s.typeRun = 0
		}
	}
}
