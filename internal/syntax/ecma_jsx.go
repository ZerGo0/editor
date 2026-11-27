package syntax

import (
	"strings"

	"github.com/egoist/mygo/ui/editor"
)

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
