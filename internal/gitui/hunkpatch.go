package gitui

import (
	"strings"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/egoist/mygo/ui/editor"
)

const hunkContext = 3

func hunkPatch(path, oldText, newText string, h editor.DiffHunk, contextFromNew bool) []byte {
	oldLines, newLines := git.SplitLines(oldText), git.SplitLines(newText)
	oStart, oEnd := clampRange(h.OldStart, h.OldEnd, len(oldLines))
	nStart, nEnd := clampRange(h.NewStart, h.NewEnd, len(newLines))
	if oStart == oEnd && nStart == nEnd {
		return nil
	}
	oldNoEOL, newNoEOL := missingEOL(oldText), missingEOL(newText)
	side, start, end, sideNoEOL := oldLines, oStart, oEnd, oldNoEOL
	if contextFromNew {
		side, start, end, sideNoEOL = newLines, nStart, nEnd, newNoEOL
	}
	first := max(0, start-hunkContext)
	last := min(len(side), end+hunkContext)
	var lines []git.Line
	context := func(i int) {
		lines = append(lines, git.Line{Kind: ' ', Text: side[i], NoEOL: sideNoEOL && i == len(side)-1})
	}
	for i := first; i < start; i++ {
		context(i)
	}
	for i := oStart; i < oEnd; i++ {
		lines = append(lines, git.Line{Kind: '-', Text: oldLines[i], NoEOL: oldNoEOL && i == len(oldLines)-1})
	}
	for i := nStart; i < nEnd; i++ {
		lines = append(lines, git.Line{Kind: '+', Text: newLines[i], NoEOL: newNoEOL && i == len(newLines)-1})
	}
	for i := end; i < last; i++ {
		context(i)
	}
	hk := git.Hunk{Lines: lines}
	for _, l := range lines {
		if l.Kind != '+' {
			hk.OldLines++
		}
		if l.Kind != '-' {
			hk.NewLines++
		}
	}
	lead := start - first
	hk.OldStart = rangeStart(oStart-lead, hk.OldLines)
	hk.NewStart = rangeStart(nStart-lead, hk.NewLines)
	fd := git.FileDiff{OldPath: path, NewPath: path, Hunks: []git.Hunk{hk}}
	return git.BuildPatch(fd, func(int, int) bool { return true }, false)
}

func clampRange(start, end, n int) (int, int) {
	start = min(max(start, 0), n)
	end = min(max(end, start), n)
	return start, end
}

func rangeStart(first0, lines int) int {
	if lines == 0 {
		return first0
	}
	return first0 + 1
}

func missingEOL(text string) bool {
	return text != "" && !strings.HasSuffix(text, "\n")
}
