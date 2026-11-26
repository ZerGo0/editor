//go:build !darwin && !linux

package largefile

import "math"

// TECHDEBT: Free space is not checked on this platform; add its statfs equivalent if the editor ships there.
func diskFree(string) (uint64, error) {
	return math.MaxUint64, nil
}
