package largefile

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

const needle = "NeedleXYZ"

func TestFindAcrossBoundaries(t *testing.T) {
	data := bytes.Repeat([]byte("filler line\n"), 3*findChunk/len("filler line\n"))
	positions := []int{5, blockSize - 3, 5*blockSize - 1, findChunk - 4, 2*findChunk - 1, len(data) - len(needle)}
	for _, p := range positions {
		copy(data[p:], needle)
	}
	d := openTemp(t, data)
	ctx := context.Background()
	var got []int64
	for from := int64(0); ; {
		start, end, found, err := d.Find(ctx, from, needle, true, false)
		if err != nil {
			t.Fatal(err)
		}
		if !found {
			break
		}
		if end-start != int64(len(needle)) {
			t.Fatalf("match %d..%d has the wrong length", start, end)
		}
		got = append(got, start)
		from = start + 1
	}
	if len(got) != len(positions) {
		t.Fatalf("forward matches %v, want %v", got, positions)
	}
	for i, p := range positions {
		if got[i] != int64(p) {
			t.Fatalf("forward matches %v, want %v", got, positions)
		}
	}
	got = got[:0]
	for from := d.Len(); ; {
		start, _, found, err := d.Find(ctx, from, needle, true, true)
		if err != nil {
			t.Fatal(err)
		}
		if !found {
			break
		}
		got = append(got, start)
		from = start
	}
	for i, p := range positions {
		if got[len(got)-1-i] != int64(p) {
			t.Fatalf("backward matches %v, want %v reversed", got, positions)
		}
	}
}

func TestFindCaseAndEdits(t *testing.T) {
	d := openTemp(t, bytes.Repeat([]byte("abc\n"), blockSize))
	ctx := context.Background()
	if _, _, found, _ := d.Find(ctx, 0, "needlexyz", true, false); found {
		t.Fatal("found a pattern that is not there")
	}
	mid := d.Len() / 2
	d.Insert(mid, "leXYZ")
	d.Insert(mid, "Need")
	start, _, found, err := d.Find(ctx, 0, "needlexyz", false, false)
	if err != nil || !found || start != mid {
		t.Fatalf("case-insensitive find across pieces = %d, %v, %v; want %d", start, found, err, mid)
	}
	if _, _, found, _ := d.Find(ctx, 0, "needlexyz", true, false); found {
		t.Fatal("case-sensitive find ignored case")
	}
	if _, _, found, err := d.Find(ctx, 0, "", true, false); found || err != nil {
		t.Fatalf("empty pattern: %v, %v", found, err)
	}
	if _, _, _, err := d.Find(ctx, 0, string(make([]byte, maxPattern+1)), true, false); err == nil {
		t.Fatal("accepted a pattern over the limit")
	}
}

func TestFindCanceled(t *testing.T) {
	d := openTemp(t, bytes.Repeat([]byte("abc\n"), blockSize))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, _, err := d.Find(ctx, 0, needle, true, false); !errors.Is(err, context.Canceled) {
		t.Fatalf("forward: %v", err)
	}
	if _, _, _, err := d.Find(ctx, d.Len(), needle, true, true); !errors.Is(err, context.Canceled) {
		t.Fatalf("backward: %v", err)
	}
}
