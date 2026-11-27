package syntax

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

func isASCIILetter(c byte) bool { return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }

func identRunEnd(line string, i int, asciiPart func(c byte) bool) int {
	for i < len(line) {
		c := line[i]
		if c < utf8.RuneSelf {
			if !asciiPart(c) {
				break
			}
			i++
			continue
		}
		r, n := utf8.DecodeRuneInString(line[i:])
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			break
		}
		i += n
	}
	return i
}

func nextBytes(line string, i int) (byte, byte) {
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	if i >= len(line) {
		return 0, 0
	}
	if i+1 < len(line) {
		return line[i], line[i+1]
	}
	return line[i], 0
}

func blockCommentEnd(line string, i int) (end int, closed bool) {
	n := strings.Index(line[i:], "*/")
	if n < 0 {
		return len(line), false
	}
	return i + n + 2, true
}
