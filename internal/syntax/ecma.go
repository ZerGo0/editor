package syntax

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/egoist/mygo/ui/editor"
)

const (
	entBlock = 1 + iota
	entObj
	entParen
	entTBrace
	entTParen
	entTAngle
	entInterp
	entTemplate
	entTernary
	entAnn
	entRet
	entAs
	entAlias
	entJSXTag
	entJSXChild
	entCase
)

const (
	ecmaEntryBits  = 4
	ecmaEntryMask  = 1<<ecmaEntryBits - 1
	ecmaEntries    = 8
	ecmaStackMask  = 1<<(ecmaEntryBits*ecmaEntries) - 1
	ecmaCommentBit = 1 << (ecmaEntryBits * ecmaEntries)
	ecmaPrevShift  = ecmaEntryBits*ecmaEntries + 1
	ecmaPrevMask   = 3
	ecmaStateBits  = ecmaPrevShift + 2

	pvStatement = 0
	pvOperand   = 1
	pvValue     = 2

	genericLookahead = 160
)

var (
	ecmaHardKeywords = set("break case catch class const continue debugger default delete do else enum export extends finally for function if import in instanceof interface let new return super switch this throw try typeof var void while with yield await implements public private protected static readonly constructor")
	ecmaSoftKeywords = set("abstract accessor as asserts async declare from get infer is keyof namespace module of out override satisfies set type unique using global")
	ecmaTypeKeywords = set("keyof typeof infer is asserts readonly unique extends implements in out new as abstract")
	ecmaConstants    = set("true false null undefined NaN Infinity")
	ecmaGlobals      = set("console window document globalThis process navigator localStorage sessionStorage location history crypto performance fetch")
	ecmaRunes        = set("$state $derived $props $effect $bindable $inspect $host")
	ecmaStatementKw  = set("else do try finally static")
	ecmaDeclKw       = set("class interface enum namespace module function type import export")
	ecmaOperators    = []string{">>>=", "===", "!==", "**=", "<<=", ">>=", ">>>", "&&=", "||=", "??=", "=>", "==", "!=", "<=", ">=", "&&", "||", "??", "?.", "++", "--", "**", "<<", ">>", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^="}
	arrowAhead       = regexp.MustCompile("^\\s*(async\\s+)?(function\\b|\\([^()]*\\)\\s*(:[^=]+)?=>|[A-Za-z_$][\\w$]*\\s*=>)")
)

func set(list string) map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(list) {
		m[w] = true
	}
	return m
}

type ecma struct {
	jsx bool
}

type ecmaScan struct {
	*ecma
	writer
	line     string
	stack    uint64
	comment  bool
	pv       int
	last     byte
	prevDot  bool
	prevWord string
	decl     string
	stopAt   bool
	stopped  bool
}

func (e *ecma) Tokenize(line string, state uint64, tokens []editor.Token) ([]editor.Token, uint64) {
	s, _ := e.scan(line, state, tokens, false)
	return s.toks, s.state()
}

func (e *ecma) expr(line string, state uint64, tokens []editor.Token) ([]editor.Token, int, uint64, bool) {
	s, end := e.scan(line, state, tokens, true)
	return s.toks, end, s.state(), s.stopped
}

func (e *ecma) scan(line string, state uint64, tokens []editor.Token, stop bool) (*ecmaScan, int) {
	s := &ecmaScan{ecma: e, writer: newWriter(tokens), line: line, stack: state & ecmaStackMask,
		comment: state&ecmaCommentBit != 0, pv: int(state >> ecmaPrevShift & ecmaPrevMask), stopAt: stop}
	return s, s.run()
}

func (s *ecmaScan) state() uint64 {
	v := s.stack | uint64(s.pv)<<ecmaPrevShift
	if s.comment {
		v |= ecmaCommentBit
	}
	return v
}

func (s *ecmaScan) top() uint64 { return s.stack & ecmaEntryMask }

func (s *ecmaScan) push(k uint64) { s.stack = (s.stack<<ecmaEntryBits | k) & ecmaStackMask }

func (s *ecmaScan) pop() { s.stack >>= ecmaEntryBits }

func isSoftEntry(k uint64) bool {
	switch k {
	case entTernary, entCase, entAnn, entRet, entAs, entAlias:
		return true
	}
	return false
}

func isTypeEntry(k uint64) bool {
	switch k {
	case entTParen, entTAngle, entAnn, entRet, entAs, entAlias:
		return true
	}
	return false
}

func (s *ecmaScan) inType() bool {
	st := s.stack
	for st&ecmaEntryMask == entTernary {
		st >>= ecmaEntryBits
	}
	return isTypeEntry(st & ecmaEntryMask)
}

