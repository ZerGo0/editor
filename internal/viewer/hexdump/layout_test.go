package hexdump

import (
	"errors"
	"testing"
)

func TestRowsAndPages(t *testing.T) {
	tests := []struct {
		size  int64
		rows  int64
		pages int64
	}{
		{0, 0, 0},
		{1, 1, 1},
		{16, 1, 1},
		{17, 2, 1},
		{PageSize, PageSize / BytesPerRow, 1},
		{PageSize + 1, PageSize/BytesPerRow + 1, 2},
		{1 << 30, 1 << 26, 1 << 14},
	}
	for _, tt := range tests {
		if got := Rows(tt.size); got != tt.rows {
			t.Errorf("Rows(%d) = %d, want %d", tt.size, got, tt.rows)
		}
		if got := PageCount(tt.size); got != tt.pages {
			t.Errorf("PageCount(%d) = %d, want %d", tt.size, got, tt.pages)
		}
	}
}

func TestRowBoundsAndPageBounds(t *testing.T) {
	if s, e := RowBounds(1, 20); s != 16 || e != 20 {
		t.Errorf("RowBounds(1, 20) = %d, %d", s, e)
	}
	if s, e := PageBounds(1, PageSize+5); s != PageSize || e != PageSize+5 {
		t.Errorf("PageBounds(1, ...) = %d, %d", s, e)
	}
	if RowOf(31) != 1 || RowOf(32) != 2 {
		t.Error("RowOf does not floor to the row")
	}
	if PageOf(PageSize-1) != 0 || PageOf(PageSize) != 1 {
		t.Error("PageOf does not floor to the page")
	}
}

func TestPagesFor(t *testing.T) {
	rowsPerPage := int64(PageSize / BytesPerRow)
	size := int64(PageSize*3 + 100)
	tests := []struct {
		name        string
		first, rows int64
		wantFirst   int64
		wantLast    int64
		wantOK      bool
	}{
		{"one page", 0, 10, 0, 0, true},
		{"crosses a page", rowsPerPage - 2, 5, 0, 1, true},
		{"tail rows are clipped", rowsPerPage*3 + 1, 100, 3, 3, true},
		{"row past the end", rowsPerPage*3 + 100, 1, 0, 0, false},
		{"no rows", 0, 0, 0, 0, false},
		{"negative row", -1, 3, 0, 0, false},
	}
	for _, tt := range tests {
		first, last, ok := PagesFor(tt.first, tt.rows, size)
		if first != tt.wantFirst || last != tt.wantLast || ok != tt.wantOK {
			t.Errorf("%s: PagesFor = %d, %d, %v; want %d, %d, %v", tt.name, first, last, ok, tt.wantFirst, tt.wantLast, tt.wantOK)
		}
	}
}

func TestOffsetDigits(t *testing.T) {
	tests := []struct {
		size int64
		want int
	}{{0, MinDigits}, {1, MinDigits}, {1 << 32, MinDigits}, {1<<32 + 1, 9}, {1 << 40, 10}}
	for _, tt := range tests {
		if got := OffsetDigits(tt.size); got != tt.want {
			t.Errorf("OffsetDigits(%d) = %d, want %d", tt.size, got, tt.want)
		}
	}
	if got := FormatOffset(0x1f, 8); got != "0000001f" {
		t.Errorf("FormatOffset = %q", got)
	}
}

func TestFormatRow(t *testing.T) {
	data := []byte("\x7fELF\x02\x01\x01\x00ABCDEFGH")
	want := "7f 45 4c 46 02 01 01 00  41 42 43 44 45 46 47 48"
	if got := FormatHex(data); got != want {
		t.Errorf("FormatHex = %q, want %q", got, want)
	}
	if got := FormatASCII(data); got != ".ELF....ABCDEFGH" {
		t.Errorf("FormatASCII = %q", got)
	}
	short := FormatHex([]byte{0xab})
	if len(short) != HexWidth || short[:2] != "ab" || short[2:] != spaces(HexWidth-2) {
		t.Errorf("FormatHex of a short row = %q", short)
	}
	if got := HexString([]byte{1, 0xff, 0x10}); got != "01 ff 10" {
		t.Errorf("HexString = %q", got)
	}
	if len(want) != HexWidth {
		t.Errorf("HexWidth = %d, row is %d wide", HexWidth, len(want))
	}
}

