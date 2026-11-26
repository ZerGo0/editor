package gitui

import (
	"time"

	"github.com/ZerGo0/editor/internal/git"
)

type PullMode int

const (
	PullMerge PullMode = iota
	PullRebase
	PullFFOnly
)

const defaultLogPageSize = 300

type Settings struct {
	AutoFetch   time.Duration
	PullMode    PullMode
	LogPageSize int
}

func DefaultSettings() Settings {
	return Settings{PullMode: PullMerge, LogPageSize: defaultLogPageSize}
}

func (g *Git) SetSettings(s Settings) {
	if s.LogPageSize <= 0 {
		s.LogPageSize = defaultLogPageSize
	}
	changed := g.settings.AutoFetch != s.AutoFetch
	g.settings = s
	if changed {
		g.scheduleAutoFetch()
	}
}

func (g *Git) logPageSize() int {
	if g == nil || g.settings.LogPageSize <= 0 {
		return defaultLogPageSize
	}
	return g.settings.LogPageSize
}

func (g *Git) pullOptions() git.PullOptions {
	return git.PullOptions{Rebase: g.settings.PullMode == PullRebase, FFOnly: g.settings.PullMode == PullFFOnly}
}

func (g *Git) scheduleAutoFetch() {
	if g.autoFetch != nil {
		g.autoFetch.Stop()
		g.autoFetch = nil
	}
	if g.closed || !g.ready() || g.settings.AutoFetch <= 0 {
		return
	}
	g.autoFetch = time.AfterFunc(g.settings.AutoFetch, func() { g.post(g.fetchQuietly) })
}

func (g *Git) fetchQuietly() {
	if !g.ready() {
		return
	}
	if g.busy != "" {
		g.scheduleAutoFetch()
		return
	}
	r := g.repo
	go func() {
		err := r.Fetch(g.ctx, "", true, nil)
		g.post(func() {
			if err == nil {
				g.refreshNow()
			}
			g.scheduleAutoFetch()
		})
	}()
}