func (s *ecmaScan) skipSoft() uint64 {
	st := s.stack
	for st != 0 && isSoftEntry(st&ecmaEntryMask) {
		st >>= ecmaEntryBits
	}
	return st
}

func (s *ecmaScan) closeTo(kinds ...uint64) (uint64, bool) {
	st := s.skipSoft()
	k := st & ecmaEntryMask
	for _, w := range kinds {
		if k == w {
			s.stack = st >> ecmaEntryBits
			return k, true
		}
	}
	return 0, false
}

func (s *ecmaScan) tok(end int, kind editor.TokenKind, pv int, last byte) int {
	s.pv, s.last, s.prevDot, s.prevWord = pv, last, false, ""
	return s.emit(end, kind)
}

func (s *ecmaScan) firstByte() byte {
	for i := 0; i < len(s.line); i++ {
		if c := s.line[i]; c != ' ' && c != '\t' && c != '\r' {
			return c
		}
	}
	return 0
}

func (s *ecmaScan) run() int {
	n := len(s.line)
	i := 0
	if s.comment {
		i = s.blockComment(0)
	}
	if s.pv != pvOperand && i < n {
		first := s.firstByte()
		for first != 0 && strings.IndexByte("|&?=", first) < 0 {
			k := s.top()
			if k != entAnn && k != entRet && k != entAs && k != entAlias {
				break
			}
			s.pop()
		}
	}
	for i < n && !s.stopped {
		switch s.top() {
		case entTemplate:
			i = s.template(i)
		case entJSXChild:
			i = s.jsxChild(i)
		case entJSXTag:
			i = s.jsxTag(i)
		default:
			i = s.code(i)
		}
	}
	return i
}

func (s *ecmaScan) blockComment(i int) int {
	end, closed := blockCommentEnd(s.line, i)
	s.comment = !closed
	return s.emit(end, editor.TokenComment)
}

func isIdentStartByte(c byte) bool {
	return c == '_' || c == '$' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= utf8.RuneSelf
}

func identStartAt(line string, i int) bool {
	c := line[i]
	if c < utf8.RuneSelf {
		return isIdentStartByte(c)
	}
	r, _ := utf8.DecodeRuneInString(line[i:])
	return unicode.IsLetter(r)
}

func isIdentPartByte(c byte) bool { return isIdentStartByte(c) || isDigit(c) }

func (s *ecmaScan) identEnd(i int) int { return scanIdent(s.line, i) }

func scanIdent(line string, i int) int { return identRunEnd(line, i, isIdentPartByte) }

func (s *ecmaScan) code(i int) int {
	line := s.line
	c := line[i]
	switch {
	case c == ' ' || c == '\t' || c == '\r':
		return s.emit(i+1, editor.TokenText)
	case c == '/':
		if i+1 < len(line) {
			switch line[i+1] {
			case '/':
				return s.emit(len(line), editor.TokenComment)
			case '*':
				s.comment = true
				return s.blockComment(i + 2)
			}
		}
		if s.pv != pvValue && !s.inType() {
			if end := regexEnd(line, i); end > 0 {
				return s.tok(end, editor.TokenRegexp, pvValue, 0)
			}
		}
		return s.operator(i)
	case c == '"' || c == '\'':
		return s.str(i)
	case c == '`':
		s.push(entTemplate)
		return s.tok(i+1, editor.TokenString, pvValue, 0)
	case isDigit(c) || c == '.' && i+1 < len(line) && isDigit(line[i+1]) && s.pv != pvValue:
		return s.tok(s.numberEnd(i), editor.TokenNumber, pvValue, 0)
	case c == '@' && i+1 < len(line) && isIdentStartByte(line[i+1]):
		j := s.identEnd(i + 1)
		for j+1 < len(line) && line[j] == '.' && isIdentStartByte(line[j+1]) {
			j = s.identEnd(j + 1)
		}
		return s.tok(j, editor.TokenMeta, pvValue, 0)
	case c == '#' && i == 0 && i+1 < len(line) && line[i+1] == '!':
		return s.emit(len(line), editor.TokenComment)
	case identStartAt(line, i) || c == '#' && i+1 < len(line) && isIdentStartByte(line[i+1]):
		return s.word(i)
	case c >= utf8.RuneSelf:
		_, n := utf8.DecodeRuneInString(line[i:])
		return s.tok(i+n, editor.TokenText, pvOperand, 0)
	}
	return s.operator(i)
}

