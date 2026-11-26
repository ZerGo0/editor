package largefile

import (
	"bytes"
	"context"
	"errors"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const (
	testSeed      = 7
	indexWait     = 10 * time.Second
	indexPoll     = time.Millisecond
	longLineBytes = 3 * blockSize / 2
)

func writeTemp(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "big.txt")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func genText(rng *rand.Rand, size int) []byte {
	words := []string{"alpha", "beta", "gamma", "δέλτα", "日本", "\t", " ", "x"}
	var b bytes.Buffer
	for b.Len() < size {
		switch rng.IntN(40) {
		case 0:
			b.Write(bytes.Repeat([]byte("L"), longLineBytes))
			b.WriteByte('\n')
		case 1:
			b.WriteString("\r\n")
		case 2:
			b.WriteString("\n\n")
		default:
			for range rng.IntN(12) {
				b.WriteString(words[rng.IntN(len(words))])
			}
			b.WriteByte('\n')
		}
	}
	return b.Bytes()
}

func openTemp(t *testing.T, data []byte) *Doc {
	t.Helper()
	d, err := Open(writeTemp(t, data))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

func waitIndexed(t *testing.T, d *Doc) {
	t.Helper()
	deadline := time.Now().Add(indexWait)
	for !d.base.idx.isDone() {
		if _, _, err := d.Indexing(); err != nil {
			t.Fatal(err)
		}
		if time.Now().After(deadline) {
			t.Fatal("index did not finish")
		}
		time.Sleep(indexPoll)
	}
}

func content(t *testing.T, d *Doc) []byte {
	t.Helper()
	buf := make([]byte, d.Len())
	if n, err := d.ReadAt(buf, 0); err != nil && n != len(buf) {
		t.Fatalf("read %d of %d: %v", n, len(buf), err)
	}
	return buf
}

func checkModel(t *testing.T, rng *rand.Rand, d *Doc, model []byte) {
	t.Helper()
	if got := content(t, d); !bytes.Equal(got, model) {
		t.Fatalf("content differs: len %d, want %d", len(got), len(model))
	}
	n, exact := d.LineCount()
	want := int64(bytes.Count(model, newline)) + 1
	if !exact || n != want {
		t.Fatalf("LineCount = %d, %v; want %d, true", n, exact, want)
	}
	starts := []int64{0}
	for i, c := range model {
		if c == '\n' {
			starts = append(starts, int64(i)+1)
		}
	}
	for range 50 {
		line := rng.Int64N(int64(len(starts)))
		if got, ok := d.LineStart(line); !ok || got != starts[line] {
			t.Fatalf("LineStart(%d) = %d, %v; want %d", line, got, ok, starts[line])
		}
		off := rng.Int64N(int64(len(model)) + 1)
		wantLine := int64(bytes.Count(model[:off], newline))
		if got, ok := d.LineOf(off); !ok || got != wantLine {
			t.Fatalf("LineOf(%d) = %d, %v; want %d", off, got, ok, wantLine)
		}
	}
}

type modelEdit struct {
	start    int64
	removed  []byte
	inserted []byte
}

func applyModel(model []byte, e modelEdit) []byte {
	out := append([]byte{}, model[:e.start]...)
	out = append(out, e.inserted...)
	return append(out, model[e.start+int64(len(e.removed)):]...)
}

func TestRandomEditsMatchModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(testSeed, testSeed))
	model := genText(rng, 6*blockSize)
	d := openTemp(t, model)
	waitIndexed(t, d)
	checkModel(t, rng, d, model)
	var undo, redo []modelEdit
	for i := range 600 {
		d.breakGroup()
		switch op := rng.IntN(10); {
		case op < 2 && len(undo) > 0:
			e := undo[len(undo)-1]
			undo = undo[:len(undo)-1]
			model = applyModel(model, modelEdit{start: e.start, removed: e.inserted, inserted: e.removed})
			redo = append(redo, e)
			start, end, ok := d.Undo()
			if !ok || start != e.start || end != e.start+int64(len(e.removed)) {
				t.Fatalf("op %d: Undo = %d, %d, %v; want %d, %d", i, start, end, ok, e.start, e.start+int64(len(e.removed)))
			}
		case op < 3 && len(redo) > 0:
			e := redo[len(redo)-1]
			redo = redo[:len(redo)-1]
			model = applyModel(model, e)
			undo = append(undo, e)
			if _, _, ok := d.Redo(); !ok {
				t.Fatalf("op %d: Redo failed", i)
			}
		default:
			start := rng.Int64N(int64(len(model)) + 1)
			end := start
			if op%2 == 0 {
				end = min(int64(len(model)), start+rng.Int64N(2*blockSize))
			}
			ins := genText(rng, rng.IntN(64))
			if op == 9 {
				ins = nil
			}
			if start == end && len(ins) == 0 {
				continue
			}
			e := modelEdit{start: start, removed: append([]byte{}, model[start:end]...), inserted: ins}
			model = applyModel(model, e)
			undo = append(undo, e)
			redo = nil
			d.Replace(start, end, string(ins))
		}
		if i%40 == 0 {
			checkModel(t, rng, d, model)
		}
	}
	checkModel(t, rng, d, model)
}

