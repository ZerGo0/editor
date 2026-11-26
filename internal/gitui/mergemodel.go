package gitui

import (
	"strings"

	"github.com/ZerGo0/editor/internal/git"
)

type mergeChoice int

const (
	choiceNone mergeChoice = iota
	choiceOurs
	choiceTheirs
	choiceBoth
)

type mergeSide int

const (
	sideOurs mergeSide = iota
	sideResult
	sideTheirs
)

type lineRange struct {
	start, end int
}

type mergeModel struct {
	chunks  []git.Chunk
	choices []mergeChoice
	eol     bool
}

func newMergeModel(base, ours, theirs string) *mergeModel {
	chunks := git.Merge3(git.SplitLines(base), git.SplitLines(ours), git.SplitLines(theirs))
	m := &mergeModel{
		chunks:  chunks,
		choices: make([]mergeChoice, len(chunks)),
		eol:     strings.HasSuffix(ours, "\n") || (ours == "" && strings.HasSuffix(theirs, "\n")),
	}
	m.applyNonConflicting(true, true)
	return m
}

func (m *mergeModel) applyNonConflicting(ours, theirs bool) {
	for i, ch := range m.chunks {
		switch {
		case ch.Kind == git.ChunkOurs && ours:
			m.choices[i] = choiceOurs
		case ch.Kind == git.ChunkTheirs && theirs:
			m.choices[i] = choiceTheirs
		}
	}
}

func (m *mergeModel) acceptAll(ours bool) {
	for i, ch := range m.chunks {
		if ch.Kind == git.ChunkStable {
			continue
		}
		if ours {
			m.choices[i] = choiceOurs
		} else {
			m.choices[i] = choiceTheirs
		}
	}
}

func (m *mergeModel) choose(i int, c mergeChoice) {
	if i < 0 || i >= len(m.chunks) || m.chunks[i].Kind == git.ChunkStable {
		return
	}
	switch {
	case m.choices[i] == choiceOurs && c == choiceTheirs, m.choices[i] == choiceTheirs && c == choiceOurs:
		if m.chunks[i].Kind == git.ChunkConflict {
			c = choiceBoth
		}
	}
	m.choices[i] = c
}

func (m *mergeModel) chunkLines(i int, side mergeSide) []string {
	ch := m.chunks[i]
	switch side {
	case sideOurs:
		return ch.Ours
	case sideTheirs:
		return ch.Theirs
	}
	if ch.Kind == git.ChunkStable {
		return ch.Ours
	}
	switch m.choices[i] {
	case choiceOurs:
		return ch.Ours
	case choiceTheirs:
		return ch.Theirs
	case choiceBoth:
		return append(append([]string(nil), ch.Ours...), ch.Theirs...)
	}
	return ch.Base
}

func (m *mergeModel) result() string {
	var lines []string
	for i := range m.chunks {
		lines = append(lines, m.chunkLines(i, sideResult)...)
	}
	if len(lines) == 0 {
		return ""
	}
	text := strings.Join(lines, "\n")
	if m.eol {
		text += "\n"
	}
	return text
}

func (m *mergeModel) ranges(side mergeSide) []lineRange {
	out := make([]lineRange, len(m.chunks))
	line := 0
	for i := range m.chunks {
		n := len(m.chunkLines(i, side))
		out[i] = lineRange{start: line, end: line + n}
		line += n
	}
	return out
}

func (m *mergeModel) unresolved() int {
	n := 0
	for i, ch := range m.chunks {
		if ch.Kind == git.ChunkConflict && m.choices[i] == choiceNone {
			n++
		}
	}
	return n
}

func (m *mergeModel) changes() []int {
	var idx []int
	for i, ch := range m.chunks {
		if ch.Kind != git.ChunkStable {
			idx = append(idx, i)
		}
	}
	return idx
}

func (m *mergeModel) clean() bool {
	for _, ch := range m.chunks {
		if ch.Kind == git.ChunkConflict {
			return false
		}
	}
	return true
}
