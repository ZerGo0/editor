package multidiff

import (
	"context"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/git/diffmode"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	LargeDiffLines = 2000
	LargeDiffBytes = 512 << 10
	CountUnknown   = -1
)

type Entry struct {
	Key         string
	Path        string
	OldPath     string
	Status      token.GitStatus
	Adds        int
	Dels        int
	Size        int64
	Fingerprint string
	Binary      bool
	Note        string
	Patch       *editor.FilePatch
	Live        func() *editor.Buffer
	Load        func(ctx context.Context, live bool) (oldText, newText string, err error)
	Setup       func(d *editor.DiffState, e *diffmode.Engine)
	Frame       func(d *editor.DiffState, e *diffmode.Engine)
	Annotations func(d *editor.DiffState) []editor.DiffAnnotation
	Actions     func(c *ui.Context)
	Section     func(c *ui.Context)
}

type Note string

func (n Note) Error() string { return string(n) }

func (e *Entry) isFile() bool { return e.Section == nil }

func (e *Entry) large() bool {
	if e.Size >= LargeDiffBytes {
		return true
	}
	return e.Adds != CountUnknown && e.Dels != CountUnknown && e.Adds+e.Dels > LargeDiffLines
}

func (e *Entry) weight() int {
	if e.Adds == CountUnknown || e.Dels == CountUnknown {
		return 0
	}
	return e.Adds + e.Dels
}

type Viewed interface {
	IsViewed(set string, e *Entry) bool
	SetViewed(set string, e *Entry, on bool)
}

type Memory struct{ marked map[string]bool }

func NewMemory() *Memory { return &Memory{marked: map[string]bool{}} }

func memoryKey(set string, e *Entry) string {
	return set + "\x00" + e.Path + "\x00" + e.Fingerprint
}

func (m *Memory) IsViewed(set string, e *Entry) bool { return m.marked[memoryKey(set, e)] }

func (m *Memory) SetViewed(set string, e *Entry, on bool) {
	if on {
		m.marked[memoryKey(set, e)] = true
		return
	}
	delete(m.marked, memoryKey(set, e))
}

type Options struct {
	Label     string
	Set       string
	Ctx       context.Context
	Post      func(func())
	Configure func(d *editor.DiffState, path string)
	Language  func(path, text string) *editor.Language
	Viewed    Viewed
	OpenFile  func(path string)
	Toolbar   func(c *ui.Context)
	Busy      bool
}
