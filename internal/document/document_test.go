package document

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeFile(t *testing.T, name string, data []byte, perm os.FileMode) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, data, perm); err != nil {
		t.Fatal(err)
	}
	return p
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestOpenText(t *testing.T) {
	p := writeFile(t, "main.go", []byte("package main\n\nfunc main() {}\n"), 0o644)
	d, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	if d.Kind != Text || d.ReadOnly || d.Buffer == nil || !d.Editable() || d.Modified() {
		t.Fatalf("doc %+v", d)
	}
	if d.Path != p || d.Buffer.String() != "package main\n\nfunc main() {}\n" {
		t.Fatalf("content %q", d.Buffer.String())
	}
}

func TestOpenEmptyFile(t *testing.T) {
	d, err := Open(writeFile(t, "empty.txt", nil, 0o644))
	if err != nil {
		t.Fatal(err)
	}
	if d.Kind != Text || d.Buffer == nil {
		t.Fatalf("doc %+v", d)
	}
}

func TestOpenBinary(t *testing.T) {
	d, err := Open(writeFile(t, "a.bin", []byte("abc\x00def"), 0o644))
	if err != nil {
		t.Fatal(err)
	}
	if d.Kind != Binary || d.Buffer != nil || d.Editable() || d.Modified() {
		t.Fatalf("doc %+v", d)
	}
	if err := d.Save(); err == nil {
		t.Fatal("binary save should fail")
	}
}

func TestOpenTextAllowsBinary(t *testing.T) {
	d, err := OpenText(writeFile(t, "a.bin", []byte("abc\x00def"), 0o644))
	if err != nil {
		t.Fatal(err)
	}
	if d.Kind != Text || d.Buffer == nil || d.Buffer.String() != "abc\x00def" {
		t.Fatalf("doc %+v", d)
	}
}

func TestIsBinarySniffWindow(t *testing.T) {
	data := make([]byte, binarySniffSize+10)
	for i := range data {
		data[i] = 'a'
	}
	data[binarySniffSize+5] = 0
	if isBinary(data) {
		t.Fatal("NUL past sniff window must not count")
	}
	data[binarySniffSize-1] = 0
	if !isBinary(data) {
		t.Fatal("NUL inside sniff window must count")
	}
	if isBinary(nil) {
		t.Fatal("nil is not binary")
	}
}

func TestOpenInvalidUTF8(t *testing.T) {
	p := writeFile(t, "bad.txt", []byte("ok \xff\xfe bad"), 0o644)
	d, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	if !d.ReadOnly || d.Buffer == nil || d.Kind != Text {
		t.Fatalf("doc %+v", d)
	}
	if err := d.Save(); err == nil {
		t.Fatal("save should fail")
	}
	if readFile(t, p) != "ok \xff\xfe bad" {
		t.Fatal("file changed")
	}
}

func TestOpenErrors(t *testing.T) {
	if _, err := Open(t.TempDir()); err == nil {
		t.Fatal("directory should fail")
	}
	if _, err := Open(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing file should fail")
	}
}

func TestSaveWritesEdits(t *testing.T) {
	p := writeFile(t, "a.txt", []byte("hello"), 0o644)
	d, _ := Open(p)
	d.Buffer.Set("changed")
	if !d.Modified() {
		t.Fatal("expected modified")
	}
	if err := d.Save(); err != nil {
		t.Fatal(err)
	}
	if readFile(t, p) != "changed" || d.Modified() || d.ChangedOnDisk() {
		t.Fatal("save state wrong")
	}
}

func TestSaveKeepsCRLF(t *testing.T) {
	const content = "one\r\ntwo\r\nthree\r\n"
	p := writeFile(t, "crlf.txt", []byte(content), 0o644)
	d, _ := Open(p)
	if err := d.Save(); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, p); got != content {
		t.Fatalf("got %q", got)
	}
}

