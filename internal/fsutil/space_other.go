//go:build !darwin && !linux

package fsutil

import "math"

// TECHDEBT: Free space is not checked on this platform; add its statfs equivalent if the editor ships there.
func FreeBytes(string) (uint64, error) {
	return math.MaxUint64, nil
}