func spaces(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = ' '
	}
	return string(b)
}

func TestHexColumnMatchesFormat(t *testing.T) {
	data := make([]byte, BytesPerRow)
	for i := range data {
		data[i] = byte(0xa0 + i)
	}
	row := FormatHex(data)
	for i, b := range data {
		col := HexColumn(i)
		if got := row[col : col+2]; got != HexString([]byte{b}) {
			t.Errorf("byte %d at column %d is %q", i, col, got)
		}
	}
}

func TestLayoutHit(t *testing.T) {
	l := Layout{Digits: MinDigits}
	tests := []struct {
		col  int
		want Hit
		ok   bool
	}{
		{0, Hit{}, false},
		{l.HexStart() - 1, Hit{}, false},
		{l.HexStart(), Hit{Index: 0}, true},
		{l.HexStart() + 2, Hit{Index: 0}, true},
		{l.HexStart() + 3, Hit{Index: 1}, true},
		{l.HexStart() + HexColumn(7), Hit{Index: 7}, true},
		{l.HexStart() + HexColumn(8), Hit{Index: 8}, true},
		{l.HexStart() + HexColumn(15) + 1, Hit{Index: 15}, true},
		{l.HexStart() + HexWidth, Hit{}, false},
		{l.ASCIIStart(), Hit{Index: 0, ASCII: true}, true},
		{l.ASCIIStart() + 15, Hit{Index: 15, ASCII: true}, true},
		{l.ASCIIStart() + 16, Hit{}, false},
		{-3, Hit{}, false},
	}
	for _, tt := range tests {
		got, ok := l.Hit(tt.col)
		if ok != tt.ok || got != tt.want {
			t.Errorf("Hit(%d) = %+v, %v; want %+v, %v", tt.col, got, ok, tt.want, tt.ok)
		}
	}
	if l.Width() != l.ASCIIStart()+BytesPerRow {
		t.Error("Width does not end after the text column")
	}
}

func TestParseOffset(t *testing.T) {
	tests := []struct {
		in      string
		size    int64
		want    int64
		wantErr error
	}{
		{"0", 10, 0, nil},
		{" 9 ", 10, 9, nil},
		{"0x1F", 100, 31, nil},
		{"0X10", 100, 16, nil},
		{"10", 10, 0, ErrOffsetRange},
		{"0", 0, 0, ErrOffsetRange},
		{"", 10, 0, ErrOffsetSyntax},
		{"0x", 10, 0, ErrOffsetSyntax},
		{"1f", 100, 0, ErrOffsetSyntax},
		{"-1", 100, 0, ErrOffsetSyntax},
		{"99999999999999999999999", 100, 0, ErrOffsetSyntax},
	}
	for _, tt := range tests {
		got, err := ParseOffset(tt.in, tt.size)
		if !errors.Is(err, tt.wantErr) || got != tt.want {
			t.Errorf("ParseOffset(%q, %d) = %d, %v; want %d, %v", tt.in, tt.size, got, err, tt.want, tt.wantErr)
		}
	}
}

func TestRange(t *testing.T) {
	tests := []struct {
		anchor, caret, size int64
		start, end          int64
		ok                  bool
	}{
		{3, 3, 10, 3, 4, true},
		{5, 2, 10, 2, 6, true},
		{2, 50, 10, 2, 10, true},
		{0, 0, 0, 0, 0, false},
		{-4, 1, 10, 0, 2, true},
	}
	for _, tt := range tests {
		start, end, ok := Range(tt.anchor, tt.caret, tt.size)
		if start != tt.start || end != tt.end || ok != tt.ok {
			t.Errorf("Range(%d, %d, %d) = %d, %d, %v", tt.anchor, tt.caret, tt.size, start, end, ok)
		}
	}
}
