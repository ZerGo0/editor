package syntax

import (
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui/editor"
)

const (
	commentMark    = "#"
	exportPrefix   = "export "
	inlineComment  = " #"
	tsvDelimiter   = '\t'
	csvDelimiter   = ','
	csvQuoteBit    = 1
	csvParityShift = 1
	hashMinLen     = 7
	hashMaxLen     = 40
	headingSeen    = 1
)

var rebaseActions = map[string]bool{
	"pick": true, "p": true, "reword": true, "r": true, "edit": true, "e": true, "squash": true, "s": true,
	"fixup": true, "f": true, "exec": true, "x": true, "break": true, "b": true, "drop": true, "d": true,
	"label": true, "l": true, "reset": true, "t": true, "merge": true, "m": true, "update-ref": true, "u": true,
}

var logLevels = map[string]editor.TokenKind{
	"ERROR": editor.TokenInvalid, "FATAL": editor.TokenInvalid, "CRITICAL": editor.TokenInvalid, "SEVERE": editor.TokenInvalid,
	"WARN": editor.TokenMeta, "WARNING": editor.TokenMeta,
	"INFO":  editor.TokenFunction,
	"DEBUG": editor.TokenVariable, "TRACE": editor.TokenVariable,
}

var dotenv = &editor.Language{
	Name: "Dotenv", Extensions: []string{".env"}, Filenames: []string{".env"},
	LineComment: commentMark, TabSize: 2,
	Tokenizer: editor.TokenizerFunc(tokenizeDotenv),
}

var ignoreFile = &editor.Language{
	Name: "Ignore", Filenames: []string{".gitignore", ".dockerignore", ".npmignore", ".prettierignore", ".eslintignore", ".helmignore", ".gcloudignore"},
	LineComment: commentMark, TabSize: 2,
	Tokenizer: editor.TokenizerFunc(tokenizeIgnore),
}

var gitCommit = &editor.Language{
	Name: "Git Commit", Filenames: []string{"COMMIT_EDITMSG", "MERGE_MSG", "TAG_EDITMSG"},
	LineComment: commentMark, TabSize: 4,
	Tokenizer: editor.TokenizerFunc(tokenizeGitCommit),
}

var gitRebase = &editor.Language{
	Name: "Git Rebase Todo", Filenames: []string{"git-rebase-todo"},
	LineComment: commentMark, TabSize: 4,
	Tokenizer: editor.TokenizerFunc(tokenizeRebase),
}

var csv = &editor.Language{
	Name: "CSV", Extensions: []string{".csv"}, TabSize: 4,
	Tokenizer: delimited(csvDelimiter),
}

var tsv = &editor.Language{
	Name: "TSV", Extensions: []string{".tsv", ".tab"}, TabSize: 4, HardTabs: true,
	Tokenizer: delimited(tsvDelimiter),
}

var logFile = &editor.Language{
	Name: "Log", Extensions: []string{".log"}, TabSize: 4,
	Tokenizer: editor.TokenizerFunc(tokenizeLog),
}

func newScan(line string, tokens []editor.Token) *scan {
	return &scan{line: line, toks: tokens, base: len(tokens)}
}

func tokenizeDotenv(line string, _ uint64, tokens []editor.Token) ([]editor.Token, uint64) {
	t := newScan(line, tokens)
	i := len(line) - len(strings.TrimLeft(line, " \t"))
	t.emit(i, editor.TokenText)
	rest := line[i:]
	if strings.HasPrefix(rest, commentMark) {
		t.emit(len(line), editor.TokenComment)
		return t.toks, 0
	}
	if strings.HasPrefix(rest, exportPrefix) {
		i += len(exportPrefix)
		t.emit(i, editor.TokenKeyword)
		rest = line[i:]
	}
	eq := strings.IndexByte(rest, '=')
	if eq <= 0 {
		t.emit(len(line), editor.TokenText)
		return t.toks, 0
	}
	t.emit(i+eq, editor.TokenProperty)
	i += eq + 1
	t.emit(i, editor.TokenOperator)
	value := line[i:]
	end := len(value)
	if q := value[:min(1, len(value))]; q == "\"" || q == "'" {
		if closeAt := strings.Index(value[1:], q); closeAt >= 0 {
			end = closeAt + 2
		}
	} else if h := strings.Index(value, inlineComment); h >= 0 {
		end = h
	}
	i += end
	t.emit(i, editor.TokenString)
	i += len(line[i:]) - len(strings.TrimLeft(line[i:], " \t"))
	t.emit(i, editor.TokenText)
	if strings.HasPrefix(line[i:], commentMark) {
		t.emit(len(line), editor.TokenComment)
	}
	t.emit(len(line), editor.TokenText)
	return t.toks, 0
}

