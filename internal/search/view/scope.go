package searchview

import "strings"

type Scope int

const (
	ScopeAll Scope = iota
	ScopeFiles
	ScopeSymbols
	ScopeText
	ScopeActions
	ScopeGit
	scopeCount
)

var scopeNames = [scopeCount]string{"All", "Files", "Symbols", "Text", "Actions", "Git"}

func (s Scope) String() string {
	if s < 0 || s >= scopeCount {
		return scopeNames[ScopeAll]
	}
	return scopeNames[s]
}

func parseScope(name string) Scope {
	for i, n := range scopeNames {
		if strings.EqualFold(n, name) {
			return Scope(i)
		}
	}
	return ScopeAll
}

const (
	prefixSymbols = '#'
	prefixEditor  = '@'
	prefixActions = '>'
	prefixLine    = ':'
	prefixText    = '/'
	prefixTextAlt = '?'
)

type prefixRoute struct {
	scope    Scope
	query    string
	inEditor bool
	matched  bool
}

func routePrefix(q string) prefixRoute {
	if q == "" {
		return prefixRoute{}
	}
	rest := q[1:]
	switch q[0] {
	case prefixSymbols:
		return prefixRoute{scope: ScopeSymbols, query: rest, matched: true}
	case prefixEditor:
		return prefixRoute{scope: ScopeSymbols, query: rest, inEditor: true, matched: true}
	case prefixActions:
		return prefixRoute{scope: ScopeActions, query: strings.TrimLeft(rest, " "), matched: true}
	case prefixLine:
		return prefixRoute{scope: ScopeFiles, query: q, matched: true}
	case prefixText, prefixTextAlt:
		return prefixRoute{scope: ScopeText, query: rest, matched: true}
	}
	return prefixRoute{}
}

type fileQuery struct {
	path    string
	line    int
	col     int
	hasLine bool
}

func parseFileQuery(q string) fileQuery {
	fq := fileQuery{path: q}
	rest := q
	var nums []int
	for len(nums) < 2 {
		i := strings.LastIndexByte(rest, ':')
		if i < 0 {
			break
		}
		n, ok := atoiPositive(rest[i+1:])
		if !ok {
			break
		}
		nums = append([]int{n}, nums...)
		rest = rest[:i]
	}
	if len(nums) == 0 {
		return fq
	}
	fq.path, fq.hasLine, fq.line = rest, true, nums[0]-1
	if len(nums) == 2 {
		fq.col = nums[1] - 1
	}
	return fq
}

func atoiPositive(s string) (int, bool) {
	if s == "" || len(s) > 9 {
		return 0, false
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int(r-'0')
	}
	return n, n > 0
}
