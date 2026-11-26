package problems

import (
	"fmt"
	"strings"
)

func checkLineEndings(text string) []Problem {
	crlf := strings.Count(text, "\r\n")
	lf := strings.Count(text, "\n") - crlf
	if crlf == 0 || lf == 0 {
		return nil
	}
	return []Problem{{Severity: Info, Message: fmt.Sprintf("Mixed line endings (%d CRLF, %d LF)", crlf, lf), Source: sourceEOL}}
}
