package syntax

import (
	"strings"

	"github.com/egoist/mygo/ui/editor"
)

const (
	dockerInstructionMask = 0x7
	dockerContinuedBit    = 1 << 3
	dockerShellForm       = 1
	dockerKeyValueForm    = 2
	dockerFromForm        = 3
	dockerOtherForm       = 4
	dockerLineJoin        = '\\'
	dockerImageNext       = 1
	dockerStageNext       = 2
)

var (
	dockerShell    = set("run cmd entrypoint healthcheck")
	dockerKeyValue = set("env arg label")
	dockerOther    = set("add copy expose maintainer onbuild shell stopsignal user volume workdir")
	dockerShellOps = []string{"&&", "||", ">>", "2>&1", "|", ";", ">", "<"}
)

type dockerfileLexer struct{}

type dockerScan struct {
	writer
	line      string
	form      int
	cmdStart  bool
	afterFrom int
	keyDone   bool
}

func (dockerfileLexer) Tokenize(line string, state uint64, tokens []editor.Token) ([]editor.Token, uint64) {
	s := dockerScan{writer: newWriter(tokens), line: line, form: int(state & dockerInstructionMask)}
	continued := state&dockerContinuedBit != 0
	i := 0
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	s.emit(i, editor.TokenText)
	if i < len(line) && line[i] == '#' {
		s.emit(len(line), editor.TokenComment)
		return s.toks, state
	}
	if !continued && i < len(line) {
		i = s.instruction(i)
	} else {
		s.cmdStart = s.form == dockerShellForm
	}
	if s.arguments(i) {
		return s.toks, uint64(s.form) | dockerContinuedBit
	}
	return s.toks, 0
}

func (s *dockerScan) instruction(i int) int {
	line := s.line
	j := i
	for j < len(line) && line[j] != ' ' && line[j] != '\t' {
		j++
	}
	word := strings.ToLower(line[i:j])
	switch {
	case dockerShell[word]:
		s.form, s.cmdStart = dockerShellForm, true
	case dockerKeyValue[word]:
		s.form = dockerKeyValueForm
	case word == "from":
		s.form, s.afterFrom = dockerFromForm, dockerImageNext
	case dockerOther[word]:
		s.form = dockerOtherForm
	default:
		s.form = 0
		return i
	}
	return s.emit(j, editor.TokenKeyword)
}

func (s *dockerScan) arguments(i int) bool {
	line := s.line
	for i < len(line) {
		c := line[i]
		switch {
		case c == ' ' || c == '\t' || c == '\r':
			i = s.emit(i+1, editor.TokenText)
		case c == dockerLineJoin && strings.TrimSpace(line[i+1:]) == "":
			s.emit(len(line), editor.TokenPunctuation)
			return true
		case c == '"' || c == '\'':
			i = s.quoted(i)
			s.cmdStart = false
		case c == '$':
			i = s.variable(i)
		case c == '-' && i+1 < len(line) && line[i+1] == '-' && (i == 0 || line[i-1] == ' ' || line[i-1] == '\t'):
			i = s.flag(i)
		default:
			i = s.word(i)
		}
	}
	return false
}

func (s *dockerScan) quoted(i int) int {
	line := s.line
	q := line[i]
	j := i + 1
	for j < len(line) {
		switch {
		case line[j] == '\\' && q == '"':
			s.emit(j, editor.TokenString)
			j = s.emit(min(len(line), j+2), editor.TokenEscape)
			continue
		case line[j] == '$' && q == '"':
			s.emit(j, editor.TokenString)
			j = s.variable(j)
			continue
		case line[j] == q:
			return s.emit(j+1, editor.TokenString)
		}
		j++
	}
	return s.emit(len(line), editor.TokenString)
}

func (s *dockerScan) variable(i int) int {
	line := s.line
	j := i + 1
	switch {
	case j < len(line) && line[j] == '{':
		if end := strings.IndexByte(line[j:], '}'); end >= 0 {
			return s.emit(j+end+1, editor.TokenConstant)
		}
		return s.emit(len(line), editor.TokenConstant)
	case j < len(line) && isIdentStartByte(line[j]) && line[j] != '$':
		return s.emit(scanIdent(line, j), editor.TokenConstant)
	}
	return s.emit(i+1, editor.TokenText)
}

func (s *dockerScan) flag(i int) int {
	line := s.line
	j := i + 2
	for j < len(line) && line[j] != '=' && line[j] != ' ' && line[j] != '\t' {
		j++
	}
	s.emit(j, editor.TokenAttribute)
	if j >= len(line) || line[j] != '=' {
		return j
	}
	j = s.emit(j+1, editor.TokenOperator)
	if j < len(line) && line[j] == '$' {
		return s.variable(j)
	}
	k := j
	for k < len(line) && line[k] != ' ' && line[k] != '\t' {
		k++
	}
	return s.emit(k, editor.TokenText)
}

func (s *dockerScan) word(i int) int {
	line := s.line
	for _, op := range dockerShellOps {
		if s.form == dockerShellForm && strings.HasPrefix(line[i:], op) {
			s.cmdStart = op != ">" && op != ">>" && op != "<" && op != "2>&1"
			return s.emit(i+len(op), editor.TokenOperator)
		}
	}
	if line[i] == '[' || line[i] == ']' || line[i] == ',' {
		return s.emit(i+1, editor.TokenPunctuation)
	}
	j := i
	for j < len(line) && strings.IndexByte(" \t\r\"'$,[]", line[j]) < 0 && !(s.form == dockerShellForm && strings.IndexByte("&|;<>", line[j]) >= 0) {
		j++
	}
	if j == i {
		return s.emit(i+1, editor.TokenText)
	}
	w := line[i:j]
	switch {
	case s.form == dockerFromForm && s.afterFrom == 0 && strings.EqualFold(w, "as"):
		s.afterFrom = dockerStageNext
		return s.emit(j, editor.TokenKeyword)
	case s.afterFrom != 0:
		s.afterFrom = 0
		return s.emit(j, editor.TokenType)
	case s.form == dockerKeyValueForm || s.cmdStart && strings.Contains(w, "="):
		if eq := strings.IndexByte(w, '='); eq > 0 {
			s.emit(i+eq, editor.TokenProperty)
			s.emit(i+eq+1, editor.TokenOperator)
			return s.emit(j, editor.TokenString)
		}
		if s.form == dockerKeyValueForm {
			if s.keyDone {
				return s.emit(j, editor.TokenString)
			}
			s.keyDone = true
			return s.emit(j, editor.TokenProperty)
		}
	case s.cmdStart:
		s.cmdStart = false
		return s.emit(j, editor.TokenFunction)
	case allDigits(w):
		return s.emit(j, editor.TokenNumber)
	}
	return s.emit(j, editor.TokenText)
}

func allDigits(w string) bool {
	for i := 0; i < len(w); i++ {
		if !isDigit(w[i]) && w[i] != '/' && w[i] != '-' {
			return false
		}
	}
	return w != ""
}
