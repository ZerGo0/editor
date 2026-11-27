package viewer

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type Kind int

const (
	None Kind = iota
	Image
	SVG
	Video
	Audio
	PDF
	Markdown
	Hex
)

const binarySniffSize = 8000

var kindNames = map[Kind]string{
	None: "None", Image: "Image", SVG: "SVG", Video: "Video", Audio: "Audio",
	PDF: "PDF", Markdown: "Markdown", Hex: "Hex",
}

func (k Kind) String() string { return kindNames[k] }

func (k Kind) ReplacesEditor() bool {
	switch k {
	case Image, Video, Audio, PDF, Hex:
		return true
	}
	return false
}

func (k Kind) Previewable() bool { return k == Markdown || k == SVG }

var extensionKinds = buildExtensionKinds()

func buildExtensionKinds() map[string]Kind {
	groups := map[Kind][]string{
		Image: {
			".png", ".apng", ".jpg", ".jpeg", ".jpe", ".jfif", ".gif", ".webp", ".bmp", ".dib",
			".ico", ".cur", ".tif", ".tiff", ".heic", ".heif", ".avif", ".icns", ".jp2",
		},
		SVG:      {".svg"},
		Video:    {".mp4", ".m4v", ".mov", ".qt", ".avi", ".mkv", ".webm", ".3gp", ".3g2", ".ogv"},
		Audio:    {".mp3", ".wav", ".wave", ".m4a", ".m4b", ".aac", ".flac", ".ogg", ".oga", ".opus", ".aiff", ".aif", ".aifc", ".caf"},
		PDF:      {".pdf"},
		Markdown: {".md", ".markdown", ".mdown", ".mkd", ".mkdn"},
	}
	kinds := map[string]Kind{}
	for kind, exts := range groups {
		for _, ext := range exts {
			kinds[ext] = kind
		}
	}
	return kinds
}

func Detect(path string, head []byte) Kind {
	if kind, ok := extensionKinds[strings.ToLower(filepath.Ext(path))]; ok {
		return kind
	}
	if kind := sniff(head); kind != None {
		return kind
	}
	if bytes.IndexByte(head[:min(len(head), binarySniffSize)], 0) >= 0 {
		return Hex
	}
	return None
}

func DetectFile(path string) (Kind, error) {
	f, err := os.Open(path)
	if err != nil {
		return None, err
	}
	defer f.Close()
	head := make([]byte, binarySniffSize)
	n, err := io.ReadFull(f, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return None, err
	}
	return Detect(path, head[:n]), nil
}

type signature struct {
	prefix string
	kind   Kind
}

var prefixSignatures = []signature{
	{"\x89PNG\r\n\x1a\n", Image},
	{"\xff\xd8\xff", Image},
	{"GIF87a", Image},
	{"GIF89a", Image},
	{"II*\x00", Image},
	{"MM\x00*", Image},
	{"icns", Image},
	{"\x00\x00\x00\x0cjP  \r\n\x87\n", Image},
	{"%PDF-", PDF},
	{"fLaC", Audio},
	{"OggS", Audio},
	{"ID3", Audio},
	{"caff", Audio},
	{"\x1a\x45\xdf\xa3", Video},
}

const (
	riffTagOffset      = 8
	riffTagLen         = 4
	boxTypeOffset      = 4
	boxBrandOffset     = 8
	boxFieldLen        = 4
	bmpReservedAt      = 6
	bmpHeaderSizeAt    = 14
	bmpMinLen          = 18
	iconHeaderLen      = 22
	iconCountAt        = 4
	iconReservedByteAt = 9
	iconTypeIcon       = 1
	iconTypeCursor     = 2
	mp3HeaderLen       = 4
	mp3SyncMask        = 0xe0
	mp3ReservedVersion = 1
	mp3LayerIII        = 1
	mp3ReservedBitrate = 15
	mp3ReservedRate    = 3
)

var (
	imageBrands   = []string{"heic", "heix", "hevc", "hevx", "heim", "heis", "hevm", "hevs", "mif1", "msf1", "avif", "avis"}
	audioBrands   = []string{"M4A ", "M4B ", "M4P "}
	bmpHeaderSize = []uint32{12, 40, 52, 56, 64, 108, 124}
	movieBoxTypes = []string{"moov", "mdat", "free", "wide", "skip"}
	riffKinds     = map[string]Kind{"WEBP": Image, "WAVE": Audio, "AVI ": Video}
	formKinds     = []string{"AIFF", "AIFC"}
)

func sniff(h []byte) Kind {
	for _, sig := range prefixSignatures {
		if bytes.HasPrefix(h, []byte(sig.prefix)) {
			return sig.kind
		}
	}
	switch {
	case bytes.HasPrefix(h, []byte("RIFF")) && len(h) >= riffTagOffset+riffTagLen:
		return riffKinds[string(h[riffTagOffset:riffTagOffset+riffTagLen])]
	case bytes.HasPrefix(h, []byte("FORM")) && len(h) >= riffTagOffset+riffTagLen:
		if slices.Contains(formKinds, string(h[riffTagOffset:riffTagOffset+riffTagLen])) {
			return Audio
		}
	case isBMP(h):
		return Image
	case isIcon(h):
		return Image
	case isBox(h, "ftyp"):
		return sniffBrand(h)
	case len(h) >= boxTypeOffset+boxFieldLen && slices.Contains(movieBoxTypes, string(h[boxTypeOffset:boxTypeOffset+boxFieldLen])):
		return Video
	case isMP3Frame(h):
		return Audio
	}
	return None
}

func isBox(h []byte, kind string) bool {
	return len(h) >= boxBrandOffset+boxFieldLen && string(h[boxTypeOffset:boxTypeOffset+boxFieldLen]) == kind
}

func sniffBrand(h []byte) Kind {
	brand := string(h[boxBrandOffset : boxBrandOffset+boxFieldLen])
	switch {
	case slices.Contains(imageBrands, brand):
		return Image
	case slices.Contains(audioBrands, brand):
		return Audio
	}
	return Video
}

func isBMP(h []byte) bool {
	if len(h) < bmpMinLen || h[0] != 'B' || h[1] != 'M' {
		return false
	}
	if binary.LittleEndian.Uint32(h[bmpReservedAt:]) != 0 {
		return false
	}
	size := binary.LittleEndian.Uint32(h[bmpHeaderSizeAt:])
	for _, want := range bmpHeaderSize {
		if size == want {
			return true
		}
	}
	return false
}

func isIcon(h []byte) bool {
	if len(h) < iconHeaderLen || h[0] != 0 || h[1] != 0 || h[3] != 0 {
		return false
	}
	if h[2] != iconTypeIcon && h[2] != iconTypeCursor {
		return false
	}
	return binary.LittleEndian.Uint16(h[iconCountAt:]) > 0 && h[iconReservedByteAt] == 0
}

func isMP3Frame(h []byte) bool {
	if len(h) < mp3HeaderLen || h[0] != 0xff || h[1]&mp3SyncMask != mp3SyncMask {
		return false
	}
	version, layer := (h[1]>>3)&3, (h[1]>>1)&3
	bitrate, rate := h[2]>>4, (h[2]>>2)&3
	return version != mp3ReservedVersion && layer == mp3LayerIII && bitrate != mp3ReservedBitrate && rate != mp3ReservedRate
}
