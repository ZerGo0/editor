package applog

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"

	"github.com/ZerGo0/editor/internal/fsutil"
)

const (
	FileName      = "editor.log"
	crashFileName = "crash.log"
	MaxSize       = 5 << 20
	MaxBackups    = 4
	filePerm      = 0o600
	dirPerm       = 0o700
	firstBackup   = 1
)

type Writer struct {
	mu      sync.Mutex
	path    string
	max     int64
	backups int
	f       *os.File
	size    int64
}

func Path(dir string) string { return filepath.Join(dir, FileName) }

func Setup(dir string) error {
	w, err := Open(Path(dir), MaxSize, MaxBackups)
	if err != nil {
		return err
	}
	log.SetOutput(io.MultiWriter(os.Stderr, w))
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	crashPath := filepath.Join(dir, crashFileName)
	crash, err := openCrash(crashPath)
	if err != nil {
		return err
	}
	setCrashPath(crashPath)
	defer crash.Close()
	if err := debug.SetCrashOutput(crash, debug.CrashOptions{}); err != nil {
		return fmt.Errorf("crash output: %w", err)
	}
	return nil
}

func Open(path string, max int64, backups int) (*Writer, error) {
	if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
		return nil, fmt.Errorf("log dir: %w", err)
	}
	w := &Writer{path: path, max: max, backups: backups}
	if err := w.open(); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *Writer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.size > 0 && w.size+int64(len(p)) > w.max {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}
	n, err := w.f.Write(p)
	w.size += int64(n)
	fsutil.CountWritten(n)
	return n, err
}

func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.f.Close()
}

func (w *Writer) open() error {
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, filePerm)
	if err != nil {
		return fmt.Errorf("open log: %w", err)
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return fmt.Errorf("stat log: %w", err)
	}
	w.f, w.size = f, info.Size()
	return nil
}

func (w *Writer) rotate() error {
	if err := w.f.Close(); err != nil {
		return fmt.Errorf("close log: %w", err)
	}
	if err := shift(w.path, w.backups); err != nil {
		return err
	}
	return w.open()
}

func shift(path string, backups int) error {
	for i := backups - 1; i >= firstBackup; i-- {
		if err := os.Rename(backupPath(path, i), backupPath(path, i+1)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("rotate log: %w", err)
		}
	}
	if backups < firstBackup {
		return os.Remove(path)
	}
	if err := os.Rename(path, backupPath(path, firstBackup)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("rotate log: %w", err)
	}
	return nil
}

func backupPath(path string, i int) string {
	ext := filepath.Ext(path)
	return strings.TrimSuffix(path, ext) + "." + strconv.Itoa(i) + ext
}

func openCrash(path string) (*os.File, error) {
	if info, err := os.Stat(path); err == nil && info.Size() > MaxSize {
		if err := shift(path, firstBackup); err != nil {
			return nil, err
		}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, filePerm)
	if err != nil {
		return nil, fmt.Errorf("open crash log: %w", err)
	}
	return f, nil
}
