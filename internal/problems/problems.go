package problems

import (
	"sort"
	"strings"

	"github.com/egoist/mygo/ui/editor"
)

type Severity int

const (
	Error Severity = iota
	Warning
	Info
)

const (
	MaxTextSize        = 2 << 20
	MaxProblemsPerKind = 50
)

const (
	sourceBrackets = "brackets"
	sourceGit      = "git"
	sourceJSON     = "json"
	sourceYAML     = "yaml"
	sourceEOL      = "eol"
)

type Problem struct {
	Line, Col, EndCol int
	Severity          Severity
	Message, Source   string
}

func (s Severity) String() string {
	switch s {
	case Error:
		return "error"
	case Warning:
		return "warning"
	}
	return "info"
}

func Check(lang *editor.Language, filename, text string) []Problem {
	if len(text) > MaxTextSize {
		return nil
	}
	var out []Problem
	out = append(out, checkBrackets(lang, text)...)
	out = append(out, checkConflicts(text)...)
	out = append(out, checkJSON(lang, filename, text)...)
	out = append(out, checkYAML(lang, text)...)
	out = append(out, checkLineEndings(text)...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Line != out[j].Line {
			return out[i].Line < out[j].Line
		}
		return out[i].Col < out[j].Col
	})
	return out
}

func eachLine(text string, visit func(no int, line string) bool) {
	for no := 0; text != ""; no++ {
		var line string
		line, text, _ = strings.Cut(text, "\n")
		if !visit(no, strings.TrimSuffix(line, "\r")) {
			return
		}
	}
}
