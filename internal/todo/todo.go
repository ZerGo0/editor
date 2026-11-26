package todo

import (
	"strings"

	"github.com/egoist/mygo/ui/editor"
)

const (
	maxLineLen = 2000
	maxNameLen = 64
)

type Item struct {
	Line, Col int
	Tag, Text string
}

var DefaultPatterns = []string{"TODO", "FIXME", "HACK", "XXX", "BUG", "NOTE", "TECHDEBT"}

var fallbackMarkers = []string{"//", "#", "--", ";", "/*", "*"}

func Scan(lang *editor.Language, text string, patterns []string) []Item {
	if len(patterns) == 0 {
		patterns = DefaultPatterns
	}
	var items []Item
	var toks []editor.Token
	var state uint64
	var blockClose string
	tokenized := lang != nil && lang.Tokenizer != nil
	if tokenized {
		blockClose = lang.BlockComment[1]
	}
	for lineNo := 0; text != ""; lineNo++ {
		var line string
		line, text, _ = strings.Cut(text, "\n")
		line = strings.TrimSuffix(line, "\r")
		var ranges [][2]int
		if tokenized {
			toks, state = lang.Tokenizer.Tokenize(line, state, toks[:0])
			ranges = commentRanges(toks)
		} else if start, ok := fallbackComment(line); ok {
			ranges = [][2]int{{start, len(line)}}
		}
		if len(line) > maxLineLen {
			continue
		}
		for _, r := range ranges {
			if item, ok := findTag(line, r[0], r[1], patterns, blockClose); ok {
				item.Line = lineNo
				items = append(items, item)
			}
		}
	}
	return items
}

func commentRanges(toks []editor.Token) [][2]int {
	var ranges [][2]int
	start := 0
	for _, tk := range toks {
		if tk.Kind == editor.TokenComment {
			ranges = append(ranges, [2]int{start, tk.End})
		}
		start = tk.End
	}
	return ranges
}

func fallbackComment(line string) (int, bool) {
	start := len(line) - len(strings.TrimLeft(line, " \t"))
	for _, m := range fallbackMarkers {
		if strings.HasPrefix(line[start:], m) {
			return start, true
		}
	}
	return 0, false
}

func isWordByte(c byte) bool {
	return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 0x80
}

func findTag(line string, from, to int, patterns []string, blockClose string) (Item, bool) {
	for i := from; i < to; i++ {
		if i > from && isWordByte(line[i-1]) {
			continue
		}
		for _, p := range patterns {
			if p == "" || !strings.HasPrefix(line[i:to], p) {
				continue
			}
			end := i + len(p)
			if isWordByte(p[len(p)-1]) && end < to && isWordByte(line[end]) {
				continue
			}
			return Item{Col: i, Tag: p, Text: tagText(line[end:to], blockClose)}, true
		}
	}
	return Item{}, false
}

func tagText(rest, blockClose string) string {
	if strings.HasPrefix(rest, "(") {
		if closeAt := strings.IndexByte(rest, ')'); closeAt > 0 && closeAt <= maxNameLen {
			rest = rest[closeAt+1:]
		}
	}
	rest = strings.TrimPrefix(rest, ":")
	rest = strings.TrimSpace(rest)
	if blockClose != "" {
		rest = strings.TrimSpace(strings.TrimSuffix(rest, blockClose))
	}
	return rest
}
