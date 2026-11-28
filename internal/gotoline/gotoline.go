package gotoline

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	NoEditor        = "Open a text editor first to go to a line."
	Placeholder     = "Type the line number and optional column to go to (e.g. 42:5 for line 42 and column 5)."
	separators      = ",:#"
	firstLine       = 1
	singleLineHelp  = "Current Line: %d, Character: %d. Type a line number to navigate to."
	rangeHelp       = "Current Line: %d, Character: %d. Type a line number between 1 and %d to navigate to."
	lineLabel       = "Go to line %d."
	lineColumnLabel = "Go to line %d and character %d."
)

type Target struct {
	Line     int
	Column   int
	LineOK   bool
	ColumnOK bool
}

func Parse(input string, lineCount int, lineLength func(line int) int) Target {
	numbers := numbersIn(input)
	t := Target{}
	if len(numbers) == 0 {
		return t
	}
	t.Line = numbers[0]
	if t.Line <= 0 {
		t.Line += lineCount + 1
	}
	t.LineOK = t.Line >= firstLine && t.Line <= lineCount
	if !t.LineOK {
		return t
	}
	if len(numbers) > 1 {
		t.Column = numbers[1]
		t.ColumnOK = t.Column >= 1 && t.Column <= lineLength(t.Line-1)+1
	}
	return t
}

func (t Target) Label(line, column, lineCount int) string {
	switch {
	case t.LineOK && t.ColumnOK:
		return fmt.Sprintf(lineColumnLabel, t.Line, t.Column)
	case t.LineOK:
		return fmt.Sprintf(lineLabel, t.Line)
	}
	return Prompt(line, column, lineCount)
}

func Prompt(line, column, lineCount int) string {
	if lineCount > 1 {
		return fmt.Sprintf(rangeHelp, line, column, lineCount)
	}
	return fmt.Sprintf(singleLineHelp, line, column)
}

func ByteColumn(text string, column int) int {
	offset := 0
	for i := 1; i < column && offset < len(text); i++ {
		_, size := utf8.DecodeRuneInString(text[offset:])
		offset += size
	}
	return offset
}

func numbersIn(input string) []int {
	var out []int
	for _, part := range strings.FieldsFunc(input, func(r rune) bool { return strings.ContainsRune(separators, r) }) {
		if n, ok := leadingInt(strings.TrimSpace(part)); ok {
			out = append(out, n)
		}
	}
	return out
}

func leadingInt(s string) (int, bool) {
	end := 0
	if end < len(s) && (s[end] == '-' || s[end] == '+') {
		end++
	}
	digits := end
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	if end == digits {
		return 0, false
	}
	n, err := strconv.Atoi(s[:end])
	return n, err == nil
}
