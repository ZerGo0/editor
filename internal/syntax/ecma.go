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
	s := e.scan(line, state, tokens, false)
	s.run()
	return s.toks, s.state()
}

func (e *ecma) expr(line string, state uint64, tokens []editor.Token) ([]editor.Token, int, uint64, bool) {
	s := e.scan(line, state, tokens, true)
	end := s.run()
	return s.toks, end, s.state(), s.stopped
}

func (e *ecma) scan(line string, state uint64, tokens []editor.Token, stop bool) *ecmaScan {
	return &ecmaScan{ecma: e, writer: newWriter(tokens), line: line, stack: state & ecmaStackMask,
		comment: state&ecmaCommentBit != 0, pv: int(state >> ecmaPrevShift & ecmaPrevMask), stopAt: stop}
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
	end := strings.Index(s.line[i:], "*/")
	if end < 0 {
		s.comment = true
		return s.emit(len(s.line), editor.TokenComment)
	}
	s.comment = false
	return s.emit(i+end+2, editor.TokenComment)
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

func scanIdent(line string, i int) int {
	for i < len(line) {
		c := line[i]
		if c < utf8.RuneSelf {
			if !isIdentPartByte(c) {
				break
			}
			i++
			continue
		}
		r, n := utf8.DecodeRuneInString(line[i:])
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			break
		}
		i += n
	}
	return i
}

func (s *ecmaScan) nextByte(i int) (byte, byte) {
	line := s.line
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	if i >= len(line) {
		return 0, 0
	}
	if i+1 < len(line) {
		return line[i], line[i+1]
	}
	return line[i], 0
}

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

func (s *ecmaScan) word(i int) int {
	line := s.line
	start := i
	if line[i] == '#' {
		i++
	}
	end := s.identEnd(i)
	w := line[start:end]
	nx, nx2 := s.nextByte(end)
	if s.prevDot {
		s.prevDot = false
		kind := editor.TokenProperty
		switch {
		case s.inType():
			kind = editor.TokenType
		case nx == '(' || nx == '<' && s.genericCall(end):
			kind = editor.TokenFunction
		}
		s.pv, s.last, s.prevWord = pvValue, 'i', ""
		return s.emit(end, kind)
	}
	if line[start] == '#' {
		kind := editor.TokenProperty
		if nx == '(' {
			kind = editor.TokenFunction
		}
		s.pv, s.last, s.prevWord = pvValue, 'i', ""
		return s.emit(end, kind)
	}
	prev := s.prevWord
	kind, pv, last := s.classify(w, prev, nx, nx2, end)
	s.pv, s.last, s.prevWord = pv, last, w
	return s.emit(end, kind)
}

func (s *ecmaScan) classify(w, prev string, nx, nx2 byte, end int) (editor.TokenKind, int, byte) {
	top := s.top()
	if s.inType() {
		switch {
		case (nx == ':' || nx == '?' && nx2 == ':') && (top == entTParen):
			return editor.TokenVariable, pvValue, 'i'
		case ecmaConstants[w]:
			return editor.TokenConstant, pvValue, 0
		case prev == "typeof":
			return editor.TokenVariable, pvValue, 'i'
		case ecmaTypeKeywords[w]:
			pv := pvOperand
			if w == "extends" && top == entAlias {
				pv = pvOperand
			}
			return editor.TokenKeyword, pv, 0
		}
		return editor.TokenType, pvValue, 'i'
	}
	isKey := (nx == ':' && nx2 != ':' || nx == '?' && nx2 == ':') && w != "default"
	if isKey && top != entTernary && top != entCase {
		switch top {
		case entObj, entTBrace, entBlock:
			return editor.TokenProperty, pvValue, 'i'
		case entParen, entTParen:
			return editor.TokenVariable, pvValue, 'i'
		}
	}
	if ecmaRunes[w] && (nx == '(' || nx == '.' || nx == '<') {
		return editor.TokenKeyword, pvValue, 'i'
	}
	if ecmaConstants[w] {
		return editor.TokenConstant, pvValue, 0
	}
	hard := ecmaHardKeywords[w]
	if !hard && ecmaSoftKeywords[w] && s.softKeyword(w, nx) {
		hard = true
	}
	if hard {
		return editor.TokenKeyword, s.keywordEffects(w, nx), 0
	}
	switch prev {
	case "class", "interface", "enum", "namespace", "module", "type", "new", "instanceof", "extends", "implements":
		if prev == "type" && s.decl != "type" {
			break
		}
		return editor.TokenType, pvValue, 'i'
	case "function":
		return editor.TokenFunction, pvValue, 'i'
	}
	switch {
	case nx == '(':
		return editor.TokenFunction, pvValue, 'i'
	case nx == '<' && s.genericCall(end):
		return editor.TokenFunction, pvValue, 'i'
	case nx == '=' && nx2 != '=' && nx2 != '>' && arrowAhead.MatchString(s.afterEquals(end)):
		return editor.TokenFunction, pvValue, 'i'
	case ecmaGlobals[w]:
		return editor.TokenType, pvValue, 'i'
	}
	return identKind(w), pvValue, 'i'
}

func identKind(w string) editor.TokenKind {
	r, _ := utf8.DecodeRuneInString(strings.TrimLeft(w, "#$"))
	if !unicode.IsUpper(r) {
		return editor.TokenVariable
	}
	if len(w) > 1 && w == strings.ToUpper(w) {
		return editor.TokenConstant
	}
	return editor.TokenType
}

