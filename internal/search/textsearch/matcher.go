package textsearch

import (
	"errors"
	"regexp"
	"strings"
	"unicode"
)

const MaxMatchTextBytes = 400

var ErrEmptyPattern = errors.New("textsearch: empty pattern")

type PatternError struct {
	Pattern string
	Err     error
}

func (e *PatternError) Error() string {
	return "textsearch: compile pattern " + e.Pattern + ": " + e.Err.Error()
}

func (e *PatternError) Unwrap() error { return e.Err }

type Query struct {
	Pattern   string
	MatchCase bool
	WholeWord bool
	Regex     bool
	SmartCase bool
	Root      string
	Include   []string
	Exclude   []string
}

type Matcher struct {
	re        *regexp.Regexp
	regex     bool
	multiline bool
	groups    bool
	include   globSet
	exclude   globSet
}

func Compile(q Query) (*Matcher, error) {
	if q.Pattern == "" {
		return nil, ErrEmptyPattern
	}
	pattern := NormalizeNewlines(q.Pattern)
	expr := pattern
	if q.Regex {
		if _, err := regexp.Compile(pattern); err != nil {
			return nil, &PatternError{Pattern: q.Pattern, Err: err}
		}
	}
	if !q.Regex {
		expr = regexp.QuoteMeta(expr)
	}
	if q.WholeWord {
		expr = `\b(?:` + expr + `)\b`
	}
	multiline := isMultiline(pattern, q.Regex)
	flags := ""
	if !q.MatchCase && !(q.SmartCase && hasUppercase(pattern, q.Regex)) {
		flags += "i"
	}
	if multiline {
		flags += "m"
	}
	if flags != "" {
		expr = "(?" + flags + ")" + expr
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		return nil, &PatternError{Pattern: q.Pattern, Err: err}
	}
	return &Matcher{
		re:        re,
		regex:     q.Regex,
		multiline: multiline,
		groups:    q.Regex && re.NumSubexp() > 0,
		include:   newGlobSet(q.Include, q.Root),
		exclude:   newGlobSet(q.Exclude, q.Root),
	}, nil
}

func NormalizeNewlines(s string) string {
	if !strings.Contains(s, "\r") {
		return s
	}
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
}

func isMultiline(pattern string, regex bool) bool {
	if strings.Contains(pattern, "\n") {
		return true
	}
	if !regex {
		return false
	}
	for i := 0; i+1 < len(pattern); i++ {
		if pattern[i] != '\\' {
			continue
		}
		switch pattern[i+1] {
		case 'n', 'r', 'W':
			return true
		}
		i++
	}
	return false
}

func hasUppercase(pattern string, regex bool) bool {
	escaped := false
	for _, r := range pattern {
		switch {
		case escaped:
			escaped = false
		case regex && r == '\\':
			escaped = true
		case unicode.IsUpper(r):
			return true
		}
	}
	return false
}

func (m *Matcher) Multiline() bool { return m.multiline }

func (m *Matcher) Regex() bool { return m.regex }

func (m *Matcher) Wants(rel string) bool {
	if !m.include.empty() && !m.include.match(rel) {
		return false
	}
	return !m.exclude.match(rel)
}

func (m *Matcher) SkipDir(rel string) bool {
	return m.exclude.match(rel)
}

func (m *Matcher) FindAll(line string) [][2]int {
	var out [][2]int
	for _, r := range m.re.FindAllStringIndex(line, -1) {
		if r[1] > r[0] {
			out = append(out, [2]int{r[0], r[1]})
		}
	}
	return out
}
