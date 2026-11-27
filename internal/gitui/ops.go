package gitui

import (
	"context"
	"sync"
	"time"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/git"
	"github.com/egoist/mygo"
)

const progressInterval = 150 * time.Millisecond

type op struct {
	title   string
	banner  bool
	network bool
	run     func(ctx context.Context, r repo, progress func(string)) (git.OpResult, error)
	done    func(res git.OpResult)
}

func (g *Git) do(o op) {
	if !g.ready() {
		return
	}
	if o.banner {
		if g.busy != "" {
			g.h.Notify(g.busy + " is still running.")
			return
		}
		g.busy, g.progress = o.title, ""
	}
	r := g.repo
	sink := &progressSink{post: g.post, set: func(line string) {
		if o.banner {
			g.progress = line
		}
	}}
	applog.Go(func() {
		if !o.network {
			g.writeMu.Lock()
			defer g.writeMu.Unlock()
		}
		res, err := o.run(g.ctx, r, sink.line)
		g.post(func() {
			if o.banner {
				g.busy, g.progress = "", ""
			}
			g.refreshNow()
			if err != nil {
				g.h.ShowError(o.title+" failed", err)
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
