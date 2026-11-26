package diffmode

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	DefaultDelay           = 300 * time.Millisecond
	MaxMoveChangedLines    = 5000
	MaxWhitespaceTextBytes = 16 << 20
	moveRowHeight          = 22
	noticeLimit            = 90
	noticePrefix           = "Showing all changes: "
	unsetMode              = git.WhitespaceMode(-1)
)

var errTooLarge = errors.New("the texts are too large to filter")

type Differ func(ctx context.Context, old, new string, mode git.WhitespaceMode) ([]byte, error)

type Source struct {
	Old, New string
	Live     *editor.Buffer
	Language *editor.Language
}

type moveKey struct {
	index int
	side  editor.DiffSide
}

type stamp struct {
	old, new       *editor.Buffer
	oldVer, newVer uint64
}

type patchResult struct {
	patch editor.FilePatch
	old   string
}

type Engine struct {
	State      *editor.DiffState
	Post       func(func())
	Differ     Differ
	Delay      time.Duration
	LineNumber func(side editor.DiffSide, line int) int

	ctx    context.Context
	src    *Source
	oldBuf *editor.Buffer
	ownBuf *editor.Buffer
	ready  bool
	base   func(c *ui.Context, a editor.DiffAnnotation)

	mode     git.WhitespaceMode
	filtered bool
	busy     bool
	notice   string
	wsGen    int
	wsCancel context.CancelFunc
	wsTimer  *time.Timer
	liveVer  uint64

	moves      []Move
	annots     []editor.DiffAnnotation
	stamp      stamp
	moveGen    int
	moveCancel context.CancelFunc
	moveTimer  *time.Timer
}

func NewEngine(ctx context.Context, d *editor.DiffState, post func(func())) *Engine {
	return &Engine{State: d, Post: post, Differ: git.DiffTexts, Delay: DefaultDelay, ctx: ctx, mode: unsetMode}
}

func (e *Engine) Filtered() bool { return e.filtered }

func (e *Engine) Busy() bool { return e.busy }

func (e *Engine) Ready() bool { return e.ready }

func (e *Engine) Notice() string { return e.notice }

func (e *Engine) Moves() []Move { return e.moves }

func (e *Engine) Close() {
	e.cancelWhitespace()
	e.cancelMoves()
}

func (e *Engine) SetSource(s Source) {
	e.src = &s
	e.mode = unsetMode
	e.syncWhitespace()
}

func (e *Engine) Refresh() {
	e.syncWhitespace()
	e.syncMoves()
}

func (e *Engine) Install(base func(c *ui.Context, a editor.DiffAnnotation)) {
	e.base = base
	e.State.RenderAnnotation = func(c *ui.Context, a editor.DiffAnnotation) {
		if k, ok := a.Key.(moveKey); ok {
			e.moveRow(c, k)
			return
		}
		if e.base != nil {
			e.base(c, a)
		}
	}
}

func (e *Engine) Annotations(base []editor.DiffAnnotation) []editor.DiffAnnotation {
	switch {
	case len(e.annots) == 0:
		return base
	case len(base) == 0:
		return e.annots
	}
	return append(append(make([]editor.DiffAnnotation, 0, len(base)+len(e.annots)), base...), e.annots...)
}

func (e *Engine) after(d time.Duration, fn func()) *time.Timer {
	return time.AfterFunc(d, func() { e.Post(fn) })
}

func (e *Engine) cancelWhitespace() {
	e.wsGen++
	if e.wsCancel != nil {
		e.wsCancel()
		e.wsCancel = nil
	}
	if e.wsTimer != nil {
		e.wsTimer.Stop()
		e.wsTimer = nil
	}
	e.busy = false
}

func (e *Engine) syncWhitespace() {
	s := e.src
	if s == nil {
		return
	}
	want := Session.Whitespace
	if want == e.mode {
		if want == git.WhitespaceNone || s.Live == nil || s.Live.Version() == e.liveVer {
			return
		}
		e.liveVer = s.Live.Version()
		e.debounceWhitespace()
		return
	}
	e.mode = want
	e.cancelWhitespace()
	e.notice, e.filtered = "", false
	if want == git.WhitespaceNone {
		e.showPlain()
		return
	}
	e.startPatch()
}

func (e *Engine) debounceWhitespace() {
	e.cancelWhitespace()
	e.busy = true
	gen := e.wsGen
	e.wsTimer = e.after(e.Delay, func() {
		if gen == e.wsGen {
			e.startPatch()
		}
	})
}

func (e *Engine) showPlain() {
	s, d := e.src, e.State
	if e.oldBuf == nil {
		e.oldBuf = editor.NewBuffer(s.Old)
		e.oldBuf.SetLanguage(s.Language)
	} else if e.oldBuf.String() != s.Old {
		e.oldBuf.Reset(s.Old)
	}
	newBuf := s.Live
	if newBuf == nil {
		if e.ownBuf == nil {
			e.ownBuf = editor.NewBuffer(s.New)
			e.ownBuf.SetLanguage(s.Language)
		} else if e.ownBuf.String() != s.New {
			e.ownBuf.Reset(s.New)
		}
		newBuf = e.ownBuf
	}
	d.Editable = s.Live != nil
	d.Old, d.New = e.oldBuf, newBuf
	e.ready = true
}

func (e *Engine) startPatch() {
	e.cancelWhitespace()
	s := e.src
	newText := s.New
	if s.Live != nil {
		newText = s.Live.String()
		e.liveVer = s.Live.Version()
	}
	if len(s.Old)+len(newText) > MaxWhitespaceTextBytes {
		e.fallback(errTooLarge)
		return
	}
	ctx, cancel := context.WithCancel(e.ctx)
	e.wsCancel, e.busy = cancel, true
	gen := e.wsGen
	mode, oldText, differ := e.mode, s.Old, e.Differ
	go func() {
		res, err := buildPatch(ctx, differ, oldText, newText, mode)
		e.Post(func() {
			if gen != e.wsGen || ctx.Err() != nil {
				return
			}
			e.busy = false
			e.finishPatch(res, newText, err)
		})
	}()
}

func buildPatch(ctx context.Context, differ Differ, oldText, newText string, mode git.WhitespaceMode) (patchResult, error) {
	out, err := differ(ctx, oldText, newText, mode)
	if err != nil {
		return patchResult{}, err
	}
	var fp editor.FilePatch
	if len(out) > 0 {
		files, err := editor.ParsePatch(string(out))
		if err != nil {
			return patchResult{}, fmt.Errorf("read the patch: %w", err)
		}
		if len(files) != 1 {
			return patchResult{}, fmt.Errorf("read the patch: %d files", len(files))
		}
		fp = files[0]
	}
	aligned, alignedOld, err := AlignPatch(fp, oldText, newText)
	if err != nil {
		return patchResult{}, err
	}
	return patchResult{patch: aligned, old: alignedOld}, nil
}

func (e *Engine) finishPatch(res patchResult, newText string, err error) {
	d := e.State
	if err == nil {
		d.Editable = false
		err = d.SetPatchFiles(res.patch, res.old, newText)
	}
	if err != nil {
		e.fallback(err)
		return
	}
	e.notice, e.filtered, e.ready = "", true, true
	if l := e.src.Language; l != nil {
		d.Old.SetLanguage(l)
		d.New.SetLanguage(l)
	}
}

func (e *Engine) fallback(err error) {
	msg := strings.Join(strings.Fields(err.Error()), " ")
	if len(msg) > noticeLimit {
		msg = msg[:noticeLimit] + "..."
	}
	e.notice, e.filtered = noticePrefix+msg, false
	e.showPlain()
}
