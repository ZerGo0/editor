package document

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"
	"unicode/utf8"

	"github.com/ZerGo0/editor/internal/fsutil"
	"github.com/ZerGo0/editor/internal/largefile"
	"github.com/ZerGo0/editor/internal/syntax"
	"github.com/egoist/mygo/ui/editor"
)

type Kind int

const (
	Text Kind = iota
	Plain
	TooLarge
	Binary
)

const (
	HighlightLimit     = 20 << 20
	binarySniffSize    = 8000
	languageSampleSize = 8192
	defaultPerm        = 0o644
)

var EditLimit int64 = 512 << 20

type stamp struct {
	size    int64
	modTime time.Time
}

func stampOf(info fs.FileInfo) stamp {
	return stamp{size: info.Size(), modTime: info.ModTime()}
}

type Document struct {
	Path     string
	Kind     Kind
	Buffer   *editor.Buffer
	Large    *largefile.Doc
	ReadOnly bool
	stamp    stamp
}

type DiskState struct {
	text  string
	stamp stamp
}

func Empty(path string) *Document {
	d := &Document{Path: path, Kind: Text, Buffer: editor.NewBuffer("")}
	d.Buffer.SetLanguage(syntax.ForName(path))
	return d
}

func Open(path string) (*Document, error) {
	return open(path, false)
}

func OpenText(path string) (*Document, error) {
	return open(path, true)
}

func open(path string, allowBinary bool) (*Document, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, errors.New("is a directory")
	}
	d := &Document{Path: path, stamp: stampOf(info)}
	if info.Size() > EditLimit {
		d.Kind = TooLarge
		if d.Large, err = largefile.Open(path); err != nil {
			return nil, err
		}
		return d, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if !allowBinary && isBinary(data) {
		d.Kind = Binary
		return d, nil
	}
	text := string(data)
	d.ReadOnly = !utf8.ValidString(text)
	d.Buffer = editor.NewBuffer(text)
	if len(data) > HighlightLimit {
		d.Kind = Plain
		return d, nil
	}
	d.Kind = Text
	d.Buffer.SetLanguage(syntax.For(path, text[:min(len(text), languageSampleSize)]))
	return d, nil
}

func isBinary(data []byte) bool {
	return bytes.IndexByte(data[:min(len(data), binarySniffSize)], 0) >= 0
}

func (d *Document) Editable() bool {
	return d.Buffer != nil
}

func (d *Document) Modified() bool {
	return d.Buffer != nil && d.Buffer.Modified() || d.Large != nil && d.Large.Modified()
}

func (d *Document) Close() {
	if d.Large != nil {
		d.Large.Close()
	}
}

func (d *Document) SaveLarge(ctx context.Context, progress func(done, total int64)) error {
	if err := d.Large.Save(ctx, progress); err != nil {
		return err
	}
	if info, err := os.Stat(d.Path); err == nil {
		d.stamp = stampOf(info)
	}
	return nil
}

// TECHDEBT: Saves on the UI thread; write a buffer snapshot on a goroutine once large files make saves slow.
func (d *Document) Save() error {
	if d.Buffer == nil || d.ReadOnly {
		return errors.New("this file cannot be saved")
	}
	target, err := filepath.EvalSymlinks(d.Path)
	if errors.Is(err, fs.ErrNotExist) {
		target = d.Path
	} else if err != nil {
		return err
	}
	perm := fs.FileMode(defaultPerm)
	if info, err := os.Stat(target); err == nil {
		perm = info.Mode().Perm()
	}
	err = fsutil.WriteAtomic(target, perm, func(w io.Writer) error {
		_, err := d.Buffer.WriteTo(w)
		return err
	})
	if err != nil {
		return err
	}
	d.Buffer.MarkSaved()
	if info, err := os.Stat(target); err == nil {
		d.stamp = stampOf(info)
	}
	return nil
}

func (d *Document) Rename(path string) {
	d.Path = path
	if d.Kind == Text {
		d.Buffer.SetLanguage(syntax.ForName(path))
	}
}

func (d *Document) EnsureFinalNewline() {
	if d.Buffer == nil || d.ReadOnly {
		return
	}
	n := d.Buffer.Len()
	if n == 0 || d.Buffer.Slice(n-1, n) == "\n" || d.Buffer.Slice(n-1, n) == "\r" {
		return
	}
	d.Buffer.Edit(editor.Edit{Start: n, End: n, Text: d.Buffer.LineEnding()})
}

func (d *Document) DiskStamp() (size int64, modTime time.Time) {
	return d.stamp.size, d.stamp.modTime
}

func (d *Document) ChangedOnDisk() bool {
	if d.Large != nil {
		return false
	}
	info, err := os.Stat(d.Path)
	return err == nil && stampOf(info) != d.stamp
}

func (d *Document) ReadDisk() (DiskState, error) {
	info, err := os.Stat(d.Path)
	if err != nil {
		return DiskState{}, err
	}
	data, err := os.ReadFile(d.Path)
	if err != nil {
		return DiskState{}, err
	}
	return DiskState{text: string(data), stamp: stampOf(info)}, nil
}

func (d *Document) ApplyDisk(s DiskState) bool {
	if d.Buffer == nil || d.Buffer.Modified() {
		return false
	}
	if d.Buffer.String() != s.text {
		d.Buffer.Set(s.text)
	}
	d.Buffer.MarkSaved()
	d.stamp = s.stamp
	return true
}
