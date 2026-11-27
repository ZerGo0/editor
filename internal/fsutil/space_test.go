package fsutil

import "testing"

func TestFreeBytesReadsTheRealDisk(t *testing.T) {
	if _, err := FreeBytes(t.TempDir()); err != nil {
		t.Fatal(err)
	}
}
