package largefile

import (
	"bytes"
	"context"
	"errors"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"
)

const savedPerm = 0o600

func stubFreeSpace(t *testing.T, free uint64) {
	t.Helper()
	old := freeSpace
	freeSpace = func(string) (uint64, error) { return free, nil }
	t.Cleanup(func() { freeSpace = old })
}

func dirEntries(t *testing.T, path string) int {
	t.Helper()
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	return len(entries)
}

func TestSaveRoundTrip(t *testing.T) {
	stubFreeSpace(t, math.MaxUint64)
	rng := rand.New(rand.NewPCG(testSeed, 4))
	model := genText(rng, 3*saveChunk)
	path := writeTemp(t, model)
	if err := os.Chmod(path, savedPerm); err != nil {
		t.Fatal(err)
	}
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	waitIndexed(t, d)
	d.Insert(10, "inserted\ntext")
	d.Delete(saveChunk, saveChunk+100)
	model = append(model[:10:10], append([]byte("inserted\ntext"), model[10:]...)...)
	model = append(model[:saveChunk:saveChunk], model[saveChunk+100:]...)
	var last int64
	if err := d.Save(context.Background(), func(done, total int64) { last = done }); err != nil {
		t.Fatal(err)
	}
	if last != int64(len(model)) {
		t.Fatalf("progress ended at %d, want %d", last, len(model))
	}
	disk, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(disk, model) {
		t.Fatalf("saved file differs: %v", err)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != savedPerm {
		t.Fatalf("mode after save: %v, %v", info.Mode(), err)
	}
	if d.Modified() {
		t.Fatal("modified after save")
	}
	if _, _, ok := d.Undo(); ok {
		t.Fatal("undo history kept across a rebase")
	}
	checkModel(t, rng, d, model)
	d.Insert(0, "again\n")
	model = append([]byte("again\n"), model...)
	if err := d.Save(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	waitIndexed(t, d)
	checkModel(t, rng, d, model)
	if disk, _ := os.ReadFile(path); !bytes.Equal(disk, model) {
		t.Fatal("second save differs")
	}
}

func TestSaveCanceledLeavesOriginal(t *testing.T) {
	stubFreeSpace(t, math.MaxUint64)
	rng := rand.New(rand.NewPCG(testSeed, 5))
	data := genText(rng, 3*saveChunk)
	path := writeTemp(t, data)
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	d.Insert(0, "edit")
	ctx, cancel := context.WithCancel(context.Background())
	err = d.Save(ctx, func(done, total int64) { cancel() })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Save = %v, want canceled", err)
	}
	if disk, _ := os.ReadFile(path); !bytes.Equal(disk, data) {
		t.Fatal("original changed by a canceled save")
	}
	if n := dirEntries(t, path); n != 1 {
		t.Fatalf("%d files in the directory, want only the original", n)
	}
	if !d.Modified() {
		t.Fatal("canceled save marked the document saved")
	}
}

func TestSaveNoSpace(t *testing.T) {
	stubFreeSpace(t, spaceMargin)
	data := []byte("small file\n")
	path := writeTemp(t, data)
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	d.Insert(0, "x")
	if err := d.Save(context.Background(), nil); !errors.Is(err, ErrNoSpace) {
		t.Fatalf("Save = %v, want ErrNoSpace", err)
	}
	if disk, _ := os.ReadFile(path); !bytes.Equal(disk, data) {
		t.Fatal("file written without space")
	}
	if n := dirEntries(t, path); n != 1 {
		t.Fatalf("%d files in the directory, want only the original", n)
	}
}

func TestSaveWithEditsDuringWrite(t *testing.T) {
	stubFreeSpace(t, math.MaxUint64)
	rng := rand.New(rand.NewPCG(testSeed, 6))
	model := genText(rng, 2*saveChunk)
	path := writeTemp(t, model)
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	waitIndexed(t, d)
	edited := false
	err = d.Save(context.Background(), func(done, total int64) {
		if !edited {
			edited = true
			d.Insert(0, "late\n")
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if disk, _ := os.ReadFile(path); !bytes.Equal(disk, model) {
		t.Fatal("saved file does not match the text when the save began")
	}
	if !d.Modified() {
		t.Fatal("edit made during the save is not marked modified")
	}
	model = append([]byte("late\n"), model...)
	checkModel(t, rng, d, model)
	if err := d.Save(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if disk, _ := os.ReadFile(path); !bytes.Equal(disk, model) || d.Modified() {
		t.Fatal("second save is wrong")
	}
}
