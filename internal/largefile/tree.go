package largefile

import (
	"slices"
	"sort"
)

const chunkPieces = 64

type piece struct {
	src     *source
	off     int64
	n       int64
	nl      int64
	counted bool
}

func (p piece) end() int64 { return p.off + p.n }

type chunk struct {
	pieces    []piece
	n         int64
	nl        int64
	uncounted int
}

func newChunk(ps []piece) *chunk {
	c := &chunk{pieces: ps}
	for _, p := range ps {
		c.n += p.n
		if p.counted {
			c.nl += p.nl
		} else {
			c.uncounted++
		}
	}
	return c
}

type tree struct {
	id        uint64
	chunks    []*chunk
	starts    []int64
	lines     []int64
	uncounted int
}

func newTree(id uint64, chunks []*chunk) *tree {
	t := &tree{id: id, chunks: chunks, starts: make([]int64, len(chunks)+1), lines: make([]int64, len(chunks)+1)}
	for i, c := range chunks {
		t.starts[i+1] = t.starts[i] + c.n
		t.lines[i+1] = t.lines[i] + c.nl
		t.uncounted += c.uncounted
	}
	return t
}

func (t *tree) len() int64 { return t.starts[len(t.chunks)] }

func (t *tree) exact() bool { return t.uncounted == 0 }

func (t *tree) chunkAt(off int64) int {
	i := sort.Search(len(t.chunks), func(i int) bool { return t.starts[i+1] > off })
	return min(i, len(t.chunks)-1)
}

func (t *tree) replace(id uint64, start, end int64, ins []piece, cut func(piece, int64) (piece, piece)) *tree {
	if len(t.chunks) == 0 {
		return newTree(id, rechunk(join(ins)))
	}
	ci := t.chunkAt(start)
	cj := t.chunkAt(max(start, end-1))
	pos := t.starts[ci]
	var left, right []piece
	for _, c := range t.chunks[ci : cj+1] {
		for _, p := range c.pieces {
			pEnd := pos + p.n
			switch {
			case pEnd <= start:
				left = append(left, p)
			case pos >= end:
				right = append(right, p)
			default:
				if pos < start {
					l, _ := cut(p, start-pos)
					left = append(left, l)
				}
				if pEnd > end {
					_, r := cut(p, end-pos)
					right = append(right, r)
				}
			}
			pos = pEnd
		}
	}
	mid := rechunk(join(slices.Concat(left, ins, right)))
	return newTree(id, slices.Concat(t.chunks[:ci], mid, t.chunks[cj+1:]))
}

func (t *tree) settle(count func(piece) (int64, bool)) *tree {
	chunks := slices.Clone(t.chunks)
	for i, c := range chunks {
		if c.uncounted == 0 {
			continue
		}
		ps := slices.Clone(c.pieces)
		for j, p := range ps {
			if !p.counted {
				ps[j].nl, ps[j].counted = count(p)
			}
		}
		chunks[i] = newChunk(ps)
	}
	return newTree(t.id, chunks)
}

func join(ps []piece) []piece {
	out := make([]piece, 0, len(ps))
	for _, p := range ps {
		if p.n == 0 {
			continue
		}
		if k := len(out) - 1; k >= 0 && out[k].src == p.src && out[k].end() == p.off && out[k].counted == p.counted {
			out[k].n += p.n
			out[k].nl += p.nl
			continue
		}
		out = append(out, p)
	}
	return out
}

func rechunk(ps []piece) []*chunk {
	if len(ps) == 0 {
		return nil
	}
	k := (len(ps) + chunkPieces - 1) / chunkPieces
	out := make([]*chunk, 0, k)
	for i := range k {
		out = append(out, newChunk(slices.Clone(ps[i*len(ps)/k:(i+1)*len(ps)/k])))
	}
	return out
}
