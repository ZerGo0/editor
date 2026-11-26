package hexdump

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	BytesPerRow = 16
	GroupSize   = 8
	PageSize    = 1 << 16
	MinDigits   = 8
	HexWidth    = BytesPerRow*hexByteWidth + BytesPerRow/GroupSize - 2
	ColumnGap   = 2

	hexByteWidth  = 3
	groupWidth    = GroupSize*hexByteWidth + 1
	hexRadix      = 16
	decimalRadix  = 10
	hexPrefix     = "0x"
	printableLow  = 0x20
	printableHigh = 0x7e
	placeholder   = '.'
	offsetBits    = 63
)

var (
	ErrOffsetSyntax = errors.New("enter a decimal offset or a hex offset such as 0x1f")
	ErrOffsetRange  = errors.New("offset is past the end of the file")
)

func Rows(size int64) int64 { return (size + BytesPerRow - 1) / BytesPerRow }

func RowOf(off int64) int64 { return off / BytesPerRow }

func RowBounds(row, size int64) (start, end int64) {
	start = row * BytesPerRow
	return start, min(start+BytesPerRow, size)
}

func PageOf(off int64) int64 { return off / PageSize }

func PageCount(size int64) int64 { return (size + PageSize - 1) / PageSize }

func PageBounds(page, size int64) (start, end int64) {
	start = page * PageSize
	return start, min(start+PageSize, size)
}

func PagesFor(firstRow, rows, size int64) (first, last int64, ok bool) {
	total := Rows(size)
	if rows <= 0 || firstRow < 0 || firstRow >= total {
		return 0, 0, false
	}
	lastRow := min(firstRow+rows, total) - 1
	return PageOf(firstRow * BytesPerRow), PageOf(lastRow * BytesPerRow), true
}

func OffsetDigits(size int64) int {
	if size <= 1 {
		return MinDigits
	}
	return max(MinDigits, len(strconv.FormatInt(size-1, hexRadix)))
}

func FormatOffset(off int64, digits int) string {
	return fmt.Sprintf("%0*x", digits, off)
}

func HexColumn(i int) int { return i*hexByteWidth + i/GroupSize }

func FormatHex(data []byte) string {
	const hexDigits = "0123456789abcdef"
	out := make([]byte, HexWidth)
	for i := range out {
		out[i] = ' '
	}
	for i := 0; i < min(len(data), BytesPerRow); i++ {
		col := HexColumn(i)
		out[col] = hexDigits[data[i]>>4]
		out[col+1] = hexDigits[data[i]&0x0f]
	}
	return string(out)
}

func FormatASCII(data []byte) string {
	out := make([]byte, len(data))
	for i, b := range data {
		out[i] = placeholder
		if b >= printableLow && b <= printableHigh {
			out[i] = b
		}
	}
	return string(out)
}

func HexString(data []byte) string {
	const hexDigits = "0123456789abcdef"
	var sb strings.Builder
	sb.Grow(len(data) * hexByteWidth)
	for i, b := range data {
		if i > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteByte(hexDigits[b>>4])
		sb.WriteByte(hexDigits[b&0x0f])
	}
	return sb.String()
}

type Layout struct{ Digits int }

type Hit struct {
	Index int
	ASCII bool
}

func (l Layout) HexStart() int { return l.Digits + ColumnGap }

func (l Layout) ASCIIStart() int { return l.HexStart() + HexWidth + ColumnGap }

func (l Layout) Width() int { return l.ASCIIStart() + BytesPerRow }

func (l Layout) Hit(col int) (Hit, bool) {
	if rel := col - l.ASCIIStart(); rel >= 0 && rel < BytesPerRow {
		return Hit{Index: rel, ASCII: true}, true
	}
	rel := col - l.HexStart()
	if rel < 0 || rel >= HexWidth {
		return Hit{}, false
	}
	group, within := rel/groupWidth, rel%groupWidth
	return Hit{Index: group*GroupSize + min(within/hexByteWidth, GroupSize-1)}, true
}

func ParseOffset(text string, size int64) (int64, error) {
	text = strings.TrimSpace(text)
	radix := decimalRadix
	if len(text) >= len(hexPrefix) && strings.EqualFold(text[:len(hexPrefix)], hexPrefix) {
		text, radix = text[len(hexPrefix):], hexRadix
	}
	off, err := strconv.ParseUint(text, radix, offsetBits)
	if err != nil {
		return 0, ErrOffsetSyntax
	}
	if int64(off) >= size {
		return 0, fmt.Errorf("%w (last offset %d)", ErrOffsetRange, max(size-1, 0))
	}
	return int64(off), nil
}

func Range(anchor, caret, size int64) (start, end int64, ok bool) {
	if size <= 0 {
		return 0, 0, false
	}
	start = max(min(anchor, caret), 0)
	end = min(max(anchor, caret)+1, size)
	return start, end, start < end
}
