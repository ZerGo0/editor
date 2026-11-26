package git

import "strings"

type ChunkKind int

const (
	ChunkStable ChunkKind = iota
	ChunkOurs
	ChunkTheirs
	ChunkConflict
)

type Chunk struct {
	Kind   ChunkKind
	Base   []string
	Ours   []string
	Theirs []string
}

func SplitLines(text string) []string {
	if text == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func Merge3(base, ours, theirs []string) []Chunk {
	ids := internLines(base, ours, theirs)
	toOurs := baseMap(len(base), matchLines(ids[0], ids[1]))
	toTheirs := baseMap(len(base), matchLines(ids[0], ids[2]))
	var chunks []Chunk
	bi, oi, ti := 0, 0, 0
	region := func(b, o, t int) {
		if c, ok := classifyRegion(base[bi:b], ours[oi:o], theirs[ti:t]); ok {
			chunks = append(chunks, c)
		}
		bi, oi, ti = b, o, t
	}
	for i := 0; i < len(base); {
		if toOurs[i] < 0 || toTheirs[i] < 0 {
			i++
			continue
		}
		region(i, toOurs[i], toTheirs[i])
		j := i
		for j+1 < len(base) && toOurs[j+1] == toOurs[j]+1 && toTheirs[j+1] == toTheirs[j]+1 {
			j++
		}
		region(j+1, toOurs[j]+1, toTheirs[j]+1)
		i = j + 1
	}
	region(len(base), len(ours), len(theirs))
	return chunks
}

func baseMap(n int, pairs []linePair) []int {
	m := make([]int, n)
	for i := range m {
		m[i] = -1
	}
	for _, p := range pairs {
		m[p.a] = p.b
	}
	return m
}

func classifyRegion(base, ours, theirs []string) (Chunk, bool) {
	if len(base) == 0 && len(ours) == 0 && len(theirs) == 0 {
		return Chunk{}, false
	}
	c := Chunk{Base: base, Ours: ours, Theirs: theirs}
	switch {
	case equalLines(ours, theirs):
		c.Kind = ChunkStable
	case equalLines(base, ours):
		c.Kind = ChunkTheirs
	case equalLines(base, theirs):
		c.Kind = ChunkOurs
	default:
		c.Kind = ChunkConflict
	}
	return c, true
}

func equalLines(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
