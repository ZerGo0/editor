package searchview

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/ZerGo0/editor/internal/design/list"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/search/textsearch"
	"github.com/egoist/mygo/ui"
)

func itoa(n int) string { return strconv.Itoa(n) }

type matchParts struct {
	before string
	hit    string
	after  string
}

func splitMatch(m textsearch.Match) matchParts {
	line := m.Preview
	start, end := min(m.PreviewStart, len(line)), min(m.PreviewEnd, len(line))
	start = max(start, 0)
	end = max(end, start)
	before := strings.TrimLeft(line[:start], " \t")
	before = tailRunes(before, beforeContext)
	return matchParts{before: before, hit: line[start:end], after: line[end:]}
}

func tailRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	runes := []rune(s)
	return "…" + string(runes[len(runes)-n:])
}

func matchSpans(p *token.Palette, s list.Style, m textsearch.Match, replacement string, replacing bool) []ui.Span {
	parts := splitMatch(m)
	spans := []ui.Span{{Text: parts.before, Color: s.Text}}
	hit := ui.Span{Text: parts.hit, Weight: token.WeightBold, Color: s.Text, Background: p.Severity.Warning.Alpha(token.OpacityMatchFill.Value()), Radius: token.RadiusInline.Px()}
	if replacing {
		hit.Strikethrough = true
		hit.Background = p.Diff.RemovedText
		spans = append(spans, hit)
		if replacement != "" {
			spans = append(spans, ui.Span{Text: replacement, Weight: token.WeightBold, Color: s.Text, Background: p.Diff.InsertedText, Radius: token.RadiusInline.Px()})
		}
	} else {
		spans = append(spans, hit)
	}
	spans = append(spans, ui.Span{Text: parts.after, Color: s.Text})
	if m.Multiline() {
		spans = append(spans, ui.Span{Text: " …", Weight: token.WeightBold, Color: s.Text})
	}
	return spans
}
