package diffmode

import (
	"strings"
	"unicode"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/egoist/mygo/ui/editor"
)

const (
	maxRunProduct = 250000
	trimmedSpace  = " \t\r\v\f"
)

type runLine struct {
	text   string
	marker string
}

func lineKey(mode git.WhitespaceMode, s string) string {
	if mode == git.WhitespaceTrim {
		return strings.TrimRight(s, trimmedSpace)
	}
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
}

func FilterPatch(p editor.FilePatch, mode git.WhitespaceMode) editor.FilePatch {
	if mode == git.WhitespaceNone || p.Binary {
		return p
	}
	out := p
	out.Hunks = make([]editor.PatchHunk, 0, len(p.Hunks))
	for _, h := range p.Hunks {
		if fh, ok := filterHunk(h, mode); ok {
			out.Hunks = append(out.Hunks, fh)
		}
	}
	return out
}

func PatchLineNumbers(p editor.FilePatch) func(side editor.DiffSide, line int) int {
	type span struct{ buffer, file int }
	var oldSpans, newSpans []span
	oldBuf, newBuf := 0, 0
	for _, h := range p.Hunks {
		oldSpans = append(oldSpans, span{oldBuf, hunkStart(h.OldStart, h.OldLines)})
		newSpans = append(newSpans, span{newBuf, hunkStart(h.NewStart, h.NewLines)})
		for _, l := range h.Lines {
			if l == "" || l[0] == noEOLPrefix {
				continue
			}
			if l[0] != addPrefix {
				oldBuf++
			}
			if l[0] != deletePrefix {
				newBuf++
			}
		}
	}
	return func(side editor.DiffSide, line int) int {
		spans := newSpans
		if side == editor.DiffOld {
			spans = oldSpans
		}
		for k := len(spans) - 1; k >= 0; k-- {
			if line >= spans[k].buffer {
				return spans[k].file + line - spans[k].buffer + 1
			}
		}
		return line + 1
	}
}

func isChange(l string) bool {
	return l != "" && (l[0] == deletePrefix || l[0] == addPrefix)
}

func filterHunk(h editor.PatchHunk, mode git.WhitespaceMode) (editor.PatchHunk, bool) {
	var lines []string
	for i := 0; i < len(h.Lines); {
		if !isChange(h.Lines[i]) {
			lines = append(lines, h.Lines[i])
			i++
			continue
		}
		var minus, plus []runLine
		var last *runLine
		for ; i < len(h.Lines); i++ {
			l := h.Lines[i]
			switch {
			case l != "" && l[0] == deletePrefix:
				minus = append(minus, runLine{text: l[1:]})
				last = &minus[len(minus)-1]
			case l != "" && l[0] == addPrefix:
				plus = append(plus, runLine{text: l[1:]})
				last = &plus[len(plus)-1]
			case l != "" && l[0] == noEOLPrefix && last != nil:
				last.marker = l
			default:
				goto runEnd
			}
		}
	runEnd:
		lines = append(lines, filterRun(minus, plus, mode)...)
	}
	h.Lines = lines
	changed, oldLines, newLines := false, 0, 0
	for _, l := range lines {
		if l == "" || l[0] == noEOLPrefix {
			continue
		}
		if l[0] != addPrefix {
			oldLines++
		}
		if l[0] != deletePrefix {
			newLines++
		}
		changed = changed || isChange(l)
	}
	h.OldLines, h.NewLines = oldLines, newLines
	return h, changed
}

func appendRunLine(out []string, prefix byte, l runLine) []string {
	out = append(out, string(prefix)+l.text)
	if l.marker != "" {
		out = append(out, l.marker)
	}
	return out
}

func matchRun(minus, plus []string) [][2]int {
	n, m := len(minus), len(plus)
	table := make([][]int32, n+1)
	for i := range table {
		table[i] = make([]int32, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if minus[i] == plus[j] {
				table[i][j] = table[i+1][j+1] + 1
			} else {
				table[i][j] = max(table[i+1][j], table[i][j+1])
			}
		}
	}
	var pairs [][2]int
	for i, j := 0, 0; i < n && j < m; {
		switch {
		case minus[i] == plus[j]:
			pairs = append(pairs, [2]int{i, j})
			i++
			j++
		case table[i+1][j] >= table[i][j+1]:
			i++
		default:
			j++
		}
	}
	return pairs
}

func filterRun(minus, plus []runLine, mode git.WhitespaceMode) []string {
	var out []string
	if len(minus)*len(plus) > maxRunProduct {
		for _, l := range minus {
			out = appendRunLine(out, deletePrefix, l)
		}
		for _, l := range plus {
			out = appendRunLine(out, addPrefix, l)
		}
		return out
	}
	minusKeys, plusKeys := make([]string, len(minus)), make([]string, len(plus))
	for i, l := range minus {
		minusKeys[i] = lineKey(mode, l.text)
	}
	for i, l := range plus {
		plusKeys[i] = lineKey(mode, l.text)
	}
	blank := func(key string) bool { return mode == git.WhitespaceIgnoreBlank && key == "" }
	emitGap := func(mFrom, mTo, pFrom, pTo int) {
		for i := mFrom; i < mTo; i++ {
			if !blank(minusKeys[i]) {
				out = appendRunLine(out, deletePrefix, minus[i])
			}
		}
		for j := pFrom; j < pTo; j++ {
			prefix := byte(addPrefix)
			if blank(plusKeys[j]) {
				prefix = contextPrefix
			}
			out = appendRunLine(out, prefix, plus[j])
		}
	}
	mi, pi := 0, 0
	for _, pair := range matchRun(minusKeys, plusKeys) {
		emitGap(mi, pair[0], pi, pair[1])
		l := plus[pair[1]]
		l.marker = plus[pair[1]].marker
		out = appendRunLine(out, contextPrefix, l)
		mi, pi = pair[0]+1, pair[1]+1
	}
	emitGap(mi, len(minus), pi, len(plus))
	return out
}
