package syntax

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui/editor"
)

type wordGroup struct {
	kind editor.TokenKind
	list string
}

func kw(list string) wordGroup    { return wordGroup{editor.TokenKeyword, list} }
func types(list string) wordGroup { return wordGroup{editor.TokenType, list} }
func consts(list string) wordGroup {
	return wordGroup{editor.TokenConstant, list}
}
func funcs(list string) wordGroup { return wordGroup{editor.TokenFunction, list} }
func vars(list string) wordGroup  { return wordGroup{editor.TokenVariable, list} }

func words(groups ...wordGroup) map[string]editor.TokenKind {
	m := make(map[string]editor.TokenKind)
	for _, g := range groups {
		for _, w := range strings.Fields(g.list) {
			m[w] = g.kind
		}
	}
	return m
}

func numbered(prefix string, from, to, step int) string {
	var b strings.Builder
	for n := from; n <= to; n += step {
		b.WriteString(prefix + strconv.Itoa(n) + " ")
	}
	return b.String()
}

func re(pattern string) *regexp.Regexp { return regexp.MustCompile(pattern) }

var (
	brackets = []editor.BracketPair{{Open: "(", Close: ")"}, {Open: "[", Close: "]"}, {Open: "{", Close: "}"}}

	hash      = []string{"#"}
	slash     = []string{"//"}
	dash      = []string{"--"}
	semicolon = []string{";"}
	percent   = []string{"%"}
	bang      = []string{"!"}

	cBlock = []blockComment{{open: "/*", close: "*/"}}

	dq       = stringSpec{open: "\"", close: "\"", escape: '\\'}
	sq       = stringSpec{open: "'", close: "'", escape: '\\'}
	dqMulti  = stringSpec{open: "\"", close: "\"", escape: '\\', multiline: true}
	dqTriple = stringSpec{open: "\"\"\"", close: "\"\"\"", escape: '\\', multiline: true}
	sqTriple = stringSpec{open: "'''", close: "'''", escape: '\\', multiline: true}
	rawDQ    = stringSpec{open: "\"", close: "\""}
	rawSQ    = stringSpec{open: "'", close: "'"}
	charLit  = stringSpec{open: "'", close: "'", escape: '\\', optional: true}
)

func autoClose(quotes ...string) []editor.BracketPair {
	out := append([]editor.BracketPair{}, brackets...)
	for _, q := range quotes {
		out = append(out, editor.BracketPair{Open: q, Close: q, NotIn: []editor.TokenKind{editor.TokenString, editor.TokenComment}})
	}
	return out
}

func strs(specs ...stringSpec) []stringSpec { return specs }
