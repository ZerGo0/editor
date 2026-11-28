//go:build darwin || linux

package fsutil

import "syscall"

const writeAccess = 0x2

func Writable(path string) bool {
	return syscall.Access(path, writeAccess) == nil
}
