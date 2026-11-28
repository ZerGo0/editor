package errtext

import (
	"errors"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Reasoner interface {
	Reason() string
}

type matcher func(err error) (string, bool)

var (
	matchers      = []matcher{reasoned, canceled, gitText, gitHub, network, fileSystem}
	packagePrefix = regexp.MustCompile(`^[a-z][a-z0-9]*: `)
)

func Text(err error) string {
	if err == nil {
		return ""
	}
	for _, m := range matchers {
		if s, ok := m(err); ok {
			return s
		}
	}
	return sentence(err.Error())
}

func reasoned(err error) (string, bool) {
	var r Reasoner
	if errors.As(err, &r) {
		return r.Reason(), true
	}
	return "", false
}

func sentence(s string) string {
	for packagePrefix.MatchString(s) {
		s = packagePrefix.ReplaceAllString(s, "")
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return "Something went wrong."
	}
	r, size := utf8.DecodeRuneInString(s)
	s = string(unicode.ToUpper(r)) + s[size:]
	if !strings.ContainsAny(s[len(s)-1:], ".!?") {
		s += "."
	}
	return s
}