func regexEnd(line string, i int) int {
	inClass := false
	for j := i + 1; j < len(line); j++ {
		switch line[j] {
		case '\\':
			j++
		case '[':
			inClass = true
		case ']':
			inClass = false
		case '/':
			if inClass || j == i+1 {
				continue
			}
			j++
			for j < len(line) && line[j] >= 'a' && line[j] <= 'z' {
				j++
			}
			return j
		}
	}
	return -1
}

func (s *ecmaScan) numberEnd(i int) int {
	line := s.line
	j := i
	if line[j] == '0' && j+1 < len(line) && strings.IndexByte("xXbBoO", line[j+1]) >= 0 {
		j += 2
		for j < len(line) && (isIdentPartByte(line[j])) {
			j++
		}
		return j
	}
	for j < len(line) {
		c := line[j]
		switch {
		case isDigit(c) || c == '_':
		case c == '.' && j+1 < len(line) && isDigit(line[j+1]):
		case c == '.' && j+1 < len(line) && !isIdentStartByte(line[j+1]) && line[j+1] != '.':
		case (c == 'e' || c == 'E') && j+1 < len(line) && (isDigit(line[j+1]) || (line[j+1] == '+' || line[j+1] == '-') && j+2 < len(line) && isDigit(line[j+2])):
			j++
		case c == 'n':
			return j + 1
		default:
			return j
		}
		j++
	}
	return j
}

func escapeEnd(line string, i int) int {
	if i+1 >= len(line) {
		return len(line)
	}
	switch line[i+1] {
	case 'x':
		return min(len(line), i+4)
	case 'u':
		if i+2 < len(line) && line[i+2] == '{' {
			if e := strings.IndexByte(line[i:], '}'); e >= 0 {
				return i + e + 1
			}
			return len(line)
		}
		return min(len(line), i+6)
	}
	return i + 2
}

func (s *ecmaScan) str(i int) int {
	line := s.line
	q := line[i]
	j := i + 1
	for j < len(line) {
		switch line[j] {
		case '\\':
			s.emit(j, editor.TokenString)
			j = s.emit(min(len(line), escapeEnd(line, j)), editor.TokenEscape)
			continue
		case q:
			return s.tok(j+1, editor.TokenString, pvValue, 0)
		}
		j++
	}
	return s.tok(j, editor.TokenString, pvValue, 0)
}

func (s *ecmaScan) template(i int) int {
	line := s.line
	for j := i; j < len(line); j++ {
		switch line[j] {
		case '\\':
			s.emit(j, editor.TokenString)
			j = s.emit(min(len(line), escapeEnd(line, j)), editor.TokenEscape) - 1
		case '`':
			s.pop()
			return s.tok(j+1, editor.TokenString, pvValue, 0)
		case '$':
			if j+1 < len(line) && line[j+1] == '{' {
				s.emit(j, editor.TokenString)
				s.push(entInterp)
				return s.tok(j+2, editor.TokenEscape, pvOperand, 0)
			}
		}
	}
	return s.emit(len(line), editor.TokenString)
}

func (s *ecmaScan) punct(end int, pv int, last byte) int {
	return s.tok(end, editor.TokenPunctuation, pv, last)
}

func (s *ecmaScan) op(end int, pv int) int {
	return s.tok(end, editor.TokenOperator, pv, 0)
}

func (s *ecmaScan) operator(i int) int {
	line := s.line
	c := line[i]
	var c1, c2 byte
	if i+1 < len(line) {
		c1 = line[i+1]
	}
	if i+2 < len(line) {
		c2 = line[i+2]
	}
	if s.top() == entAs && strings.IndexByte("|&[].<>", c) < 0 {
		s.pop()
	}
	switch c {
	case '{':
		return s.openBrace(i)
	case '}':
		return s.closeBrace(i)
	case '(', '[':
		if s.inType() {
			s.push(entTParen)
		} else {
			s.push(entParen)
		}
		return s.punct(i+1, pvOperand, 0)
	case ')', ']':
		s.closeTo(entParen, entTParen)
		return s.punct(i+1, pvValue, c)
	case ';':
		for isSoftEntry(s.top()) {
			s.pop()
		}
		s.decl = ""
		return s.punct(i+1, pvStatement, 0)
	case ',':
		for k := s.top(); k == entAnn || k == entAs || k == entTernary; k = s.top() {
			s.pop()
		}
		return s.punct(i+1, pvOperand, 0)
	case '.':
		if c1 == '.' && c2 == '.' {
			return s.op(i+3, pvOperand)
		}
		s.punct(i+1, pvValue, 0)
		s.prevDot = true
		return i + 1
	case ':':
		return s.colon(i)
	case '?':
		return s.question(i, c1, c2)
	case '=':
		if c1 == '>' {
			if s.top() == entRet {
				s.pop()
			}
			if s.inType() {
				return s.op(i+2, pvOperand)
			}
			return s.op(i+2, pvStatement)
		}
		if c1 != '=' {
			if k := s.top(); k == entAnn || k == entAs {
				s.pop()
			} else if s.decl == "type" && !s.inType() {
				s.decl = ""
				s.push(entAlias)
			}
		}
		return s.op(i+opLen(line[i:]), pvOperand)
	case '<':
		return s.lessThan(i, c1)
	case '>':
		if s.top() == entTAngle || s.inType() && s.skipSoft()&ecmaEntryMask == entTAngle {
			s.closeTo(entTAngle)
			return s.punct(i+1, pvValue, 0)
		}
		return s.op(i+opLen(line[i:]), pvOperand)
	case '!':
		if s.pv == pvValue && c1 != '=' {
			return s.emit(i+1, editor.TokenOperator)
		}
	case '|', '&':
		if s.inType() && c1 != c {
			return s.op(i+1, pvOperand)
		}
	}
	return s.op(i+opLen(line[i:]), pvOperand)
}

