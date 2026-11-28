package multidiff

import (
	"context"
	"slices"

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
	Title       string
	Path        string
	OldPath     string
	Status      token.GitStatus
	Adds        int
	Dels        int
	Size        int64
	Fingerprint string
	Binary      bool
	Whole       bool
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

const MaxViewedMarks = 100

type Memory struct {
	marked map[string]bool
	order  []string
}

func NewMemory(marks ...string) *Memory {
	m := &Memory{marked: map[string]bool{}}
	for _, key := range marks {
		m.mark(key)
	}
	return m
}

func (m *Memory) Marks() []string { return slices.Clone(m.order) }

func (m *Memory) mark(key string) {
	if m.marked[key] {
		return
	}
	m.marked[key] = true
	m.order = append(m.order, key)
	if len(m.order) > MaxViewedMarks {
		delete(m.marked, m.order[0])
		m.order = m.order[1:]
	}
}

func memoryKey(set string, e *Entry) string {
	return set + "\x00" + e.Path + "\x00" + e.Fingerprint
}

func (m *Memory) IsViewed(set string, e *Entry) bool { return m.marked[memoryKey(set, e)] }

func (m *Memory) SetViewed(set string, e *Entry, on bool) {
	key := memoryKey(set, e)
	if on {
		m.mark(key)
		return
	}
	if m.marked[key] {
		delete(m.marked, key)
		m.order = slices.DeleteFunc(m.order, func(k string) bool { return k == key })
	}
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
