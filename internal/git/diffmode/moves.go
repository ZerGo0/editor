package diffmode

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"unicode"

	"github.com/egoist/mygo/ui/editor"
)

const (
	MinMoveLines      = 3
	MinMoveChars      = 60
	maxKeyOccurrences = 32
	cancelCheckEvery  = 512
	noHunk            = -1
	textLineSeparator = "\n"
	blankKey          = ""
)

type Move struct {
	OldStart, OldEnd, NewStart, NewEnd int
}

type candidate struct {
	old, new, length int
}

type moveFinder struct {
	hunks   []editor.DiffHunk
	oldKeys []string
	newKeys []string
	oldHunk []int
	newHunk []int
	usedOld []bool
	usedNew []bool
}

func DetectMoves(ctx context.Context, hunks []editor.DiffHunk, oldText, newText string) []Move {
	f := newMoveFinder(hunks, oldText, newText)
	cands, ok := f.candidates(ctx)
	if !ok {
		return nil
	}
	slices.SortFunc(cands, func(a, b candidate) int {
		return cmp.Or(cmp.Compare(b.length, a.length), cmp.Compare(a.old, b.old), cmp.Compare(a.new, b.new))
	})
	var moves []Move
	for i, c := range cands {
		if i%cancelCheckEvery == 0 && ctx.Err() != nil {
			return nil
		}
		moves = f.accept(moves, c)
	}
	slices.SortFunc(moves, func(a, b Move) int { return cmp.Compare(a.OldStart, b.OldStart) })
	return moves
}

func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	lines := strings.Split(text, textLineSeparator)
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func trimmedKeys(lines []string) []string {
	keys := make([]string, len(lines))
	for i, l := range lines {
		keys[i] = strings.TrimSpace(l)
	}
	return keys
}

func hunkIndex(n int, hunks []editor.DiffHunk, span func(editor.DiffHunk) (int, int)) []int {
	idx := make([]int, n)
	for i := range idx {
		idx[i] = noHunk
	}
	for hi, h := range hunks {
		from, to := span(h)
		for l := max(from, 0); l < min(to, n); l++ {
			idx[l] = hi
		}
	}
	return idx
}

func newMoveFinder(hunks []editor.DiffHunk, oldText, newText string) *moveFinder {
	oldKeys, newKeys := trimmedKeys(splitLines(oldText)), trimmedKeys(splitLines(newText))
	return &moveFinder{
		hunks:   hunks,
		oldKeys: oldKeys,
		newKeys: newKeys,
		oldHunk: hunkIndex(len(oldKeys), hunks, func(h editor.DiffHunk) (int, int) { return h.OldStart, h.OldEnd }),
		newHunk: hunkIndex(len(newKeys), hunks, func(h editor.DiffHunk) (int, int) { return h.NewStart, h.NewEnd }),
		usedOld: make([]bool, len(oldKeys)),
		usedNew: make([]bool, len(newKeys)),
	}
}

func (f *moveFinder) pairs(i, j int) bool {
	if i < 0 || j < 0 || i >= len(f.oldKeys) || j >= len(f.newKeys) {
		return false
	}
	hi, hj := f.oldHunk[i], f.newHunk[j]
	if hi == noHunk || hj == noHunk || f.oldKeys[i] != f.newKeys[j] {
		return false
	}
	if hi == hj {
		h := f.hunks[hi]
		return i-h.OldStart != j-h.NewStart
	}
	return true
}

func (f *moveFinder) startsRun(i, j int) bool {
	for back := 1; f.pairs(i-back, j-back); back++ {
		if f.oldKeys[i-back] != blankKey {
			return false
		}
	}
	return true
}

func (f *moveFinder) candidates(ctx context.Context) ([]candidate, bool) {
	index := map[string][]int{}
	for j, key := range f.newKeys {
		if key != blankKey && f.newHunk[j] != noHunk {
			index[key] = append(index[key], j)
		}
	}
	var out []candidate
	for i, key := range f.oldKeys {
		if i%cancelCheckEvery == 0 && ctx.Err() != nil {
			return nil, false
		}
		if key == blankKey || f.oldHunk[i] == noHunk {
			continue
		}
		positions := index[key]
		if len(positions) > maxKeyOccurrences {
			continue
		}
		for _, j := range positions {
			if !f.pairs(i, j) || !f.startsRun(i, j) {
				continue
			}
			n := 1
			for f.pairs(i+n, j+n) {
				n++
			}
			out = append(out, candidate{old: i, new: j, length: n})
		}
	}
	return out, true
}

func (f *moveFinder) accept(moves []Move, c candidate) []Move {
	start := -1
	flush := func(end int) {
		if start < 0 {
			return
		}
		moves = f.take(moves, c, start, end)
		start = -1
	}
	for t := 0; t < c.length; t++ {
		if f.usedOld[c.old+t] || f.usedNew[c.new+t] {
			flush(t)
			continue
		}
		if start < 0 {
			start = t
		}
	}
	flush(c.length)
	return moves
}

func (f *moveFinder) take(moves []Move, c candidate, from, to int) []Move {
	for from < to && f.oldKeys[c.old+from] == blankKey {
		from++
	}
	for to > from && f.oldKeys[c.old+to-1] == blankKey {
		to--
	}
	if !f.qualifies(c.old+from, c.old+to) {
		return moves
	}
	for t := from; t < to; t++ {
		f.usedOld[c.old+t], f.usedNew[c.new+t] = true, true
	}
	return append(moves, Move{OldStart: c.old + from, OldEnd: c.old + to, NewStart: c.new + from, NewEnd: c.new + to})
}

func (f *moveFinder) qualifies(from, to int) bool {
	lines, chars := 0, 0
	for _, key := range f.oldKeys[from:to] {
		if key == blankKey {
			continue
		}
		lines++
		for _, r := range key {
			if !unicode.IsSpace(r) {
				chars++
			}
		}
	}
	return lines >= MinMoveLines || chars >= MinMoveChars
}
