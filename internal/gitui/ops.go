package gitui

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/git"
	"github.com/egoist/mygo"
)

const progressInterval = 150 * time.Millisecond

type op struct {
	title      string
	banner     bool
	background bool
	network    bool
	remote     bool
	run        func(ctx context.Context, r repo, progress func(string)) (git.OpResult, error)
	done       func(res git.OpResult)
}

func (o op) cancelable() bool {
	return o.network || o.remote
}

func (g *Git) do(o op) {
	if !g.ready() {
		return
	}
	if reason := g.blocked(o); reason != "" {
		g.h.Notify(reason)
		return
	}
	ctx, cancel := context.WithCancel(g.ctx)
	var t *task
	if o.banner {
		t = g.addTask(o.title, cancel, o.cancelable(), !o.background)
	}
	r := g.repo
	sink := &progressSink{post: g.post, set: t.report}
	applog.Go(func() {
		if !o.network {
			g.writeMu.Lock()
			defer g.writeMu.Unlock()
		}
		res, err := o.run(ctx, r, sink.line)
		cancel()
		g.post(func() {
			g.removeTask(t)
			g.refreshNow()
			if errors.Is(err, context.Canceled) {
				return
			}
			if err != nil {
				g.showError(o.title+" failed", err)
				return
			}
			if res.Stopped {
				g.stopped(o.title)
			}
			if o.done != nil {
				o.done(res)
			}
		})
	})
}

func (g *Git) blocked(o op) string {
	switch {
	case !o.banner:
		return ""
	case g.busy != "":
		return g.busy + " is still running."
	}
	if t := g.remoteTask(); t != nil && o.cancelable() {
		return t.title + " is still running."
	}
	return ""
}

func (g *Git) doSimple(title string, run func(ctx context.Context, r repo) error, done func()) {
	g.do(op{
		title: title,
		run: func(ctx context.Context, r repo, _ func(string)) (git.OpResult, error) {
			return git.OpResult{}, run(ctx, r)
		},
		done: func(git.OpResult) {
			if done != nil {
				done()
			}
		},
	})
}

func (g *Git) stopped(title string) {
	g.h.Notify(title + " stopped. Resolve the conflicts or finish the edit, then continue.")
	g.openConflictsIfAny()
}

func (g *Git) showError(title string, err error) {
	g.h.ShowError(title, describe(err))
}

type describedError struct {
	text string
	err  error
}

func (e describedError) Error() string { return e.text }
func (e describedError) Unwrap() error { return e.err }

func describe(err error) error {
	if text, ok := git.Describe(err); ok {
		return describedError{text: text, err: err}
	}
	return err
}

type progressSink struct {
	mu   sync.Mutex
	last time.Time
	post func(func())
	set  func(string)
}

func (p *progressSink) line(s string) {
	p.mu.Lock()
	now := time.Now()
	due := now.Sub(p.last) >= progressInterval
	if due {
		p.last = now
	}
	p.mu.Unlock()
	if due {
		p.post(func() { p.set(s) })
	}
}

type question struct {
	message     string
	detail      string
	action      string
	destructive bool
}

func (g *Git) confirm(q question, yes func()) {
	win := g.h.Window()
	applog.Go(func() {
		if g.deps.ask(win, q) {
			g.post(yes)
		}
	})
}

func askNative(win *mygo.Window, q question) bool {
	opts := mygo.MessageOptions{
		Parent:       win,
		Type:         mygo.MessageWarning,
		Message:      q.message,
		Detail:       q.detail,
		Buttons:      []string{q.action, "Cancel"},
		CancelButton: 1,
	}
	if q.destructive {
		opts.DestructiveButtons = []int{0}
	}
	res, err := mygo.Dialog.Message(opts)
	return err == nil && res.Button == 0
}
