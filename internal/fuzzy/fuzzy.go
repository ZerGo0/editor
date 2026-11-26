package fuzzy

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	scoreMatch        = 16
	scoreGapStart     = -3
	scoreGapExtension = -1
	bonusBoundary     = scoreMatch / 2
	bonusCamel        = bonusBoundary - 1
	bonusConsecutive  = 4
	bonusFirstChar    = 2
	bonusLastSegment  = 3
	pathSeparator     = '/'
	boundaryChars     = "/\\_-. :"
	asciiCaseOffset   = 'a' - 'A'
)

type Match struct {
	Score     int
	Positions []int
}

type Result struct {
	Index int
	Match Match
}

type term struct {
	runes         []rune
	caseSensitive bool
}

func Find(pattern, text string) (Match, bool) {
	return matchText(parse(pattern), []rune(text))
}

func Filter(pattern string, n int, text func(i int) string) []Result {
	terms := parse(pattern)
	results := make([]Result, 0, n)
	if len(terms) == 0 {
		for i := 0; i < n; i++ {
			results = append(results, Result{Index: i})
		}
		return results
	}
	lengths := make([]int, 0, n)
	var buf []rune
	for i := 0; i < n; i++ {
		s := text(i)
		buf = append(buf[:0], []rune(s)...)
		if m, ok := matchText(terms, buf); ok {
			results = append(results, Result{Index: i, Match: m})
			lengths = append(lengths, len(s))
		}
	}
	order := &resultOrder{results: results, lengths: lengths}
	sort.Sort(order)
	return results
}

type resultOrder struct {
	results []Result
	lengths []int
}

func (o *resultOrder) Len() int { return len(o.results) }

func (o *resultOrder) Swap(i, j int) {
	o.results[i], o.results[j] = o.results[j], o.results[i]
	o.lengths[i], o.lengths[j] = o.lengths[j], o.lengths[i]
}

func (o *resultOrder) Less(i, j int) bool {
	a, b := o.results[i], o.results[j]
	if a.Match.Score != b.Match.Score {
		return a.Match.Score > b.Match.Score
	}
	if o.lengths[i] != o.lengths[j] {
		return o.lengths[i] < o.lengths[j]
	}
	return a.Index < b.Index
}

func parse(pattern string) []term {
	fields := strings.Fields(pattern)
	terms := make([]term, len(fields))
	for i, field := range fields {
		caseSensitive := strings.IndexFunc(field, unicode.IsUpper) >= 0
		runes := []rune(field)
		if !caseSensitive {
			for j, r := range runes {
				runes[j] = fold(r)
			}
		}
		terms[i] = term{runes: runes, caseSensitive: caseSensitive}
	}
	return terms
}

func fold(r rune) rune {
	if r < utf8.RuneSelf {
		if r >= 'A' && r <= 'Z' {
			return r + asciiCaseOffset
		}
		return r
	}
	return unicode.ToLower(unicode.ToUpper(r))
}

func matchText(terms []term, text []rune) (Match, bool) {
	if len(terms) == 0 {
		return Match{}, true
	}
	total := 0
	for _, t := range terms {
		total += len(t.runes)
	}
	positions := make([]int, total)
	lastSeparator := lastIndex(text, pathSeparator)
	score, offset := 0, 0
	for _, t := range terms {
		dst := positions[offset : offset+len(t.runes)]
		s, ok := matchTerm(t, text, lastSeparator, dst)
		if !ok {
			return Match{}, false
		}
		score += s
		offset += len(dst)
	}
	if len(terms) > 1 {
		sort.Ints(positions)
		positions = dedupe(positions)
	}
	return Match{Score: score, Positions: positions}, true
}

func matchTerm(t term, text []rune, lastSeparator int, dst []int) (int, bool) {
	last := len(t.runes) - 1
	i, end := 0, -1
	for j, r := range text {
		if t.equal(r, t.runes[i]) {
			if i == last {
				end = j
				break
			}
			i++
		}
	}
	if end < 0 {
		return 0, false
	}
	k := last
	for j := end; k >= 0; j-- {
		if t.equal(text[j], t.runes[k]) {
			dst[k] = j
			k--
		}
	}
	return score(text, dst, lastSeparator), true
}

func (t term) equal(r, want rune) bool {
	if t.caseSensitive {
		return r == want
	}
	return fold(r) == want
}

func score(text []rune, positions []int, lastSeparator int) int {
	total := 0
	prev := -1
	runBonus := 0
	for i, p := range positions {
		bonus := bonusAt(text, p)
		if prev >= 0 && p == prev+1 {
			bonus = max(bonus, runBonus, bonusConsecutive)
		} else {
			runBonus = bonus
			if gap := p - prev - 1; prev >= 0 && gap > 0 {
				total += scoreGapStart + (gap-1)*scoreGapExtension
			}
		}
		if i == 0 {
			bonus *= bonusFirstChar
		}
		if p > lastSeparator {
			total += bonusLastSegment
		}
		total += scoreMatch + bonus
		prev = p
	}
	return total
}

func bonusAt(text []rune, i int) int {
	if i == 0 {
		return bonusBoundary
	}
	prev, cur := text[i-1], text[i]
	switch {
	case strings.ContainsRune(boundaryChars, prev):
		return bonusBoundary
	case unicode.IsLower(prev) && unicode.IsUpper(cur):
		return bonusCamel
	case unicode.IsLetter(prev) && unicode.IsDigit(cur):
		return bonusCamel
	}
	return 0
}

func lastIndex(text []rune, r rune) int {
	for i := len(text) - 1; i >= 0; i-- {
		if text[i] == r {
			return i
		}
	}
	return -1
}

func dedupe(sorted []int) []int {
	out := sorted[:1]
	for _, v := range sorted[1:] {
		if v != out[len(out)-1] {
			out = append(out, v)
		}
	}
	return out
}
