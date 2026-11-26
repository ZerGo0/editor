package viewer

import (
	"bytes"
	"encoding/binary"
)

const (
	gifHeaderLen     = 6
	gifPackedAt      = gifHeaderLen + 4
	gifDescriptorLen = 7
	gifImageHeader   = 9
	gifTableFlag     = 0x80
	gifTableSizeMask = 0x07
	gifTableEntry    = 3
	gifExtension     = 0x21
	gifImage         = 0x2c
	pngSignatureLen  = 8
	pngChunkOverhead = 12
	pngTypeLen       = 4
)

func gifFrameCount(data []byte) int {
	if len(data) < gifHeaderLen+gifDescriptorLen || !bytes.HasPrefix(data, []byte("GIF8")) {
		return 0
	}
	pos := gifHeaderLen + gifDescriptorLen
	pos += gifColorTable(data[gifPackedAt])
	frames := 0
	for pos < len(data) {
		switch data[pos] {
		case gifExtension:
			pos += 2
			pos = skipGIFBlocks(data, pos)
		case gifImage:
			frames++
			if pos+gifImageHeader >= len(data) {
				return frames
			}
			flags := data[pos+gifImageHeader]
			pos += 1 + gifImageHeader + gifColorTable(flags)
			pos++
			pos = skipGIFBlocks(data, pos)
		default:
			return frames
		}
	}
	return frames
}

func gifColorTable(flags byte) int {
	if flags&gifTableFlag == 0 {
		return 0
	}
	return gifTableEntry << (flags&gifTableSizeMask + 1)
}

func skipGIFBlocks(data []byte, pos int) int {
	for pos < len(data) {
		n := int(data[pos])
		pos += 1 + n
		if n == 0 {
			return pos
		}
	}
	return len(data)
}

func isAPNG(data []byte) bool {
	if !bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")) {
		return false
	}
	for pos := pngSignatureLen; pos+pngChunkOverhead <= len(data); {
		length := int(binary.BigEndian.Uint32(data[pos:]))
		switch string(data[pos+pngTypeLen : pos+2*pngTypeLen]) {
		case "acTL":
			return true
		case "IDAT":
			return false
		}
		pos += pngChunkOverhead + length
	}
	return false
}
