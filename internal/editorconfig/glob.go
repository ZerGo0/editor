package editorconfig

import (
	"regexp"
	"strconv"
	"strings"
	"sync"
)

const (
	globSeparator = '/'
	globEscape    = '\\'
	rangeDots     = ".."
	numberPattern = `([+-]?\d+)`
)

type compiled struct {
	re     *regexp.Regexp
	ranges [][2]int
}

var cache sync.Map

func Match(glob, rel string) bool {
	c := compile(glob)
	if c.re == nil {
		return false
	}
	m := c.re.FindStringSubmatch(rel)
	if m == nil {
		return false
	}
	for i, r := range c.ranges {
		if m[i+1] == "" {
			continue
		}
		n, err := strconv.Atoi(m[i+1])
		if err != nil || n < r[0] || n > r[1] {
			return false
		}
	}
	return true
}

func compile(glob string) compiled {
	if v, ok := cache.Load(glob); ok {
		return v.(compiled)
	}
	p := glob
	switch {
	case strings.HasPrefix(p, "/"):
		p = p[1:]
	case !strings.ContainsRune(p, globSeparator):
		p = "**/" + p
	}
	t := translator{}
	body := t.translate(p)
	var c compiled
	if re, err := regexp.Compile("^" + body + "$"); err == nil {
		c = compiled{re: re, ranges: t.ranges}
	}
	cache.Store(glob, c)
	return c
}

type translator struct {
	ranges [][2]int
}

func (t *translator) translate(p string) string {
	var b strings.Builder
	for i := 0; i < len(p); i++ {
		ch := p[i]
		switch ch {
		case globEscape:
			if i+1 < len(p) {
				i++
				b.WriteString(regexp.QuoteMeta(string(p[i])))
			}
		case '*':
			if i+1 < len(p) && p[i+1] == '*' {
				i++
				switch {
				case i+1 < len(p) && p[i+1] == globSeparator && (i == 1 || p[i-2] == globSeparator):
					i++
					b.WriteString("(?:.*/)?")
				default:
					b.WriteString(".*")
				}
				continue
			}
			b.WriteString("[^/]*")
		case '?':
			b.WriteString("[^/]")
		case '[':
			end := classEnd(p, i)
			if end < 0 {
				b.WriteString(regexp.QuoteMeta("["))
				continue
			}
			b.WriteString(class(p[i+1 : end]))
			i = end
		case '{':
			end := braceEnd(p, i)
			if end < 0 {
				b.WriteString(regexp.QuoteMeta("{"))
				continue
			}
			b.WriteString(t.brace(p[i+1 : end]))
			i = end
		default:
			b.WriteString(regexp.QuoteMeta(string(ch)))
		}
	}
	return b.String()
}

func classEnd(p string, start int) int {
	for i := start + 1; i < len(p); i++ {
		switch p[i] {
		case globEscape:
			i++
		case globSeparator:
			return -1
		case ']':
			if i > start+1 {
				return i
			}
		}
	}
	return -1
}

func class(body string) string {
	negate := strings.HasPrefix(body, "!") || strings.HasPrefix(body, "^")
	if negate {
		body = body[1:]
	}
	var b strings.Builder
	b.WriteByte('[')
	if negate {
		b.WriteByte('^')
	}
	for i := 0; i < len(body); i++ {
		switch ch := body[i]; {
		case ch == globEscape && i+1 < len(body):
			i++
			b.WriteString(regexp.QuoteMeta(string(body[i])))
		case ch == '-':
			b.WriteByte('-')
		case ch == ']' || ch == '[' || ch == '^' || ch == globEscape:
			b.WriteByte(globEscape)
			b.WriteByte(ch)
		default:
			b.WriteByte(ch)
		}
	}
	b.WriteByte(']')
	return b.String()
}

func braceEnd(p string, start int) int {
	depth := 0
	for i := start; i < len(p); i++ {
		switch p[i] {
		case globEscape:
			i++
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func (t *translator) brace(body string) string {
	if lo, hi, ok := numericRange(body); ok {
		t.ranges = append(t.ranges, [2]int{lo, hi})
		return numberPattern
	}
	parts := splitTop(body)
	if len(parts) < 2 {
		return regexp.QuoteMeta("{") + t.translate(body) + regexp.QuoteMeta("}")
	}
	out := make([]string, len(parts))
	for i, part := range parts {
		out[i] = t.translate(part)
	}
	return "(?:" + strings.Join(out, "|") + ")"
}

func numericRange(body string) (int, int, bool) {
	a, b, ok := strings.Cut(body, rangeDots)
	if !ok {
		return 0, 0, false
	}
	lo, err1 := strconv.Atoi(a)
	hi, err2 := strconv.Atoi(b)
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return min(lo, hi), max(lo, hi), true
}

func splitTop(body string) []string {
	var parts []string
	depth, start := 0, 0
	for i := 0; i < len(body); i++ {
		switch body[i] {
		case globEscape:
			i++
		case '{':
			depth++
		case '}':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, body[start:i])
				start = i + 1
			}
		}
	}
	return append(parts, body[start:])
}
