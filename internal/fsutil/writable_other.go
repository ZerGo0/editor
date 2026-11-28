//go:build !darwin && !linux

package fsutil

// TECHDEBT: Write permission is not checked on this platform; add its access check if the editor ships there.
func Writable(string) bool {
	return true
}
