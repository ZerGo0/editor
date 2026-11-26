package largefile

import (
	"sort"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

const (
	maxDisplayRunes = 4096
	rawLineCap      = maxDisplayRunes * utf8.UTFMax
	tabWidth        = 4
	replacement     = "\uFFFD"
	asciiDelete     = 0x7f
	c1Last          = 0x9f
	lineSeparator   = 0x2028
	paraSeparator   = 0x2029
)

type display struct {
	text  string
	bytes []int32
	units []int32
	cut   bool
}

func newDisplay(raw string, cut bool) display {
	var b strings.Builder
	var d display
	col, units, runes, i := 0, 0, 0, 0
	for i < len(raw) && runes < maxDisplayRunes {
		d.bytes = append(d.bytes, int32(i))
		d.units = append(d.units, int32(units))
		r, size := utf8.DecodeRuneInString(raw[i:])
		switch {
		case r == '\t':
			n := tabWidth - col%tabWidth
			b.WriteString(strings.Repeat(" ", n))
			col += n
			units += n
		case r == utf8.RuneError && size == 1, hidden(r):
			b.WriteString(replacement)
			col++
			units++
		default:
			b.WriteString(raw[i : i+size])
			col++
			units += utf16.RuneLen(r)
		}
		i += size
		runes++
	}
	d.bytes = append(d.bytes, int32(i))
	d.units = append(d.units, int32(units))
	d.cut = cut || i < len(raw)
	d.text = b.String()
	return d
}

func hidden(r rune) bool {
	return r < ' ' || (r >= asciiDelete && r <= c1Last) || r == lineSeparator || r == paraSeparator
}

func (d display) shown() int { return int(d.bytes[len(d.bytes)-1]) }

func (d display) unitOf(off int) int {
	i := sort.Search(len(d.bytes), func(i int) bool { return int(d.bytes[i]) > off }) - 1
	return int(d.units[max(0, i)])
}

func (d display) byteOf(unit int) int {
	i := sort.Search(len(d.units), func(i int) bool { return int(d.units[i]) >= unit })
	return int(d.bytes[min(i, len(d.bytes)-1)])
}
