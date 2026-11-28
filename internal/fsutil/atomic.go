package fsutil

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

const saveOp = "save"

func WriteAtomic(path string, perm fs.FileMode, write func(io.Writer) error) (err error) {
	if _, err := os.Stat(path); err == nil && !Writable(path) {
		return &fs.PathError{Op: saveOp, Path: path, Err: fs.ErrPermission}
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return saveError(path, err)
	}
	defer func() {
		if err != nil {
			_ = os.Remove(tmp.Name())
			err = saveError(path, err)
		}
	}()
	if err = write(countingWriter{tmp}); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Chmod(tmp.Name(), perm); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func saveError(path string, err error) error {
	var pe *fs.PathError
	var le *os.LinkError
	switch {
	case errors.As(err, &pe):
		err = pe.Err
	case errors.As(err, &le):
		err = le.Err
	default:
		return err
	}
	return &fs.PathError{Op: saveOp, Path: path, Err: err}
}

func WriteFileAtomic(path string, data []byte, perm fs.FileMode) error {
	return WriteAtomic(path, perm, func(w io.Writer) error {
		_, err := w.Write(data)
		return err
	})
}
