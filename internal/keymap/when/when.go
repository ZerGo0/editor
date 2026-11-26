package when

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

type Context map[string]any

type Clause struct {
	text string
	root node
	keys []string
}

type node interface {
	eval(ctx Context) bool
}

type op int

const (
	opEq op = iota
	opNe
	opMatch
	opLt
	opLe
	opGt
	opGe
	opIn
	opNotIn
)

const (
	literalTrue  = "true"
	literalFalse = "false"
	regexFlagI   = 'i'
	floatBits    = 64
)

var operators = []struct {
	text string
	op   op
}{
	{"===", opEq}, {"!==", opNe}, {"==", opEq}, {"!=", opNe}, {"=~", opMatch},
	{"<=", opLe}, {">=", opGe}, {"<", opLt}, {">", opGt},
}

func Parse(s string) (*Clause, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	p := &parser{src: s}
	root, err := p.or()
	if err != nil {
		return nil, fmt.Errorf("when %q: %w", s, err)
	}
	if p.skipSpace(); p.pos < len(p.src) {
		return nil, fmt.Errorf("when %q: unexpected %q", s, p.src[p.pos:])
	}
	slices.Sort(p.keys)
	return &Clause{text: s, root: root, keys: slices.Compact(p.keys)}, nil
}

func (c *Clause) String() string {
	if c == nil {
		return ""
	}
	return c.text
}

func (c *Clause) Keys() []string {
	if c == nil {
		return nil
	}
	return c.keys
}

func (c *Clause) Unknown(ctx Context) []string {
	var out []string
	for _, k := range c.Keys() {
		if _, ok := ctx[k]; !ok {
			out = append(out, k)
		}
	}
	return out
}

func (c *Clause) Eval(ctx Context) bool {
	if c == nil {
		return true
	}
	return len(c.Unknown(ctx)) == 0 && c.root.eval(ctx)
}

func Normalize(s string) string {
	return strings.Join(strings.Fields(s), "")
}

type (
	orNode   struct{ a, b node }
	andNode  struct{ a, b node }
	notNode  struct{ a node }
	constant bool
	keyNode  string
	cmpNode  struct {
		key   string
		op    op
		value string
		re    *regexp.Regexp
	}
)

func (n orNode) eval(ctx Context) bool  { return n.a.eval(ctx) || n.b.eval(ctx) }
func (n andNode) eval(ctx Context) bool { return n.a.eval(ctx) && n.b.eval(ctx) }
func (n notNode) eval(ctx Context) bool { return !n.a.eval(ctx) }
func (n constant) eval(Context) bool    { return bool(n) }
func (n keyNode) eval(ctx Context) bool { return truthy(ctx[string(n)]) }

func (n cmpNode) eval(ctx Context) bool {
	v := ctx[n.key]
	switch n.op {
	case opEq:
		return text(v) == n.value
	case opNe:
		return text(v) != n.value
	case opMatch:
		return n.re.MatchString(text(v))
	case opIn, opNotIn:
		return contains(ctx[n.value], text(v)) == (n.op == opIn)
	}
	a, errA := strconv.ParseFloat(text(v), floatBits)
	b, errB := strconv.ParseFloat(n.value, floatBits)
	if errA != nil || errB != nil {
		return false
	}
	switch n.op {
	case opLt:
		return a < b
	case opLe:
		return a <= b
	case opGt:
		return a > b
	default:
		return a >= b
	}
}

func truthy(v any) bool {
	switch v := v.(type) {
	case nil:
		return false
	case bool:
		return v
	case string:
		return v != ""
	case int:
		return v != 0
	case float64:
		return v != 0
	}
	return true
}

func text(v any) string {
	switch v := v.(type) {
	case nil:
		return ""
	case string:
		return v
	}
	return fmt.Sprint(v)
}

func contains(list any, item string) bool {
	switch l := list.(type) {
	case []string:
		return slices.Contains(l, item)
	case map[string]bool:
		return l[item]
	}
	return false
}

type parser struct {
	src  string
	pos  int
	keys []string
}

func (p *parser) skipSpace() {
	for p.pos < len(p.src) && (p.src[p.pos] == ' ' || p.src[p.pos] == '\t' || p.src[p.pos] == '\n' || p.src[p.pos] == '\r') {
		p.pos++
	}
}

