package viewer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectByExtension(t *testing.T) {
	tests := []struct {
		path string
		want Kind
	}{
		{"a.png", Image}, {"A.PNG", Image}, {"a.jpg", Image}, {"a.JPEG", Image}, {"a.gif", Image},
		{"a.webp", Image}, {"a.bmp", Image}, {"a.ico", Image}, {"a.tiff", Image}, {"a.heic", Image},
		{"a.svg", SVG}, {"a.mp4", Video}, {"a.mov", Video}, {"a.webm", Video},
		{"a.mp3", Audio}, {"a.wav", Audio}, {"a.m4a", Audio}, {"a.flac", Audio},
		{"a.pdf", PDF}, {"README.md", Markdown}, {"notes.markdown", Markdown},
		{"main.go", None}, {"Makefile", None}, {"video.ts", None},
		{"dir.png/readme", None},
	}
	for _, tt := range tests {
		if got := Detect(tt.path, nil); got != tt.want {
			t.Errorf("Detect(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestDetectByMagic(t *testing.T) {
	riff := func(tag string) []byte { return []byte("RIFF\x00\x00\x00\x00" + tag + "padding") }
	ftyp := func(brand string) []byte { return []byte("\x00\x00\x00\x18ftyp" + brand + "\x00\x00\x00\x00") }
	bmp := append([]byte("BM\x00\x00\x00\x00\x00\x00\x00\x00\x36\x00\x00\x00"), 40, 0, 0, 0, 1, 1, 1, 1)
	ico := append([]byte{0, 0, 1, 0, 1, 0, 16, 16, 0, 0}, make([]byte, 16)...)
	tests := []struct {
		name string
		head []byte
		want Kind
	}{
		{"png", []byte("\x89PNG\r\n\x1a\n\x00"), Image},
		{"jpeg", []byte("\xff\xd8\xff\xe0\x00\x10JFIF"), Image},
		{"gif", []byte("GIF89a\x01\x00"), Image},
		{"webp", riff("WEBP"), Image},
		{"wav", riff("WAVE"), Audio},
		{"avi", riff("AVI "), Video},
		{"riff of another kind", riff("ACON"), Hex},
		{"bmp", bmp, Image},
		{"ico", ico, Image},
		{"tiff little endian", []byte("II*\x00\x08\x00\x00\x00"), Image},
		{"tiff big endian", []byte("MM\x00*\x00\x00\x00\x08"), Image},
		{"heic", ftyp("heic"), Image},
		{"avif", ftyp("avif"), Image},
		{"mp4", ftyp("isom"), Video},
		{"mov", ftyp("qt  "), Video},
		{"m4a", ftyp("M4A "), Audio},
		{"quicktime without ftyp", []byte("\x00\x00\x00\x14moov\x00\x00"), Video},
		{"pdf", []byte("%PDF-1.7\n"), PDF},
		{"mp3 with a tag", []byte("ID3\x04\x00\x00"), Audio},
		{"mp3 frame", []byte("\xff\xfb\x90\x64\x00"), Audio},
		{"adts is not an mp3 frame", []byte("\xff\xf1\x50\x80\x00"), Hex},
		{"flac", []byte("fLaC\x00\x00"), Audio},
		{"ogg", []byte("OggS\x00\x02"), Audio},
		{"aiff", []byte("FORM\x00\x00\x00\x00AIFFCOMM"), Audio},
		{"matroska", []byte("\x1a\x45\xdf\xa3\x01"), Video},
		{"icns", []byte("icns\x00\x00\x10\x00"), Image},
	}
	for _, tt := range tests {
		if got := Detect("file", tt.head); got != tt.want {
			t.Errorf("%s: Detect = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestDetectTextAndBinary(t *testing.T) {
	tests := []struct {
		name string
		head []byte
		want Kind
	}{
		{"empty", nil, None},
		{"source code", []byte("package main\n\nfunc main() {}\n"), None},
		{"unicode text", []byte("h\u00e9llo w\u00f6rld"), None},
		{"text that starts like a BMP", []byte("BM is a company name, not a bitmap header"), None},
		{"text that starts like an MP3 sync", []byte("\xff\xfe text with a BOM"), None},
		{"nul byte", []byte("abc\x00def"), Hex},
		{"executable", []byte("\x7fELF\x02\x01\x01\x00\x00\x00"), Hex},
	}
	for _, tt := range tests {
		if got := Detect("file.dat", tt.head); got != tt.want {
			t.Errorf("%s: Detect = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestDetectOnlyLooksAtTheFirstBytesForNul(t *testing.T) {
	head := []byte(strings.Repeat("a", binarySniffSize) + "\x00")
	if got := Detect("file", head); got != None {
		t.Errorf("a NUL past the sniff window gave %v", got)
	}
}

func TestDetectPrefersTheExtension(t *testing.T) {
	if got := Detect("photo.png", []byte("%PDF-1.4")); got != Image {
		t.Errorf("got %v, want the extension's kind", got)
	}
	if got := Detect("data.bin", []byte("\x89PNG\r\n\x1a\n")); got != Image {
		t.Errorf("got %v, want the magic kind", got)
	}
}

func TestDetectFile(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		name string
		data string
		want Kind
	}{
		{"shot", "\x89PNG\r\n\x1a\nrest", Image},
		{"blob", "ab\x00cd", Hex},
		{"notes.txt", "plain text", None},
		{"empty", "", None},
	}
	for _, tt := range tests {
		p := filepath.Join(dir, tt.name)
		if err := os.WriteFile(p, []byte(tt.data), 0o644); err != nil {
			t.Fatal(err)
		}
		got, err := DetectFile(p)
		if err != nil || got != tt.want {
			t.Errorf("DetectFile(%s) = %v, %v; want %v", tt.name, got, err, tt.want)
		}
	}
	if _, err := DetectFile(filepath.Join(dir, "missing")); err == nil {
		t.Error("DetectFile on a missing file returned no error")
	}
}

func TestKindHelpers(t *testing.T) {
	for _, k := range []Kind{Image, Video, Audio, PDF, Hex} {
		if !k.ReplacesEditor() || k.Previewable() {
			t.Errorf("%v: replaces %v previewable %v", k, k.ReplacesEditor(), k.Previewable())
		}
	}
	for _, k := range []Kind{Markdown, SVG} {
		if k.ReplacesEditor() || !k.Previewable() {
			t.Errorf("%v: replaces %v previewable %v", k, k.ReplacesEditor(), k.Previewable())
		}
	}
	if None.ReplacesEditor() || None.Previewable() {
		t.Error("None has a viewer")
	}
	if Hex.String() != "Hex" || PDF.String() != "PDF" {
		t.Error("String names are wrong")
	}
}
