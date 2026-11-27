package syntax

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/egoist/mygo/ui/editor"
)

func (s *ecmaScan) word(i int) int {
	line := s.line
	start := i
	if line[i] == '#' {
		i++
	}
	end := s.identEnd(i)
	w := line[start:end]
	nx, nx2 := nextBytes(s.line, end)
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