func tokenizeIgnore(line string, _ uint64, tokens []editor.Token) ([]editor.Token, uint64) {
	t := newScan(line, tokens)
	i := len(line) - len(strings.TrimLeft(line, " \t"))
	t.emit(i, editor.TokenText)
	switch {
	case strings.HasPrefix(line[i:], commentMark):
		t.emit(len(line), editor.TokenComment)
	case strings.HasPrefix(line[i:], "!"):
		t.emit(i+1, editor.TokenOperator)
	}
	t.emit(len(line), editor.TokenText)
	return t.toks, 0
}

func isTrailerKey(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if c := s[i]; c != '-' && letterAt(s, i) != 1 {
			return false
		}
	}
	return true
}

func tokenizeGitCommit(line string, state uint64, tokens []editor.Token) ([]editor.Token, uint64) {
	t := newScan(line, tokens)
	switch {
	case strings.HasPrefix(line, commentMark):
		t.emit(len(line), editor.TokenComment)
	case state == 0 && line != "":
		t.emit(len(line), editor.TokenHeading)
		state = headingSeen
	default:
		if key, _, found := strings.Cut(line, ": "); found && isTrailerKey(key) {
			t.emit(len(key), editor.TokenProperty)
		}
		t.emit(len(line), editor.TokenText)
	}
	return t.toks, state
}

func nextWord(line string, from int) (start, end int) {
	start = from + len(line[from:]) - len(strings.TrimLeft(line[from:], " \t"))
	end = start
	for end < len(line) && line[end] != ' ' && line[end] != '\t' {
		end++
	}
	return start, end
}

func isCommitHash(s string) bool {
	if len(s) < hashMinLen || len(s) > hashMaxLen {
		return false
	}
	for i := 0; i < len(s); i++ {
		if c := s[i]; !isDigit(c) && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func tokenizeRebase(line string, _ uint64, tokens []editor.Token) ([]editor.Token, uint64) {
	t := newScan(line, tokens)
	start, end := nextWord(line, 0)
	t.emit(start, editor.TokenText)
	switch {
	case strings.HasPrefix(line[start:], commentMark):
		t.emit(len(line), editor.TokenComment)
	case rebaseActions[line[start:end]]:
		t.emit(end, editor.TokenKeyword)
		start, end = nextWord(line, end)
		t.emit(start, editor.TokenText)
		if isCommitHash(line[start:end]) {
			t.emit(end, editor.TokenConstant)
		}
	}
	t.emit(len(line), editor.TokenText)
	return t.toks, 0
}

func delimited(delim byte) editor.Tokenizer {
	return editor.TokenizerFunc(func(line string, state uint64, tokens []editor.Token) ([]editor.Token, uint64) {
		t := newScan(line, tokens)
		quoted := state&csvQuoteBit != 0
		parity := state >> csvParityShift & 1
		for i := 0; i < len(line); i++ {
			switch c := line[i]; {
			case c == '"':
				quoted = !quoted
			case c == delim && !quoted:
				t.emit(i, columnKind(parity))
				t.emit(i+1, editor.TokenPunctuation)
				parity ^= 1
			}
		}
		t.emit(len(line), columnKind(parity))
		next := parity << csvParityShift
		if quoted {
			next |= csvQuoteBit
		}
		return t.toks, next
	})
}

func columnKind(parity uint64) editor.TokenKind {
	if parity == 0 {
		return editor.TokenText
	}
	return editor.TokenString
}

func tokenizeLog(line string, _ uint64, tokens []editor.Token) ([]editor.Token, uint64) {
	t := newScan(line, tokens)
	for i := 0; i < len(line); {
		c := line[i]
		prevWord := i > 0 && (letterAt(line, i-1) == 1 || isDigit(line[i-1]))
		switch {
		case letterAt(line, i) == 1 && !prevWord:
			j := i
			for j < len(line) && (letterAt(line, j) == 1 || isDigit(line[j])) {
				j++
			}
			kind, ok := logLevels[line[i:j]]
			if !ok {
				kind = editor.TokenText
			}
			i = t.emit(j, kind)
		case isDigit(c) && !prevWord:
			j := i + 1
			for j < len(line) && (isDigit(line[j]) || strings.IndexByte(":.-/+", line[j]) >= 0 || strings.IndexByte("TZ", line[j]) >= 0 && isDigit(line[j-1])) {
				j++
			}
			kind := editor.TokenNumber
			if strings.ContainsAny(line[i:j], ":-/") {
				kind = editor.TokenConstant
			}
			i = t.emit(j, kind)
		default:
			_, n := utf8.DecodeRuneInString(line[i:])
			i = t.emit(i+n, editor.TokenText)
		}
	}
	return t.toks, 0
}
