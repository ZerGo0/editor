package viewer

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestGIFFrameCount(t *testing.T) {
	dir := t.TempDir()
	for _, frames := range []int{1, 2, 5} {
		path := writeGIF(t, dir, "a.gif", frames)
		data := mustRead(t, path)
		if got := gifFrameCount(data); got != frames {
			t.Errorf("gifFrameCount of %d frames = %d", frames, got)
		}
	}
	for _, bad := range [][]byte{nil, []byte("GIF89a"), []byte("not a gif at all, but long enough"), []byte("GIF89a\x01\x00\x01\x00\x80\x00\x00")} {
		if got := gifFrameCount(bad); got != 0 {
			t.Errorf("gifFrameCount(%q) = %d, want 0", bad, got)
		}
	}
}

func TestGIFFrameCountSurvivesTruncation(t *testing.T) {
	data := mustRead(t, writeGIF(t, t.TempDir(), "a.gif", 3))
	for cut := 0; cut < len(data); cut += 7 {
		gifFrameCount(data[:cut])
	}
}

func pngChunk(kind string, payload []byte) []byte {
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.BigEndian, uint32(len(payload)))
	buf.WriteString(kind)
	buf.Write(payload)
	buf.Write(make([]byte, 4))
	return buf.Bytes()
}

func TestIsAPNG(t *testing.T) {
	signature := []byte("\x89PNG\r\n\x1a\n")
	animated := bytes.Join([][]byte{signature, pngChunk("IHDR", make([]byte, 13)), pngChunk("acTL", make([]byte, 8)), pngChunk("IDAT", nil)}, nil)
	still := bytes.Join([][]byte{signature, pngChunk("IHDR", make([]byte, 13)), pngChunk("IDAT", nil), pngChunk("acTL", make([]byte, 8))}, nil)
	if !isAPNG(animated) {
		t.Error("animated PNG not detected")
	}
	if isAPNG(still) || isAPNG(signature) || isAPNG([]byte("GIF89a")) || isAPNG(nil) {
		t.Error("a still image was taken for an animation")
	}
	if isAPNG(append(append([]byte{}, signature...), 0xff, 0xff, 0xff, 0xff, 'a', 'c', 'T', 'L')) {
		t.Error("a truncated chunk was taken for an animation")
	}
}
