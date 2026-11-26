package symbols

import (
	"strings"
)

const (
	maxLineBytes     = 2000
	maxCharLiteral   = 6
	charLiteralShort = 2
)

type Symbol struct {
	Name      string
	Container string
	Kind      Kind
	Line      int
	Col       int
}

type entry struct {
	level int
	name  string
	scope scope
}

type scanner struct {
	lang     *lang
	out      []Symbol
	blockEnd string
	inBlock  bool
	group    *group
	depth    int
	stack    []entry
	pending  *entry
}

func Supports(language string) bool {
	_, ok := languages[language]
	return ok
}

func Scan(language, text string) []Symbol {
	l := languages[language]
	if l == nil {
		return nil
	}
	s := scanner{lang: l}
	rest := text
	for n := 0; ; n++ {
		line := rest
		i := strings.IndexByte(rest, '\n')
		if i >= 0 {
			line = rest[:i]
			rest = rest[i+1:]
		}
		s.scanLine(n, strings.TrimSuffix(line, "\r"))
		if i < 0 {
			break
		}
	}
	return s.out
}

func (s *scanner) top() *entry {
	if len(s.stack) == 0 {
		return nil
	}
	return &s.stack[len(s.stack)-1]
}

func (s *scanner) scanLine(n int, line string) {
	if len(line) > maxLineBytes {
		return
	}
	trimmed := strings.TrimLeft(line, " \t")
	if trimmed == "" || s.skipComment(line) {
		return
	}
	indent := len(line) - len(trimmed)
	if s.lang.indent && !strings.ContainsRune(indentClosers, rune(trimmed[0])) {
		for top := s.top(); top != nil && top.level >= indent; top = s.top() {
			s.stack = s.stack[:len(s.stack)-1]
		}
	}
	if s.scanGroup(n, line) {
		return
	}
	before := s.depth
	intent := s.matchRules(n, line, before)
	switch {
	case s.lang.braces:
		s.advanceBraces(line, intent)
	case intent != nil:
		intent.level = indent
		s.stack = append(s.stack, *intent)
	}
}

func (s *scanner) skipComment(line string) bool {
	if s.inBlock {
		if strings.Contains(line, s.blockEnd) {
			s.inBlock = false
		}
		return true
	}
	for _, d := range s.lang.blocks {
		i := indexOutsideString(line, d.start)
		if i < 0 {
			continue
		}
		if !strings.Contains(line[i+len(d.start):], d.end) {
			s.inBlock = true
			s.blockEnd = d.end
		}
		return strings.TrimSpace(line[:i]) == ""
	}
	return false
}

func indexOutsideString(line, token string) int {
	i := strings.Index(line, token)
	if i < 0 {
		return -1
	}
	prefix := line[:i]
	if strings.Count(prefix, "\"")%2 != 0 || strings.Count(prefix, "`")%2 != 0 {
		return -1
	}
	return i
}

func (s *scanner) scanGroup(n int, line string) bool {
	if s.group != nil {
		if line[0] == ')' {
			s.group = nil
			return true
		}
		if m := s.group.item.FindStringSubmatchIndex(line); m != nil {
			s.out = append(s.out, Symbol{Name: strings.Clone(line[m[2]:m[3]]), Kind: s.group.kind, Line: n, Col: m[2]})
		}
		return true
	}
	for i := range s.lang.groups {
		g := &s.lang.groups[i]
		if strings.HasPrefix(line, g.open) && isBlankOrLineComment(line[len(g.open):]) {
			s.group = g
			return true
		}
	}
	return false
}

func isBlankOrLineComment(rest string) bool {
	rest = strings.TrimSpace(rest)
	return rest == "" || strings.HasPrefix(rest, "//")
}

func (s *scanner) matchRules(n int, line string, before int) *entry {
	top := s.top()
	if s.lang.braces && before != s.lang.baseDepth && (top == nil || before != top.level) {
		return nil
	}
	inScope := top != nil && (s.lang.indent || top.level == before)
	inType := inScope && top.scope == scopeType
	for _, r := range s.lang.rules {
		if r.member && !inType {
			continue
		}
		if r.hint != "" && !strings.Contains(line, r.hint) {
			continue
		}
		if m := r.re.FindStringSubmatchIndex(line); m != nil {
			return s.accept(n, line, r, m, top, inScope)
		}
	}
	return nil
}

func (s *scanner) accept(n int, line string, r *rule, m []int, top *entry, inScope bool) *entry {
	var name, container string
	col := 0
	if r.name >= 0 {
		col = m[2*r.name]
		name = strings.Clone(line[col:m[2*r.name+1]])
	}
	member := false
	switch {
	case r.container >= 0 && m[2*r.container] >= 0:
		container = strings.Clone(line[m[2*r.container]:m[2*r.container+1]])
		member = true
	case inScope && (top.scope == scopeType || top.scope == scopeNamespace):
		container = top.name
		member = top.scope == scopeType
	}
	if !r.hidden {
		kind := r.kind
		if member {
			switch kind {
			case Function:
				kind = Method
			case Var:
				kind = Field
			}
		}
		s.out = append(s.out, Symbol{Name: name, Container: container, Kind: kind, Line: n, Col: col})
	}
	if r.scope == scopeNone {
		return nil
	}
	return &entry{name: name, scope: r.scope}
}

// TECHDEBT: Brace depth ignores multi-line strings and block comments inside a line; replace with a real tokenizer if containers drift.
func (s *scanner) advanceBraces(line string, intent *entry) {
	if intent != nil {
		s.pending = intent
	}
	var quote byte
scan:
	for i := 0; i < len(line); i++ {
		c := line[i]
		if quote != 0 {
			switch c {
			case '\\':
				i++
			case quote:
				quote = 0
			}
			continue
		}
		switch {
		case c == '"' || c == '`':
			quote = c
		case c == '\'':
			i = s.skipSingleQuote(line, i)
		case s.lang.lineComment != "" && strings.HasPrefix(line[i:], s.lang.lineComment):
			break scan
		case c == '{' || (s.lang.brackets && c == '['):
			s.depth++
			if s.pending != nil {
				s.pending.level = s.depth
				s.stack = append(s.stack, *s.pending)
				s.pending = nil
			}
		case c == '}' || (s.lang.brackets && c == ']'):
			if s.depth > 0 {
				s.depth--
			}
			for top := s.top(); top != nil && top.level > s.depth; top = s.top() {
				s.stack = s.stack[:len(s.stack)-1]
			}
		}
	}
	if s.pending != nil && strings.Contains(line, ";") {
		s.pending = nil
	}
}

func (s *scanner) skipSingleQuote(line string, i int) int {
	if s.lang.singleQuote {
		for j := i + 1; j < len(line); j++ {
			switch line[j] {
			case '\\':
				j++
			case '\'':
				return j
			}
		}
		return i
	}
	end := min(len(line), i+maxCharLiteral)
	if i+charLiteralShort < len(line) && line[i+1] != '\\' && line[i+charLiteralShort] == '\'' {
		return i + charLiteralShort
	}
	if i+1 < len(line) && line[i+1] == '\\' {
		if j := strings.IndexByte(line[i+charLiteralShort:end], '\''); j >= 0 {
			return i + charLiteralShort + j
		}
	}
	return i
}