func (p *parser) eat(s string) bool {
	p.skipSpace()
	if strings.HasPrefix(p.src[p.pos:], s) {
		p.pos += len(s)
		return true
	}
	return false
}

func (p *parser) or() (node, error) {
	n, err := p.and()
	for err == nil && p.eat("||") {
		var b node
		if b, err = p.and(); err == nil {
			n = orNode{n, b}
		}
	}
	return n, err
}

func (p *parser) and() (node, error) {
	n, err := p.unary()
	for err == nil && p.eat("&&") {
		var b node
		if b, err = p.unary(); err == nil {
			n = andNode{n, b}
		}
	}
	return n, err
}

func (p *parser) unary() (node, error) {
	p.skipSpace()
	if strings.HasPrefix(p.src[p.pos:], "!") && !strings.HasPrefix(p.src[p.pos:], "!=") {
		p.pos++
		n, err := p.unary()
		return notNode{n}, err
	}
	if p.eat("(") {
		n, err := p.or()
		if err == nil && !p.eat(")") {
			err = fmt.Errorf("missing ) at %d", p.pos)
		}
		return n, err
	}
	return p.comparison()
}

func (p *parser) comparison() (node, error) {
	key := p.word()
	switch key {
	case "":
		return nil, fmt.Errorf("expected a context key at %d", p.pos)
	case literalTrue:
		return constant(true), nil
	case literalFalse:
		return constant(false), nil
	}
	p.keys = append(p.keys, key)
	p.skipSpace()
	if p.eatWord("not") {
		if !p.eatWord("in") {
			return nil, fmt.Errorf("expected in after not at %d", p.pos)
		}
		return p.inNode(key, opNotIn)
	}
	if p.eatWord("in") {
		return p.inNode(key, opIn)
	}
	for _, o := range operators {
		if !p.eat(o.text) {
			continue
		}
		if o.op == opMatch {
			re, err := p.regex()
			return cmpNode{key: key, op: opMatch, re: re}, err
		}
		value, err := p.value()
		return cmpNode{key: key, op: o.op, value: value}, err
	}
	return keyNode(key), nil
}

func (p *parser) inNode(key string, o op) (node, error) {
	list := p.word()
	if list == "" {
		return nil, fmt.Errorf("expected a context key after in at %d", p.pos)
	}
	p.keys = append(p.keys, list)
	return cmpNode{key: key, op: o, value: list}, nil
}

func (p *parser) eatWord(w string) bool {
	save := p.pos
	if p.word() == w {
		return true
	}
	p.pos = save
	return false
}

func (p *parser) word() string {
	p.skipSpace()
	start := p.pos
	for p.pos < len(p.src) && !strings.ContainsRune(" \t\r\n()!&|=<>'\"~", rune(p.src[p.pos])) {
		p.pos++
	}
	return p.src[start:p.pos]
}

func (p *parser) value() (string, error) {
	p.skipSpace()
	if p.pos < len(p.src) && p.src[p.pos] == '\'' {
		end := strings.IndexByte(p.src[p.pos+1:], '\'')
		if end < 0 {
			return "", fmt.Errorf("unterminated string at %d", p.pos)
		}
		v := p.src[p.pos+1 : p.pos+1+end]
		p.pos += end + 2
		return v, nil
	}
	v := p.word()
	if v == "" {
		return "", fmt.Errorf("expected a value at %d", p.pos)
	}
	return v, nil
}

func (p *parser) regex() (*regexp.Regexp, error) {
	p.skipSpace()
	if p.pos >= len(p.src) || p.src[p.pos] != '/' {
		return nil, fmt.Errorf("expected /regex/ at %d", p.pos)
	}
	var b strings.Builder
	i := p.pos + 1
	for ; i < len(p.src) && p.src[i] != '/'; i++ {
		if p.src[i] == '\\' && i+1 < len(p.src) {
			b.WriteByte(p.src[i])
			i++
		}
		b.WriteByte(p.src[i])
	}
	if i >= len(p.src) {
		return nil, fmt.Errorf("unterminated regex at %d", p.pos)
	}
	p.pos = i + 1
	pattern := b.String()
	for p.pos < len(p.src) && p.src[p.pos] >= 'a' && p.src[p.pos] <= 'z' {
		if p.src[p.pos] == regexFlagI {
			pattern = "(?i)" + pattern
		}
		p.pos++
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("regex: %w", err)
	}
	return re, nil
}
