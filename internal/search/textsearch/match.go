package textsearch

import (
	"context"
	"strings"
	"unicode/utf8"
)

const ctxCheckLines = 4096

type Pos struct {
	Line int
	Col  int
}

type Match struct {
	Start, End         Pos
	StartRune, EndRune int
	Preview            string
	PreviewStart       int
	PreviewEnd         int
	Matched            string
	Groups             []string
}

func (m Match) Multiline() bool { return m.End.Line != m.Start.Line }

type hit struct {
	start, end int
	startLine  int
	startBase  int
	endLine    int
	endBase    int
	sub        []int
}

func (m *Matcher) each(ctx context.Context, text string, limit int, fn func(h hit) bool) {
	if m.multiline {
		m.eachMultiline(text, limit, fn)
		return
	}
	lineStart := 0
	for lineNo := 0; lineStart <= len(text); lineNo++ {
		if lineNo%ctxCheckLines == 0 && ctx.Err() != nil {
			return
		}
		end := len(text)
		if i := strings.IndexByte(text[lineStart:], '\n'); i >= 0 {
			end = lineStart + i
		}
		line := strings.TrimSuffix(text[lineStart:end], "\r")
		if line != "" && !m.eachInLine(line, lineNo, lineStart, fn) {
			return
		}
		lineStart = end + 1
	}
}

func (m *Matcher) eachInLine(line string, lineNo, base int, fn func(h hit) bool) bool {
	if m.groups {
		for _, loc := range m.re.FindAllStringSubmatchIndex(line, -1) {
			if loc[1] > loc[0] && !fn(hit{start: base + loc[0], end: base + loc[1], startLine: lineNo, startBase: base, endLine: lineNo, endBase: base, sub: shift(loc, base)}) {
				return false
			}
		}
		return true
	}
	for _, loc := range m.re.FindAllStringIndex(line, -1) {
		if loc[1] > loc[0] && !fn(hit{start: base + loc[0], end: base + loc[1], startLine: lineNo, startBase: base, endLine: lineNo, endBase: base}) {
			return false
		}
	}
	return true
}

func shift(loc []int, base int) []int {
	out := make([]int, len(loc))
	for i, v := range loc {
		if v < 0 {
			out[i] = -1
			continue
		}
		out[i] = v + base
	}
	return out
}

func (m *Matcher) eachMultiline(text string, limit int, fn func(h hit) bool) {
	// TECHDEBT: The cap counts zero-width matches in multi-line mode; scan with a manual loop if empty matches crowd out real ones.
	n := limit
	if n <= 0 {
		n = -1
	}
	lines := lineTracker{text: text}
	for _, loc := range m.re.FindAllStringSubmatchIndex(text, n) {
		if loc[1] <= loc[0] {
			continue
		}
		startLine, startBase := lines.advance(loc[0])
		endLine, endBase := lines.advance(loc[1])
		h := hit{start: loc[0], end: loc[1], startLine: startLine, startBase: startBase, endLine: endLine, endBase: endBase}
		if m.groups {
			h.sub = loc
		}
		if !fn(h) {
			return
		}
	}
}

type lineTracker struct {
	text string
	pos  int
	line int
	base int
}

func (t *lineTracker) advance(to int) (line, base int) {
	for t.pos < to {
		i := strings.IndexByte(t.text[t.pos:to], '\n')
		if i < 0 {
			t.pos = to
			break
		}
		t.pos += i + 1
		t.line++
		t.base = t.pos
	}
	return t.line, t.base
}

type runeCounter struct {
	base, col, runes int
}

func (c *runeCounter) at(text string, base, col int) int {
	if base == c.base && col >= c.col && c.runes > 0 {
		c.runes += utf8.RuneCountInString(text[base+c.col : base+col])
	} else {
		c.runes = utf8.RuneCountInString(text[base : base+col])
	}
	c.base, c.col = base, col
	return c.runes
}

func buildMatch(text string, h hit, rc *runeCounter) Match {
	lineEnd := len(text)
	if i := strings.IndexByte(text[h.startBase:], '\n'); i >= 0 {
		lineEnd = h.startBase + i
	}
	line := strings.TrimSuffix(text[h.startBase:lineEnd], "\r")
	startCol := h.start - h.startBase
	hitEnd := min(h.end-h.startBase, len(line))
	if h.endLine != h.startLine {
		hitEnd = len(line)
	}
	m := Match{
		Start:     Pos{Line: h.startLine, Col: startCol},
		End:       Pos{Line: h.endLine, Col: h.end - h.endBase},
		StartRune: rc.at(text, h.startBase, startCol),
		Matched:   cutBytes(text[h.start:h.end], MaxMatchTextBytes),
	}
	if h.endLine == h.startLine {
		m.EndRune = m.StartRune + utf8.RuneCountInString(text[h.start:h.end])
	} else {
		m.EndRune = utf8.RuneCountInString(text[h.endBase:h.end])
	}
	m.Preview, m.PreviewStart, m.PreviewEnd = cutPreview(line, startCol, max(hitEnd, startCol))
	if len(h.sub) > 2 {
		m.Groups = make([]string, 0, len(h.sub)/2-1)
		for i := 2; i+1 < len(h.sub); i += 2 {
			if h.sub[i] < 0 {
				m.Groups = append(m.Groups, "")
				continue
			}
			m.Groups = append(m.Groups, cutBytes(text[h.sub[i]:h.sub[i+1]], MaxMatchTextBytes))
		}
	}
	return m
}

func cutBytes(s string, limit int) string {
	if len(s) > limit {
		end := limit
		for end > 0 && !utf8.RuneStart(s[end]) {
			end--
		}
		s = s[:end]
	}
	return strings.Clone(s)
}

func cutPreview(line string, from, to int) (text string, start, end int) {
	if len(line) <= MaxMatchTextBytes {
		return strings.Clone(line), from, to
	}
	first := from - (MaxMatchTextBytes-(to-from))/2
	if first < 0 {
		first = 0
	}
	last := first + MaxMatchTextBytes
	if last > len(line) {
		last = len(line)
		first = last - MaxMatchTextBytes
	}
	for first < len(line) && !utf8.RuneStart(line[first]) {
		first++
	}
	for last > first && last < len(line) && !utf8.RuneStart(line[last]) {
		last--
	}
	start, end = max(from, first), min(to, last)
	return strings.Clone(line[first:last]), start - first, max(end-first, start-first)
}
