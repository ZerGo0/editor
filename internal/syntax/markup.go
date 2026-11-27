package syntax

import (
	"strings"

	"github.com/egoist/mygo/ui/editor"
)

const (
	flavorHTML = iota
	flavorSvelte
	flavorAstro
	flavorMDX
)

const (
	mkText = iota
	mkTag
	mkComment
	mkDouble
	mkSingle
	mkScript
	mkStyle
	mkExpr
	mkFront
)

const (
	mkModeShift     = ecmaStateBits + 1
	mkModeMask      = 0xf
	mkEmbedShift    = mkModeShift + 4
	mkBackShift     = mkEmbedShift + 2
	mkFrontDone     = 1 << (mkBackShift + 2)
	markupStateBits = mkBackShift + 3
	mkInnerMask     = 1<<mkModeShift - 1
	mkEmbedScript   = 1
	mkEmbedStyle    = 2
	frontmatterFlag = "---"
	entityMaxLen    = 32
)

var (
	mkBackModes      = [...]int{mkText, mkTag, mkDouble, mkSingle}
	markupDirectives = map[int]map[string]bool{
		flavorSvelte: set("on bind class use transition in out animate let style"),
		flavorAstro:  set("client set define is class"),
	}
	blockSeparators = map[string]string{"#each": " as ", "#await": " then "}
)

type markup struct {
	flavor int
	script *ecma
	expr   *ecma
	style  editor.Tokenizer
}

type markupScan struct {
	writer
	*markup
	line  string
	mode  int
	embed int
	back  int
	inner uint64
}

func (m *markup) Tokenize(line string, state uint64, tokens []editor.Token) ([]editor.Token, uint64) {
	s := markupScan{writer: newWriter(tokens), markup: m, line: line, mode: int(state >> mkModeShift & mkModeMask),
		embed: int(state >> mkEmbedShift & 3), back: int(state >> mkBackShift & 3), inner: state & mkInnerMask}
	if s.mode > mkFront {
		s.mode = mkText
	}
	done := state&mkFrontDone != 0
	if m.flavor == flavorAstro && !done && strings.TrimSpace(line) != "" {
		done = true
		if strings.TrimSpace(line) == frontmatterFlag {
			s.mode = mkFront
			s.emit(len(line), editor.TokenMeta)
			return s.toks, s.pack(done)
		}
	}
	for i := 0; i < len(line); {
		switch s.mode {
		case mkText:
			i = s.text(i)
		case mkTag:
			i = s.tag(i)
		case mkComment:
			i = s.comment(i)
		case mkDouble, mkSingle:
			i = s.quoted(i)
		case mkScript, mkStyle:
			i = s.raw(i)
		case mkExpr:
			i = s.expression(i)
		case mkFront:
			i = s.front(i)
		}
	}
	return s.toks, s.pack(done)
}

func (s *markupScan) pack(done bool) uint64 {
	v := s.inner | uint64(s.mode)<<mkModeShift | uint64(s.embed)<<mkEmbedShift | uint64(s.back)<<mkBackShift
	if done {
		v |= mkFrontDone
	}
	return v
}

func (s *markupScan) front(i int) int {
	if strings.TrimSpace(s.line) == frontmatterFlag {
		s.mode, s.inner = mkText, 0
		return s.emit(len(s.line), editor.TokenMeta)
	}
	s.inner = s.nest(s.script, s.line[i:], i, s.inner)
	return len(s.line)
}

func isTagStart(line string, i int) bool {
	return i+1 < len(line) && (line[i+1] == '/' || isIdentStartByte(line[i+1]))
}

func (s *markupScan) text(i int) int {
	line := s.line
	c := line[i]
	switch {
	case strings.HasPrefix(line[i:], "<!--"):
		s.mode = mkComment
		return s.emit(i+4, editor.TokenComment)
	case c == '<' && i+1 < len(line) && (line[i+1] == '!' || line[i+1] == '?'):
		end := strings.IndexByte(line[i:], '>')
		if end < 0 {
			return s.emit(len(line), editor.TokenMeta)
		}
		return s.emit(i+end+1, editor.TokenMeta)
	case c == '<' && isTagStart(line, i):
		return s.openTag(i)
	case c == '&':
		j := i + 1
		for j < len(line) && j-i < entityMaxLen && (isIdentPartByte(line[j]) || line[j] == '#') {
			j++
		}
		if j < len(line) && line[j] == ';' && j > i+1 {
			return s.emit(j+1, editor.TokenEscape)
		}
		return s.emit(i+1, editor.TokenText)
	case c == '{' && s.flavor != flavorHTML:
		return s.brace(i, 0)
	}
	j := i + 1
	for j < len(line) && line[j] != '<' && line[j] != '&' && (line[j] != '{' || s.flavor == flavorHTML) {
		j++
	}
	return s.emit(j, editor.TokenText)
}

func (s *markupScan) openTag(i int) int {
	line := s.line
	s.emit(i+1, editor.TokenPunctuation)
	j := i + 1
	closing := line[j] == '/'
	if closing {
		s.emit(j+1, editor.TokenPunctuation)
		j++
	}
	k := j
	for k < len(line) && (isIdentPartByte(line[k]) || line[k] == '-' || line[k] == ':' || line[k] == '.') {
		k++
	}
	kind := editor.TokenTag
	if s.flavor != flavorHTML && k > j && (line[j] >= 'A' && line[j] <= 'Z' || strings.IndexByte(line[j:k], '.') >= 0) {
		kind = editor.TokenType
	}
	s.emit(k, kind)
	s.embed = 0
	if !closing {
		switch strings.ToLower(line[j:k]) {
		case "script":
			s.embed = mkEmbedScript
		case "style":
			s.embed = mkEmbedStyle
		}
	}
	s.mode = mkTag
	return k
}