func TestSaveKeepsMode(t *testing.T) {
	p := writeFile(t, "run.sh", []byte("#!/bin/sh\n"), 0o755)
	if err := os.Chmod(p, 0o755); err != nil {
		t.Fatal(err)
	}
	d, _ := Open(p)
	d.Buffer.Set("#!/bin/sh\necho hi\n")
	if err := d.Save(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("mode %v", info.Mode().Perm())
	}
}

func TestSaveMissingFileUsesDefaultPerm(t *testing.T) {
	p := writeFile(t, "gone.txt", []byte("x"), 0o644)
	d, _ := Open(p)
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if err := d.Save(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if readFile(t, p) != "x" || info.Mode().Perm() != defaultPerm {
		t.Fatalf("mode %v", info.Mode().Perm())
	}
}

func TestSaveThroughSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	link := filepath.Join(dir, "link.txt")
	if err := os.WriteFile(target, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	d, err := Open(link)
	if err != nil {
		t.Fatal(err)
	}
	d.Buffer.Set("new")
	if err := d.Save(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("link replaced by regular file")
	}
	if readFile(t, target) != "new" {
		t.Fatalf("target %q", readFile(t, target))
	}
	if got, _ := os.Readlink(link); got != target {
		t.Fatalf("link points to %q", got)
	}
}

func TestChangedOnDisk(t *testing.T) {
	p := writeFile(t, "a.txt", []byte("hello"), 0o644)
	d, _ := Open(p)
	if d.ChangedOnDisk() {
		t.Fatal("unexpected change")
	}
	if err := os.WriteFile(p, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !d.ChangedOnDisk() {
		t.Fatal("size change not detected")
	}
	if err := os.WriteFile(p, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(p, time.Now(), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if !d.ChangedOnDisk() {
		t.Fatal("mtime change not detected")
	}
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if d.ChangedOnDisk() {
		t.Fatal("missing file should not count as changed")
	}
}

func TestReadDiskMissing(t *testing.T) {
	p := writeFile(t, "a.txt", []byte("x"), 0o644)
	d, _ := Open(p)
	os.Remove(p)
	if _, err := d.ReadDisk(); err == nil {
		t.Fatal("expected error")
	}
}

func TestApplyDiskWhenClean(t *testing.T) {
	p := writeFile(t, "a.txt", []byte("one"), 0o644)
	d, _ := Open(p)
	if err := os.WriteFile(p, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(p, time.Now(), time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if !d.ChangedOnDisk() {
		t.Fatal("expected change")
	}
	s, err := d.ReadDisk()
	if err != nil {
		t.Fatal(err)
	}
	if !d.ApplyDisk(s) {
		t.Fatal("apply should succeed")
	}
	if d.Buffer.String() != "two" || d.Modified() || d.ChangedOnDisk() {
		t.Fatalf("buffer %q modified %v", d.Buffer.String(), d.Modified())
	}
}

func TestApplyDiskWhenModified(t *testing.T) {
	p := writeFile(t, "a.txt", []byte("one"), 0o644)
	d, _ := Open(p)
	d.Buffer.Set("local edit")
	if err := os.WriteFile(p, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := d.ReadDisk()
	if err != nil {
		t.Fatal(err)
	}
	if d.ApplyDisk(s) {
		t.Fatal("apply must refuse when modified")
	}
	if d.Buffer.String() != "local edit" || !d.Modified() {
		t.Fatal("buffer changed")
	}
}

func TestApplyDiskWithoutBuffer(t *testing.T) {
	d := &Document{}
	if d.ApplyDisk(DiskState{text: "x"}) {
		t.Fatal("apply without buffer must fail")
	}
}

func TestApplyDiskSameTextMarksSaved(t *testing.T) {
	p := writeFile(t, "a.txt", []byte("same"), 0o644)
	d, _ := Open(p)
	s, err := d.ReadDisk()
	if err != nil {
		t.Fatal(err)
	}
	if !d.ApplyDisk(s) || d.Buffer.String() != "same" {
		t.Fatal("apply failed")
	}
}
