package gitui

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/host"
)

type phase int

const (
	phaseLoading phase = iota
	phaseNoGit
	phaseNoRepo
	phaseError
	phaseReady
)

type Git struct {
	h      host.Host
	deps   deps
	ctx    context.Context
	cancel context.CancelFunc
	closed bool

	phase   phase
	openErr error
	repo    repo
	paths   pathMap

	snap    snapshot
	refresh refresher

	writeMu  sync.Mutex
	busy     string
	progress string

	commit     commitState
	changes    changesState
	graph      graphState
	log        *logState
	branches   branchesState
	push       pushState
	stash      stashState
	stashes    stashListState
	shelf      shelfState
	rebase     rebaseState
	conflicts  conflictsState
	worktrees  worktreeState
	submodules submoduleState
	remotes    remotesState
	change     changeState

	newBranchDlg newBranchState
	tagDlg       tagState

	bases      map[string]*baseEntry
	untracked  untrackedCache
	blames     map[string]*blameState
	blameOpts  blameOptions
	diffOpts   diffOptions
	tabs       map[string]*diffTab
	tabOrder   []string
	patchTabs  map[string]*patchTab
	patchOrder []string
	histories  map[string]*logState
	mergeTools map[string]*mergeTool
	clipboard  string
	wantsPanel bool

	settings  Settings
	autoFetch *time.Timer
	template  func() string
	headHook  func()
}

func New(h host.Host) *Git {
	return newGit(h, defaultDeps())
}

func newGit(h host.Host, d deps) *Git {
	ctx, cancel := context.WithCancel(context.Background())
	g := &Git{
		h: h, deps: d, ctx: ctx, cancel: cancel,
		bases:      map[string]*baseEntry{},
		blames:     map[string]*blameState{},
		tabs:       map[string]*diffTab{},
		patchTabs:  map[string]*patchTab{},
		histories:  map[string]*logState{},
		mergeTools: map[string]*mergeTool{},
		settings:   DefaultSettings(),
	}
	g.initViews()
	g.open()
	return g
}

func (g *Git) initViews() {
	g.log = newLogState(g, "")
	g.changes.cursor = -1
	g.graph.cursor = -1
	g.changes.stack.Weights = map[string]float32{paneChanges: changesPaneWeight, paneGraph: graphPaneWeight}
}

func (g *Git) open() {
	g.phase, g.openErr = phaseLoading, nil
	root, data := g.h.RootDir(), g.h.UserDataDir()
	go func() {
		if !g.deps.available() {
			g.post(func() { g.phase = phaseNoGit })
			return
		}
		r, err := g.deps.open(g.ctx, root, data)
		g.post(func() {
			switch {
			case errors.Is(err, git.ErrNotRepo):
				g.phase = phaseNoRepo
			case errors.Is(err, git.ErrUnavailable):
				g.phase = phaseNoGit
			case err != nil:
				g.phase, g.openErr = phaseError, err
			default:
				g.setRepo(r)
			}
		})
	}()
}

func (g *Git) setRepo(r repo) {
	g.repo = r
	g.paths = newPathMap(g.h.RootDir(), r.WorkDir())
	g.phase = phaseReady
	g.refreshNow()
	g.scheduleAutoFetch()
	go g.deps.watch(g.ctx, r.MetaDir(), func() { g.post(g.scheduleRefresh) })
}

func (g *Git) Close() {
	if g.closed {
		return
	}
	g.closed = true
	g.cancel()
	g.refresh.stop()
	if g.autoFetch != nil {
		g.autoFetch.Stop()
	}
	g.log.stopTimer()
	for _, s := range g.histories {
		s.stopTimer()
	}
}

func (g *Git) post(fn func()) {
	g.h.Update(func() {
		if !g.closed {
			fn()
		}
	})
}

func (g *Git) ready() bool {
	return g.phase == phaseReady && g.repo != nil
}

func (g *Git) ChangeCount() int {
	if g.snap.status == nil {
		return 0
	}
	n := 0
	for _, f := range g.snap.status.Files {
		if f.Index != statusIgnored {
			n++
		}
	}
	return n
}

func (g *Git) WantsPanel() bool {
	want := g.wantsPanel
	g.wantsPanel = false
	return want
}

func (g *Git) initRepository() {
	if g.phase == phaseReady {
		return
	}
	root := g.h.RootDir()
	g.phase = phaseLoading
	go func() {
		err := g.deps.initRepo(g.ctx, root)
		g.post(func() {
			if err != nil {
				g.phase, g.openErr = phaseError, err
				g.h.ShowError("Cannot initialize the repository", err)
				return
			}
			g.h.Notify("Initialized a Git repository")
			g.open()
		})
	}()
}