func (s *markupScan) tag(i int) int {
	line := s.line
	c := line[i]
	switch {
	case c == ' ' || c == '\t' || c == '\r':
		return s.emit(i+1, editor.TokenText)
	case c == '>':
		s.emit(i+1, editor.TokenPunctuation)
		s.mode, s.inner = mkText, 0
		switch s.embed {
		case mkEmbedScript:
			s.mode = mkScript
		case mkEmbedStyle:
			s.mode = mkStyle
		}
		s.embed = 0
		return i + 1
	case c == '/' && i+1 < len(line) && line[i+1] == '>':
		s.mode, s.embed = mkText, 0
		return s.emit(i+2, editor.TokenPunctuation)
	case c == '=':
		return s.emit(i+1, editor.TokenOperator)
	case c == '"' || c == '\'':
		s.mode = mkDouble
		if c == '\'' {
			s.mode = mkSingle
		}
		return s.emit(i+1, editor.TokenString)
	case c == '{' && s.flavor != flavorHTML:
		return s.brace(i, 1)
	}
	j := i
	for j < len(line) && strings.IndexByte(" \t\r=>/\"'{", line[j]) < 0 {
		j++
	}
	if j == i {
		return s.emit(i+1, editor.TokenText)
	}
	return s.attributeName(i, j)
}

func (s *markupScan) attributeName(i, j int) int {
	name := s.line[i:j]
	if colon := strings.IndexByte(name, ':'); colon > 0 && markupDirectives[s.flavor][name[:colon]] {
		s.emit(i+colon, editor.TokenKeyword)
		s.emit(i+colon+1, editor.TokenPunctuation)
		bar := strings.IndexByte(name, '|')
		if bar < 0 {
			return s.emit(j, editor.TokenAttribute)
		}
		s.emit(i+bar, editor.TokenAttribute)
		return s.emit(j, editor.TokenOperator)
	}
	return s.emit(j, editor.TokenAttribute)
}

func (s *markupScan) comment(i int) int {
	end := strings.Index(s.line[i:], "-->")
	if end < 0 {
		return s.emit(len(s.line), editor.TokenComment)
	}
	s.mode = mkText
	return s.emit(i+end+3, editor.TokenComment)
}

func (s *markupScan) quoted(i int) int {
	line := s.line
	stops := "\""
	if s.mode == mkSingle {
		stops = "'"
	}
	if s.flavor == flavorSvelte {
		stops += "{"
	}
	end := strings.IndexAny(line[i:], stops)
	if end < 0 {
		return s.emit(len(line), editor.TokenString)
	}
	if line[i+end] == '{' {
		s.emit(i+end, editor.TokenString)
		return s.brace(i+end, s.mode-mkDouble+2)
	}
	s.mode = mkTag
	return s.emit(i+end+1, editor.TokenString)
}

func (s *markupScan) brace(i, back int) int {
	line := s.line
	s.emit(i+1, editor.TokenPunctuation)
	s.mode, s.back, s.inner = mkExpr, back, 0
	j := i + 1
	if s.flavor != flavorSvelte || j+1 >= len(line) || strings.IndexByte("#:/@", line[j]) < 0 || !isIdentStartByte(line[j+1]) {
		return j
	}
	k := j + 1
	for k < len(line) && isIdentPartByte(line[k]) {
		k++
	}
	s.emit(k, editor.TokenKeyword)
	sep := blockSeparators[line[j:k]]
	if sep == "" {
		return k
	}
	at := strings.Index(line[k:], sep)
	if at < 0 {
		return k
	}
	s.runExpr(k, k+at)
	s.emit(k+at+1, editor.TokenText)
	s.emit(k+at+len(sep)-1, editor.TokenKeyword)
	s.emit(k+at+len(sep), editor.TokenText)
	s.inner = 0
	return k + at + len(sep)
}

func (s *markupScan) runExpr(from, to int) (int, bool) {
	toks, end, next, closed := s.expr.expr(s.line[from:to], s.inner, s.toks[len(s.toks):])
	for _, tk := range toks {
		if tk.End > end {
			break
		}
		s.emit(from+tk.End, tk.Kind)
	}
	s.inner = next
	return from + end, closed
}

func (s *markupScan) expression(i int) int {
	end, closed := s.runExpr(i, len(s.line))
	if !closed {
		return len(s.line)
	}
	s.emit(end+1, editor.TokenPunctuation)
	s.mode, s.inner = mkBackModes[s.back], 0
	s.back = 0
	return end + 1
}

func (s *markupScan) raw(i int) int {
	closer, t := "</script", editor.Tokenizer(s.script)
	if s.mode == mkStyle {
		closer, t = "</style", s.style
	}
	stop := len(s.line)
	end := indexFold(s.line[i:], closer)
	if end >= 0 {
		stop = i + end
	}
	if stop > i {
		s.inner = s.nest(t, s.line[i:stop], i, s.inner)
	}
	if end >= 0 {
		s.mode, s.inner = mkText, 0
	}
	return stop
}

func indexFold(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i] == '<' && strings.EqualFold(s[i:i+len(sub)], sub) {
			return i
		}
	}
	return -1
}
