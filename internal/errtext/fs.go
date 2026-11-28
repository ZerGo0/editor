package errtext

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"syscall"
)

var errnoText = map[syscall.Errno]string{
	syscall.ENOSPC:       "The disk is full. Free up space and try again.",
	syscall.EDQUOT:       "Your disk quota is used up. Free up space and try again.",
	syscall.EROFS:        "The disk is read-only.",
	syscall.ENOTDIR:      "A part of its path is a file, not a folder.",
	syscall.EISDIR:       "It is a folder, not a file.",
	syscall.ENAMETOOLONG: "Its name is too long.",
	syscall.ELOOP:        "It is a link that points to itself.",
	syscall.EIO:          "The disk reported a read or write error.",
	syscall.EBUSY:        "Another program is using it.",
	syscall.ETXTBSY:      "Another program is using it.",
	syscall.EMFILE:       "Too many files are open. Close some files and try again.",
	syscall.ENFILE:       "Too many files are open. Close some files and try again.",
	syscall.EXDEV:        "It cannot be moved to another disk this way.",
	syscall.ENOTEMPTY:    "The folder is not empty.",
}

var readOps = map[string]bool{"open": true, "stat": true, "lstat": true, "read": true, "readdirent": true, "readlink": true, "evalsymlinks": true}

func canceled(err error) (string, bool) {
	switch {
	case errors.Is(err, context.Canceled):
		return "The operation was canceled.", true
	case errors.Is(err, context.DeadlineExceeded):
		return "It took too long and was stopped.", true
	}
	return "", false
}

func fileSystem(err error) (string, bool) {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		if s, ok := errnoText[errno]; ok {
			return s, true
		}
	}
	write := isWrite(err)
	switch {
	case errors.Is(err, fs.ErrPermission) && write:
		return "You do not have permission to change it. The file or its folder may be read-only.", true
	case errors.Is(err, fs.ErrPermission):
		return "You do not have permission to open it.", true
	case errors.Is(err, fs.ErrNotExist) && write:
		return "Its folder does not exist or was moved.", true
	case errors.Is(err, fs.ErrNotExist):
		return "It does not exist or was moved.", true
	case errors.Is(err, fs.ErrExist):
		return "Something with that name already exists.", true
	}
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return sentence(pe.Err.Error()), true
	}
	var le *os.LinkError
	if errors.As(err, &le) {
		return sentence(le.Err.Error()), true
	}
	return "", false
}

func isWrite(err error) bool {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return !readOps[pe.Op]
	}
	var le *os.LinkError
	return errors.As(err, &le)
}
