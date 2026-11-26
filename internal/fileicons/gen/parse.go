package main

import (
	"fmt"
	"strconv"
	"strings"
)

type ident string

type object map[string]any

type parser struct {
	src string
	pos int
}

func parseExport(src, name string) (any, error) {
	marker := "export const " + name
	at := strings.Index(src, marker)
	if at < 0 {
		return nil, fmt.Errorf("export %q not found", name)
	}
	assign := strings.Index(src[at:], "=")
	if assign < 0 {
		return nil, fmt.Errorf("export %q has no assignment", name)
	}
	p := &parser{src: src, pos: at + assign + 1}
	value, err := p.value()
	if err != nil {
		return nil, fmt.Errorf("export %q: %w", name, err)
	}
	return value, nil
}

func (p *parser) fail(format string, args ...any) error {
	line := 1 + strings.Count(p.src[:min(p.pos, len(p.src))], "\n")
	return fmt.Errorf("line %d: %s", line, fmt.Sprintf(format, args...))
}

func (p *parser) skip() {
	for p.pos < len(p.src) {
		switch {
		case isSpace(p.src[p.pos]):
			p.pos++
		case strings.HasPrefix(p.src[p.pos:], "//"):
			end := strings.IndexByte(p.src[p.pos:], '\n')
			if end < 0 {
				p.pos = len(p.src)
				return
			}
			p.pos += end
		case strings.HasPrefix(p.src[p.pos:], "/*"):
			end := strings.Index(p.src[p.pos+2:], "*/")
			if end < 0 {
				p.pos = len(p.src)
				return
			}
			p.pos += end + 4
		default:
			return
		}
	}
}

func (p *parser) peek() byte {
	p.skip()
	if p.pos >= len(p.src) {
		return 0
	}
	return p.src[p.pos]
}

func (p *parser) expect(c byte) error {
	if p.peek() != c {
		return p.fail("expected %q", c)
	}
	p.pos++
	return nil
}

func (p *parser) value() (any, error) {
	switch c := p.peek(); {
	case c == '[':
		return p.array()
	case c == '{':
		return p.object()
	case c == '\'' || c == '"' || c == '`':
		return p.str()
	case c == '-' || isDigit(c):
		return p.number()
	case isIdentStart(c):
		return p.identifier()
	default:
		return nil, p.fail("unexpected %q", c)
	}
}

func (p *parser) array() (any, error) {
	p.pos++
	items := []any{}
	for {
		switch p.peek() {
		case ']':
			p.pos++
			return items, nil
		case 0:
			return nil, p.fail("unterminated array")
		}
		item, err := p.value()
		if err != nil {
			return nil, err
		}
		items = append(items, item)
		if p.peek() == ',' {
			p.pos++
		}
	}
}

func (p *parser) object() (any, error) {
	p.pos++
	obj := object{}
	for {
		c := p.peek()
		if c == '}' {
			p.pos++
			return obj, nil
		}
		var key string
		switch {
		case c == '\'' || c == '"':
			s, err := p.str()
			if err != nil {
				return nil, err
			}
			key = s.(string)
		case isIdentStart(c):
			start := p.pos
			for p.pos < len(p.src) && isIdentPart(p.src[p.pos]) {
				p.pos++
			}
			key = p.src[start:p.pos]
		default:
			return nil, p.fail("unexpected %q in object", c)
		}
		if err := p.expect(':'); err != nil {
			return nil, err
		}
		val, err := p.value()
		if err != nil {
			return nil, err
		}
		obj[key] = val
		if p.peek() == ',' {
			p.pos++
		}
	}
}

func (p *parser) str() (any, error) {
	quote := p.src[p.pos]
	p.pos++
	var b strings.Builder
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		switch {
		case c == quote:
			p.pos++
			return b.String(), nil
		case c == '\\' && p.pos+1 < len(p.src):
			p.pos++
			b.WriteByte(unescape(p.src[p.pos]))
			p.pos++
		case quote == '`' && c == '$' && strings.HasPrefix(p.src[p.pos:], "${"):
			return nil, p.fail("template interpolation is not supported")
		default:
			b.WriteByte(c)
			p.pos++
		}
	}
	return nil, p.fail("unterminated string")
}

func unescape(c byte) byte {
	switch c {
	case 'n':
		return '\n'
	case 't':
		return '\t'
	default:
		return c
	}
}

func (p *parser) number() (any, error) {
	start := p.pos
	for p.pos < len(p.src) && (isDigit(p.src[p.pos]) || strings.IndexByte("-+.eE", p.src[p.pos]) >= 0) {
		p.pos++
	}
	n, err := strconv.ParseFloat(p.src[start:p.pos], 64)
	if err != nil {
		return nil, p.fail("bad number %q", p.src[start:p.pos])
	}
	return n, nil
}

func (p *parser) identifier() (any, error) {
	start := p.pos
	for p.pos < len(p.src) && (isIdentPart(p.src[p.pos]) || p.src[p.pos] == '.') {
		p.pos++
	}
	word := p.src[start:p.pos]
	switch word {
	case "true":
		return true, nil
	case "false":
		return false, nil
	}
	if p.peek() != '(' {
		return ident(word), nil
	}
	p.pos++
	if p.peek() == ')' {
		return nil, p.fail("call %s() has no argument", word)
	}
	arg, err := p.value()
	if err != nil {
		return nil, err
	}
	if p.peek() == ',' {
		p.pos++
	}
	if err := p.expect(')'); err != nil {
		return nil, err
	}
	return arg, nil
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

func isIdentStart(c byte) bool {
	return c == '_' || c == '$' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

func isIdentPart(c byte) bool {
	return isIdentStart(c) || isDigit(c)
}
