package diffmode

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui/editor"
)

const (
	contextPrefix = ' '
	deletePrefix  = '-'
	addPrefix     = '+'
	noEOLPrefix   = '\\'
)

func hunkStart(start, count int) int {
	if count == 0 {
		return max(start, 0)
	}
	return max(start-1, 0)
}

func joinText(lines []string, eol bool) string {
	text := strings.Join(lines, textLineSeparator)
	if eol && len(lines) > 0 {
		text += textLineSeparator
	}
	return text
}

func splitText(text string) ([]string, bool) {
	lines := splitLines(text)
	return lines, text == "" || strings.HasSuffix(text, textLineSeparator)
}

func AlignPatch(p editor.FilePatch, oldText, newText string) (editor.FilePatch, string, error) {
	if p.Binary {
		return p, "", fmt.Errorf("the patch is binary")
	}
	oldLines, oldEOL := splitText(oldText)
	newLines, newEOL := splitText(newText)
	out := make([]string, 0, len(newLines))
	aligned := p
	aligned.Hunks = make([]editor.PatchHunk, 0, len(p.Hunks))
	o, n := 0, 0
	for _, h := range p.Hunks {
		hs, hn := hunkStart(h.OldStart, h.OldLines), hunkStart(h.NewStart, h.NewLines)
		if hs < o || hs > len(oldLines) || hn < n || hn > len(newLines) {
			return p, "", fmt.Errorf("hunk at line %d is out of place", h.OldStart)
		}
		out = append(out, newLines[n:hn]...)
		o, n = hs, hn
		nh := h
		nh.OldStart = len(out) + 1
		if h.OldLines == 0 {
			nh.OldStart = len(out)
		}
		nh.Lines = make([]string, len(h.Lines))
		for k, l := range h.Lines {
			nh.Lines[k] = l
			if l == "" || l[0] == noEOLPrefix {
				continue
			}
			switch l[0] {
			case contextPrefix:
				if o >= len(oldLines) || n >= len(newLines) {
					return p, "", fmt.Errorf("hunk at line %d runs past the files", h.OldStart)
				}
				nh.Lines[k] = string(contextPrefix) + newLines[n]
				out = append(out, newLines[n])
				o++
				n++
			case deletePrefix:
				if o >= len(oldLines) || oldLines[o] != l[1:] {
					return p, "", fmt.Errorf("hunk at line %d does not match line %d", h.OldStart, o+1)
				}
				out = append(out, oldLines[o])
				o++
			case addPrefix:
				if n >= len(newLines) || newLines[n] != l[1:] {
					return p, "", fmt.Errorf("hunk at line %d does not match new line %d", h.OldStart, n+1)
				}
				n++
			default:
				return p, "", fmt.Errorf("hunk at line %d has an unknown line", h.OldStart)
			}
		}
		aligned.Hunks = append(aligned.Hunks, nh)
	}
	eol := oldEOL
	if n < len(newLines) {
		eol = newEOL
	}
	out = append(out, newLines[n:]...)
	return aligned, joinText(out, eol), nil
}
