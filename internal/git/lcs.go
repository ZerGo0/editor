package git

type linePair struct {
	a int
	b int
}

func internLines(seqs ...[]string) [][]int {
	ids := make(map[string]int)
	out := make([][]int, len(seqs))
	for i, seq := range seqs {
		out[i] = make([]int, len(seq))
		for j, line := range seq {
			id, ok := ids[line]
			if !ok {
				id = len(ids)
				ids[line] = id
			}
			out[i][j] = id
		}
	}
	return out
}

func matchLines(a, b []int) []linePair {
	var pairs []linePair
	lcsMatch(a, b, 0, 0, &pairs)
	return pairs
}

func lcsMatch(a, b []int, aOff, bOff int, out *[]linePair) {
	prefix := 0
	for prefix < len(a) && prefix < len(b) && a[prefix] == b[prefix] {
		*out = append(*out, linePair{aOff + prefix, bOff + prefix})
		prefix++
	}
	a, b = a[prefix:], b[prefix:]
	aOff, bOff = aOff+prefix, bOff+prefix
	suffix := 0
	for suffix < len(a) && suffix < len(b) && a[len(a)-1-suffix] == b[len(b)-1-suffix] {
		suffix++
	}
	var tail []linePair
	for i := suffix; i >= 1; i-- {
		tail = append(tail, linePair{aOff + len(a) - i, bOff + len(b) - i})
	}
	a, b = a[:len(a)-suffix], b[:len(b)-suffix]
	if len(a) > 0 && len(b) > 0 {
		x, y, ok := bisect(a, b)
		if ok && (x > 0 || y > 0) && (x < len(a) || y < len(b)) {
			lcsMatch(a[:x], b[:y], aOff, bOff, out)
			lcsMatch(a[x:], b[y:], aOff+x, bOff+y, out)
		}
	}
	*out = append(*out, tail...)
}

const unsetFrontier = -1

func bisect(a, b []int) (int, int, bool) {
	n, m := len(a), len(b)
	maxD := (n + m + 1) / 2
	offset := maxD + 1
	size := 2*maxD + 3
	v1 := make([]int, size)
	v2 := make([]int, size)
	for i := range v1 {
		v1[i], v2[i] = unsetFrontier, unsetFrontier
	}
	v1[offset+1], v2[offset+1] = 0, 0
	delta := n - m
	front := delta%2 != 0
	k1Start, k1End, k2Start, k2End := 0, 0, 0, 0
	for d := 0; d < maxD; d++ {
		for k1 := -d + k1Start; k1 <= d-k1End; k1 += 2 {
			idx := offset + k1
			var x1 int
			if k1 == -d || (k1 != d && v1[idx-1] < v1[idx+1]) {
				x1 = v1[idx+1]
			} else {
				x1 = v1[idx-1] + 1
			}
			y1 := x1 - k1
			for x1 < n && y1 < m && a[x1] == b[y1] {
				x1++
				y1++
			}
			v1[idx] = x1
			if x1 > n {
				k1End += 2
			} else if y1 > m {
				k1Start += 2
			} else if front {
				idx2 := offset + delta - k1
				if idx2 >= 0 && idx2 < size && v2[idx2] != unsetFrontier && x1 >= n-v2[idx2] {
					return x1, y1, true
				}
			}
		}
		for k2 := -d + k2Start; k2 <= d-k2End; k2 += 2 {
			idx := offset + k2
			var x2 int
			if k2 == -d || (k2 != d && v2[idx-1] < v2[idx+1]) {
				x2 = v2[idx+1]
			} else {
				x2 = v2[idx-1] + 1
			}
			y2 := x2 - k2
			for x2 < n && y2 < m && a[n-x2-1] == b[m-y2-1] {
				x2++
				y2++
			}
			v2[idx] = x2
			if x2 > n {
				k2End += 2
			} else if y2 > m {
				k2Start += 2
			} else if !front {
				idx1 := offset + delta - k2
				if idx1 >= 0 && idx1 < size && v1[idx1] != unsetFrontier {
					x1 := v1[idx1]
					y1 := offset + x1 - idx1
					if x1 >= n-x2 {
						return x1, y1, true
					}
				}
			}
		}
	}
	return 0, 0, false
}
