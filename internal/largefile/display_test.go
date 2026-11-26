package largefile

import (
	"strings"
	"testing"
)

func TestDisplayMapsBytesToUnits(t *testing.T) {
	raw := "a\tb\xffé😀\x01"
	d := newDisplay(raw, false)
	want := "a   b" + replacement + "é😀" + replacement
	if d.text != want {
		t.Fatalf("text %q, want %q", d.text, want)
	}
	cases := []struct{ off, unit int }{{0, 0}, {1, 1}, {2, 4}, {3, 5}, {4, 6}, {6, 7}, {10, 9}, {11, 10}}
	for _, c := range cases {
		if got := d.unitOf(c.off); got != c.unit {
			t.Errorf("unitOf(%d) = %d, want %d", c.off, got, c.unit)
		}
		if got := d.byteOf(c.unit); got != c.off {
			t.Errorf("byteOf(%d) = %d, want %d", c.unit, got, c.off)
		}
	}
	if d.cut || d.shown() != len(raw) {
		t.Fatalf("cut %v, shown %d", d.cut, d.shown())
	}
}

func TestDisplayCutsLongLines(t *testing.T) {
	d := newDisplay(strings.Repeat("x", maxDisplayRunes+10), false)
	if !d.cut || d.shown() != maxDisplayRunes || len(d.text) != maxDisplayRunes {
		t.Fatalf("cut %v, shown %d, text %d", d.cut, d.shown(), len(d.text))
	}
}

func TestNavigationHandlesCRLFAndLongLines(t *testing.T) {
	long := strings.Repeat("y", rawLineCap+100)
	text := "one\r\ntwo\n" + long + "\nlast"
	d := openTemp(t, []byte(text))
	waitIndexed(t, d)
	li, err := readLine(d, 0)
	if err != nil || li.raw != "one" || li.end != 3 || li.next != 5 {
		t.Fatalf("line 0: %q end %d next %d err %v", li.raw, li.end, li.next, err)
	}
	longStart := int64(len("one\r\ntwo\n"))
	li, err = readLine(d, longStart)
	if err != nil || !li.cut || len(li.raw) != rawLineCap || li.end != longStart+int64(len(long)) || li.next != li.end+1 {
		t.Fatalf("long line: cut %v raw %d end %d next %d err %v", li.cut, len(li.raw), li.end, li.next, err)
	}
	li, err = readLine(d, li.next)
	if err != nil || li.raw != "last" || li.hasNext() || li.end != d.Len() {
		t.Fatalf("last line: %q next %d end %d err %v", li.raw, li.next, li.end, err)
	}
	if got := lineEndOf(d, 1); got != 3 {
		t.Fatalf("lineEndOf(1) = %d, want 3", got)
	}
	if got := lineStartOf(d, longStart+int64(len(long))); got != longStart {
		t.Fatalf("lineStartOf(long end) = %d, want %d", got, longStart)
	}
	if got := prevBoundary(d, 5); got != 3 {
		t.Fatalf("prevBoundary over CRLF = %d, want 3", got)
	}
	if got := nextBoundary(d, 3); got != 5 {
		t.Fatalf("nextBoundary over CRLF = %d, want 5", got)
	}
	if next, ok := nextLineStart(d, d.Len()-1); ok {
		t.Fatalf("nextLineStart on the last line = %d", next)
	}
}