func opLen(s string) int {
	for _, o := range ecmaOperators {
		if strings.HasPrefix(s, o) {
			return len(o)
		}
	}
	return 1
}

func (s *ecmaScan) openBrace(i int) int {
	if s.top() == entRet && s.pv != pvOperand {
		s.pop()
	}
	switch {
	case s.inType():
		s.push(entTBrace)
	case s.decl == "interface":
		s.decl = ""
		s.push(entTBrace)
	case s.decl == "class" || s.decl == "enum" || s.decl == "namespace" || s.decl == "module" || s.decl == "function":
		s.decl = ""
		s.push(entBlock)
	case s.pv == pvOperand:
		s.push(entObj)
	default:
		s.push(entBlock)
	}
	return s.punct(i+1, pvStatement, 0)
}

func (s *ecmaScan) closeBrace(i int) int {
	if s.stopAt && s.skipSoft() == 0 {
		s.stopped = true
		return i
	}
	k, ok := s.closeTo(entBlock, entObj, entTBrace, entInterp)
	switch {
	case !ok:
		return s.punct(i+1, pvValue, 0)
	case k == entInterp && s.top() == entTemplate:
		return s.tok(i+1, editor.TokenEscape, pvValue, 0)
	case k == entBlock:
		return s.punct(i+1, pvStatement, 0)
	case k == entObj:
		return s.punct(i+1, pvValue, '}')
	}
	return s.punct(i+1, pvValue, 0)
}

func (s *ecmaScan) colon(i int) int {
	top := s.top()
	last := s.last
	switch {
	case top == entCase:
		s.pop()
		return s.punct(i+1, pvStatement, 0)
	case top == entTernary:
		s.pop()
		return s.op(i+1, pvOperand)
	case top == entObj || s.inType():
	case last == ')':
		s.push(entRet)
	case last == 'i' || last == ']' || last == '}':
		switch top {
		case 0, entBlock, entTBrace, entParen, entTParen:
			s.push(entAnn)
		}
	case s.prevWord == "default":
		return s.punct(i+1, pvStatement, 0)
	}
	return s.op(i+1, pvOperand)
}

func (s *ecmaScan) question(i int, c1, c2 byte) int {
	switch {
	case c1 == '.' && !isDigit(c2):
		s.op(i+2, pvOperand)
		s.prevDot = true
		return i + 2
	case c1 == '?':
		return s.op(i+opLen(s.line[i:]), pvOperand)
	case s.last == 'i' && s.pv == pvValue && (c1 == ':' || c1 == ',' || c1 == ')'):
		return s.emit(i+1, editor.TokenOperator)
	}
	s.push(entTernary)
	return s.op(i+1, pvOperand)
}

func (s *ecmaScan) lessThan(i int, c1 byte) int {
	line := s.line
	if s.jsx && s.pv != pvValue && !s.inType() && (isIdentStartByte(c1) || c1 == '>') {
		if s.jsxOpen(i) {
			return s.pos
		}
	}
	generic := s.inType() || s.last == 'i' && s.pv == pvValue && (s.decl == "function" || s.decl == "class" || s.decl == "interface" || s.decl == "type" || s.genericCall(i))
	if generic {
		s.push(entTAngle)
		return s.punct(i+1, pvOperand, 0)
	}
	return s.op(i+opLen(line[i:]), pvOperand)
}