func (s *ecmaScan) afterEquals(end int) string {
	rest := s.line[end:]
	if k := strings.IndexByte(rest, '='); k >= 0 {
		return rest[k+1:]
	}
	return ""
}

func (s *ecmaScan) softKeyword(w string, nx byte) bool {
	switch {
	case nx == '{' || nx == '[' || nx == '*' || nx == '"' || nx == '\'' || nx == '`' || isIdentStartByte(nx):
	case nx == '(' && w == "async":
	default:
		return false
	}
	return true
}

func (s *ecmaScan) keywordEffects(w string, nx byte) int {
	switch {
	case w == "export":
		if nx == '{' || nx == '*' {
			s.decl = w
		}
	case ecmaDeclKw[w]:
		if w != "type" || isIdentStartByte(nx) {
			s.decl = w
		}
	case w == "extends" || w == "implements":
		if (s.decl == "class" || s.decl == "interface") && s.top() != entRet {
			s.push(entRet)
		}
	case (w == "as" || w == "satisfies") && s.decl != "import" && s.decl != "export":
		s.push(entAs)
	case w == "case":
		s.push(entCase)
	}
	switch {
	case w == "this" || w == "super":
		return pvValue
	case ecmaStatementKw[w]:
		return pvStatement
	}
	return pvOperand
}

func (s *ecmaScan) genericCall(lt int) bool {
	line := s.line
	i := strings.IndexByte(line[lt:], '<')
	if i < 0 {
		return false
	}
	i += lt
	depth := 0
	for j := i; j < len(line) && j < i+genericLookahead; j++ {
		switch c := line[j]; c {
		case '<':
			depth++
		case '>':
			depth--
			if depth == 0 {
				k := j + 1
				return k < len(line) && (line[k] == '(' || line[k] == '`')
			}
		case ';':
			return false
		case '|', '&':
			if j+1 < len(line) && line[j+1] == c {
				return false
			}
		}
	}
	return false
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

func (s *ecmaScan) jsxName(i int) int {
	line := s.line
	j := i
	for j < len(line) && (isIdentPartByte(line[j]) || line[j] == '.' || line[j] == ':' || line[j] == '-') {
		j++
	}
	return j
}

func (s *ecmaScan) jsxOpen(i int) bool {
	line := s.line
	end := s.jsxName(i + 1)
	rest := strings.TrimLeft(line[end:], " \t")
	if end > i+1 && (strings.HasPrefix(rest, ",") || strings.HasPrefix(rest, "extends ")) {
		return false
	}
	s.punct(i+1, pvOperand, 0)
	s.jsxTagName(i+1, end)
	s.push(entJSXTag)
	return true
}

func (s *ecmaScan) jsxTagName(from, end int) {
	if end == from {
		return
	}
	kind := editor.TokenTag
	if c := s.line[from]; c >= 'A' && c <= 'Z' || strings.IndexByte(s.line[from:end], '.') >= 0 {
		kind = editor.TokenType
	}
	s.tok(end, kind, pvValue, 0)
}

func (s *ecmaScan) jsxTag(i int) int {
	line := s.line
	c := line[i]
	switch {
	case c == ' ' || c == '\t' || c == '\r':
		return s.emit(i+1, editor.TokenText)
	case c == '>':
		s.pop()
		s.push(entJSXChild)
		return s.punct(i+1, pvValue, 0)
	case c == '/' && i+1 < len(line) && line[i+1] == '>':
		s.pop()
		return s.punct(i+2, pvValue, 0)
	case c == '{':
		s.push(entInterp)
		return s.punct(i+1, pvOperand, 0)
	case c == '"' || c == '\'':
		j := strings.IndexByte(line[i+1:], c)
		if j < 0 {
			return s.tok(len(line), editor.TokenString, pvValue, 0)
		}
		return s.tok(i+j+2, editor.TokenString, pvValue, 0)
	case c == '=':
		return s.op(i+1, pvOperand)
	case isIdentStartByte(c):
		return s.tok(s.jsxName(i), editor.TokenAttribute, pvValue, 0)
	}
	return s.emit(i+1, editor.TokenText)
}

func (s *ecmaScan) jsxChild(i int) int {
	line := s.line
	c := line[i]
	switch {
	case c == '{':
		s.push(entInterp)
		return s.punct(i+1, pvOperand, 0)
	case c == '<' && i+1 < len(line) && line[i+1] == '/':
		s.punct(i+2, pvOperand, 0)
		end := s.jsxName(i + 2)
		s.jsxTagName(i+2, end)
		j := end
		for j < len(line) && line[j] == ' ' {
			j++
		}
		s.emit(j, editor.TokenText)
		s.pop()
		if j < len(line) && line[j] == '>' {
			return s.punct(j+1, pvValue, 0)
		}
		return j
	case c == '<' && i+1 < len(line) && (isIdentStartByte(line[i+1]) || line[i+1] == '>'):
		if s.jsxOpen(i) {
			return s.pos
		}
	}
	j := i + 1
	for j < len(line) && line[j] != '{' && line[j] != '<' {
		j++
	}
	return s.emit(j, editor.TokenText)
}
