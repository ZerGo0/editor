package searchview

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/ZerGo0/editor/internal/search/textsearch"
	"github.com/egoist/mygo/ui"
)

func itoa(n int) string { return strconv.Itoa(n) }

func runeSlice(s string, positions []int) (marked []bool) {
	marked = make([]bool, utf8.RuneCountInString(s))
	for _, p := range positions {
		if p >= 0 && p < len(marked) {
			marked[p] = true
		}
	}
	return marked
}

func highlightSpans(t *ui.Theme, text string, positions []int, selected bool) []ui.Span {
	if len(positions) == 0 {
		return []ui.Span{{Text: text}}
	}
	marked := runeSlice(text, positions)
	var spans []ui.Span
	var run strings.Builder
	current := false
	flush := func() {
		if run.Len() == 0 {
			return
		}
		sp := ui.Span{Text: run.String()}
		if current {
			sp.Weight = 700
			if selected {
				sp.Underline = true
			} else {
				sp.Color = t.Accent
			}
		}
		spans = append(spans, sp)
		run.Reset()
	}
	i := 0
	for _, r := range text {
		if marked[i] != current {
			flush()
			current = marked[i]
		}
		run.WriteRune(r)
		i++
	}
	flush()
	return spans
}

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

func matchSpans(t *ui.Theme, m textsearch.Match, replacement string, replacing, selected bool) []ui.Span {
	p := splitMatch(m)
	high := t.Warning.Alpha(highlightAlpha)
	if selected {
		high = t.AccentText.Alpha(selectedHigh)
	}
	spans := []ui.Span{{Text: p.before}}
	hit := ui.Span{Text: p.hit, Weight: 700, Background: high, Radius: 2}
	if replacing {
		hit.Strikethrough = true
		hit.Background = t.Danger.Alpha(strongTint)
		spans = append(spans, hit)
		if replacement != "" {
			spans = append(spans, ui.Span{Text: replacement, Weight: 700, Background: t.Success.Alpha(strongTint), Radius: 2})
		}
	} else {
		spans = append(spans, hit)
	}
	spans = append(spans, ui.Span{Text: p.after})
	if m.Multiline() {
		spans = append(spans, ui.Span{Text: " …", Weight: 700})
	}
	return spans
}
