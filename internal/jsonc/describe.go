package jsonc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"unicode/utf8"
)

const (
	NoLine         = -1
	noOffset       = -1
	numberPrefix   = "number "
	quotedCharFrom = "invalid character "
)

type Problem struct {
	Text      string
	Line, Col int
}

func (p Problem) Error() string { return p.Text }

func (p Problem) Located() bool { return p.Line != NoLine }

func Unlocated(text string) Problem {
	return Problem{Text: text, Line: NoLine}
}

func At(data []byte, offset int, text string) Problem {
	if offset < 0 {
		return Unlocated(text)
	}
	line, col := Position(data, offset)
	return Problem{Text: text, Line: line, Col: col}
}

func Position(data []byte, offset int) (line, col int) {
	offset = max(0, min(offset, len(data)))
	head := data[:offset]
	for {
		i := bytes.IndexByte(head, '\n')
		if i < 0 {
			break
		}
		line++
		head = head[i+1:]
	}
	return line, utf8.RuneCount(head)
}

type Failure struct {
	Message  string
	Expected string
	Found    string
	Field    string
	Offset   int
	Syntax   bool
}

func (f Failure) Sentence() string {
	if f.Message == "" {
		return ""
	}
	return strings.ToUpper(f.Message[:1]) + f.Message[1:] + "."
}

func Describe(err error) Failure {
	var syntax *json.SyntaxError
	var typed *json.UnmarshalTypeError
	switch {
	case errors.As(err, &syntax):
		return Failure{Message: syntaxMessage(syntax), Offset: syntaxOffset(syntax), Syntax: true}
	case errors.As(err, &typed):
		f := Failure{Field: typed.Field, Offset: int(typed.Offset), Expected: expectedOf(typed.Type), Found: foundOf(typed.Value)}
		if after, ok := strings.CutPrefix(typed.Value, numberPrefix); ok && f.Expected == "a whole number" {
			f.Found = after
		}
		f.Message = fmt.Sprintf("expected %s, but found %s", f.Expected, f.Found)
		return f
	}
	return Failure{Message: strings.TrimSpace(err.Error()), Offset: noOffset}
}

func syntaxMessage(e *json.SyntaxError) string {
	msg := e.Error()
	if msg == "unexpected end of JSON input" {
		return "unexpected end of file"
	}
	if rest, ok := strings.CutPrefix(msg, quotedCharFrom); ok {
		if char, _, found := strings.Cut(rest, " "); found {
			return "unexpected character " + char
		}
	}
	return "invalid JSON"
}

func syntaxOffset(e *json.SyntaxError) int {
	if strings.HasPrefix(e.Error(), quotedCharFrom) {
		return int(e.Offset) - 1
	}
	return int(e.Offset)
}

func expectedOf(t reflect.Type) string {
	switch t.Kind() {
	case reflect.String:
		return "a string"
	case reflect.Bool:
		return "true or false"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "a whole number"
	case reflect.Float32, reflect.Float64:
		return "a number"
	case reflect.Slice, reflect.Array:
		return "a list"
	}
	return "an object"
}

func foundOf(value string) string {
	switch {
	case value == "string":
		return "a string"
	case value == "bool":
		return "true or false"
	case value == "array":
		return "a list"
	case value == "object":
		return "an object"
	case strings.HasPrefix(value, numberPrefix), value == "number":
		return "a number"
	}
	return value
}