func TestEditsBeforeIndexFinishes(t *testing.T) {
	rng := rand.New(rand.NewPCG(testSeed, 1))
	model := genText(rng, 4*blockSize)
	d, err := openDoc(writeTemp(t, model))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if _, exact := d.LineCount(); exact {
		t.Fatal("LineCount is exact before indexing")
	}
	if line, ok := d.LineOf(0); !ok || line != 0 {
		t.Fatalf("LineOf(0) = %d, %v", line, ok)
	}
	if _, ok := d.LineStart(1); ok {
		t.Fatal("LineStart(1) is known before indexing")
	}
	d.Insert(0, "a\nb\n")
	if got, ok := d.LineStart(2); !ok || got != 4 {
		t.Fatalf("LineStart(2) = %d, %v; want 4", got, ok)
	}
	mid := int64(len(model) / 2)
	d.Delete(mid, mid+10)
	model = append([]byte("a\nb\n"), model...)
	model = append(model[:mid:mid], model[mid+10:]...)
	d.base.startIndex(d.notifyIndex)
	waitIndexed(t, d)
	checkModel(t, rng, d, model)
}

func TestTypingGroupsUndo(t *testing.T) {
	d := openTemp(t, []byte("hello\n"))
	for i, c := range "abc" {
		d.Insert(int64(i), string(c))
	}
	for end := int64(3); end > 0; end-- {
		d.Delete(end-1, end)
	}
	d.Insert(0, "xyz")
	d.Delete(0, 3)
	if got := string(content(t, d)); got != "hello\n" {
		t.Fatalf("content %q", got)
	}
	steps := 0
	for {
		if _, _, ok := d.Undo(); !ok {
			break
		}
		steps++
	}
	if got := string(content(t, d)); got != "hello\n" || d.Modified() {
		t.Fatalf("after undo: %q, modified %v", got, d.Modified())
	}
	if steps != 4 {
		t.Fatalf("undo steps = %d, want 4", steps)
	}
	if _, _, ok := d.Redo(); !ok || string(content(t, d)) != "abchello\n" {
		t.Fatalf("redo: %q", content(t, d))
	}
}

func TestSliceBoundsAndShrunkFile(t *testing.T) {
	rng := rand.New(rand.NewPCG(testSeed, 2))
	data := genText(rng, 4*blockSize)
	path := writeTemp(t, data)
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	waitIndexed(t, d)
	if s, err := d.Slice(-5, 10); err != nil || s != string(data[:10]) {
		t.Fatalf("Slice(-5, 10) = %q, %v", s, err)
	}
	if err := os.Truncate(path, blockSize); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Slice(int64(len(data))-10, int64(len(data))); !errors.Is(err, errChanged) {
		t.Fatalf("read past truncation: %v", err)
	}
}

func TestOpenRejectsDirectoryAndMissingFile(t *testing.T) {
	dir := t.TempDir()
	if _, err := Open(dir); err == nil {
		t.Fatal("opened a directory")
	}
	if _, err := Open(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("opened a missing file")
	}
}

func TestEmptyFile(t *testing.T) {
	d := openTemp(t, nil)
	waitIndexed(t, d)
	if n, exact := d.LineCount(); n != 1 || !exact || d.Len() != 0 {
		t.Fatalf("LineCount = %d, %v; Len %d", n, exact, d.Len())
	}
	d.Insert(0, "one\ntwo")
	if got, ok := d.LineStart(1); !ok || got != 4 {
		t.Fatalf("LineStart(1) = %d, %v", got, ok)
	}
}

func TestCloseCancelsIndex(t *testing.T) {
	rng := rand.New(rand.NewPCG(testSeed, 3))
	d, err := Open(writeTemp(t, genText(rng, 4*indexChunk)))
	if err != nil {
		t.Fatal(err)
	}
	idx := d.base.idx
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-idx.finished:
	case <-time.After(indexWait):
		t.Fatal("index goroutine still runs after Close")
	}
	if _, err := d.Slice(0, 1); !errors.Is(err, errClosed) {
		t.Fatalf("Slice after Close: %v", err)
	}
	if err := d.Save(context.Background(), nil); !errors.Is(err, errClosed) {
		t.Fatalf("Save after Close: %v", err)
	}
}
