package gitui

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/egoist/mygo"
)

const (
	refreshDelay   = 300 * time.Millisecond
	indexLockName  = "index.lock"
	lockRetryLimit = 10
	statusIgnored  = '!'
)

var ignoredGitDirs = map[string]bool{"objects": true, "logs": true, "hooks": true, "lfs": true}

type snapshot struct {
	status   *git.Status
	op       git.RepoState
	branches []git.Branch
	remotes  []git.Remote
	err      error
}

type refresher struct {
	timer   *time.Timer
	running bool
	again   bool
	gen     int
	retries int
}

func (r *refresher) stop() {
	if r.timer != nil {
		r.timer.Stop()
	}
}

func (g *Git) DiskChanged() {
	g.scheduleRefresh()
}

func (g *Git) scheduleRefresh() {
	if !g.ready() || g.closed {
		return
	}
	if g.refresh.timer != nil {
		g.refresh.timer.Reset(refreshDelay)
		return
	}
	g.refresh.timer = time.AfterFunc(refreshDelay, func() { g.post(g.refreshNow) })
}

func (g *Git) refreshNow() {
	if !g.ready() {
		return
	}
	if g.refresh.running {
		g.refresh.again = true
		return
	}
	g.refresh.running = true
	g.refresh.gen++
	gen, r := g.refresh.gen, g.repo
	force := g.refresh.retries >= lockRetryLimit
	go func() {
		if !force && locked(r.MetaDir()) {
			g.post(func() {
				g.refresh.running = false
				g.refresh.retries++
				g.scheduleRefresh()
			})
			return
		}
		snap := loadSnapshot(g.ctx, r)
		g.post(func() {
			g.refresh.running, g.refresh.retries = false, 0
			if gen == g.refresh.gen {
				g.applySnapshot(snap)
			}
			if g.refresh.again {
				g.refresh.again = false
				g.refreshNow()
			}
		})
	}()
}

func locked(gitDir string) bool {
	_, err := os.Stat(filepath.Join(gitDir, indexLockName))
	return err == nil
}

func loadSnapshot(ctx context.Context, r repo) snapshot {
	st, err := r.Status(ctx)
	if err != nil {
		return snapshot{err: err}
	}
	op, err := r.State(ctx)
	if err != nil {
		return snapshot{err: err}
	}
	branches, err := r.Branches(ctx)
	if err != nil {
		return snapshot{err: err}
	}
	remotes, err := r.Remotes(ctx)
	if err != nil {
		return snapshot{err: err}
	}
	return snapshot{status: st, op: op, branches: branches, remotes: remotes}
}

func (g *Git) applySnapshot(s snapshot) {
	old := g.snap.status
	g.snap = s
	if s.status == nil || old == nil {
		return
	}
	switch {
	case old.OID != s.status.OID || old.Branch != s.status.Branch:
		g.headChanged()
	case old.Upstream != s.status.Upstream || old.Ahead != s.status.Ahead || old.Behind != s.status.Behind:
		g.reloadGraphIfStarted()
	}
}

func (g *Git) reloadGraphIfStarted() {
	if g.graph.started {
		g.reloadGraph()
	}
}

func (g *Git) headChanged() {
	if g.headHook != nil {
		g.headHook()
	}
	g.bases = map[string]*baseEntry{}
	g.reblameAll()
	if g.log.started {
		g.log.reload()
	}
	g.reloadGraphIfStarted()
}

func (g *Git) currentBranch() string {
	if g.snap.status == nil {
		return ""
	}
	return g.snap.status.Branch
}

func (g *Git) headLabel() string {
	st := g.snap.status
	switch {
	case st == nil:
		return ""
	case st.Detached:
		return shortHash(st.OID)
	case st.Branch == "":
		return "No branch"
	}
	return st.Branch
}

func watchDir(ctx context.Context, dir string, changed func()) {
	w, err := mygo.WatchFiles(ctx, dir, mygo.FileWatchOptions{
		Recursive: true,
		HintsOnly: true,
		Ignore: func(p string, isDir bool) bool {
			return isDir && ignoredGitDirs[firstSegment(p)]
		},
	})
	if err != nil {
		return
	}
	defer func() { _ = w.Close() }()
	for {
		if _, err := w.Next(ctx); err != nil {
			return
		}
		changed()
	}
}

func firstSegment(p string) string {
	p = path.Clean(p)
	first, _, _ := strings.Cut(p, "/")
	return first
}
