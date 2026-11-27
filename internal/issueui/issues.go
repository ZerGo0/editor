package issueui

import (
	"context"
	"errors"
	"slices"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/github"
	"github.com/ZerGo0/editor/internal/host"
)

const sideViewTitle = "Issues"

type phase int

const (
	phaseLoading phase = iota
	phaseNoRepo
	phaseNoRemote
	phaseNoGH
	phaseNotLoggedIn
	phaseError
	phaseReady
)

type Issues struct {
	h      host.Host
	env    Env
	ctx    context.Context
	cancel context.CancelFunc
	closed bool

	settings Settings

	phase    phase
	phaseErr error
	connGen  int

	local   Local
	be      Backend
	repo    github.Remote
	remotes map[string]github.Remote
	user    string

	side    *sideState
	details map[int]*issueData
	create  *createForm
	meta    metaState
	mention mentionState
	work    workState
	working workingRuntime
}

func New(h host.Host) *Issues {
	return NewWith(h, defaultEnv())
}

func NewWith(h host.Host, env Env) *Issues {
	ctx, cancel := context.WithCancel(context.Background())
	i := &Issues{
		h:        h,
		env:      env,
		ctx:      ctx,
		cancel:   cancel,
		settings: DefaultSettings(),
		details:  map[int]*issueData{},
	}
	i.connect()
	return i
}

func (i *Issues) Close() {
	i.closed = true
	i.cancel()
}

func (i *Issues) Title() string {
	return sideViewTitle
}

func (i *Issues) Badge() int {
	if i.phase != phaseReady || i.side == nil || len(i.side.sections) == 0 {
		return 0
	}
	first := i.side.sections[0]
	if !first.loaded {
		return 0
	}
	return first.total
}

func (i *Issues) SetSettings(s Settings) {
	i.settings = s.normalized()
	if i.phase == phaseReady {
		i.side = newSideState(i.settings.Queries)
	}
}

func (i *Issues) post(fn func()) {
	i.h.Update(func() {
		if !i.closed {
			fn()
		}
	})
}

type connection struct {
	phase   phase
	err     error
	local   Local
	be      Backend
	repo    github.Remote
	remotes map[string]github.Remote
	user    string
	work    workState
}

func (i *Issues) connect() {
	i.connGen++
	gen := i.connGen
	i.phase, i.phaseErr = phaseLoading, nil
	root, dataDir, env := i.h.RootDir(), i.h.UserDataDir(), i.env
	applog.Go(func() {
		conn := resolveConnection(i.ctx, env, root, dataDir)
		i.post(func() {
			if gen == i.connGen {
				i.apply(conn)
			}
		})
	})
}

func resolveConnection(ctx context.Context, env Env, root, dataDir string) connection {
	local, err := env.OpenLocal(ctx, root)
	if err != nil {
		if errors.Is(err, git.ErrNotRepo) {
			return connection{phase: phaseNoRepo}
		}
		return connection{phase: phaseError, err: err}
	}
	remotes, err := local.Remotes(ctx)
	if err != nil {
		return connection{phase: phaseError, err: err}
	}
	repo, byName, ok := pickRemote(remotes)
	if !ok {
		return connection{phase: phaseNoRemote}
	}
	token, err := env.Token(ctx, repo.Host)
	switch {
	case errors.Is(err, github.ErrNoGH):
		return connection{phase: phaseNoGH, repo: repo}
	case errors.Is(err, github.ErrNotLoggedIn):
		return connection{phase: phaseNotLoggedIn, repo: repo, err: err}
	case err != nil:
		return connection{phase: phaseError, repo: repo, err: err}
	}
	be := env.NewBackend(repo.Host, token)
	viewer, err := be.Viewer(ctx)
	switch {
	case isUnauthorized(err):
		return connection{phase: phaseNotLoggedIn, repo: repo, err: err}
	case err != nil:
		return connection{phase: phaseError, repo: repo, err: err}
	}
	conn := connection{phase: phaseReady, local: local, be: be, repo: repo, remotes: byName, user: viewer.Login}
	conn.work, _ = loadWork(dataDir, repo)
	if conn.work.Active != nil {
		if status, err := local.Status(ctx); err == nil && status.Branch != conn.work.Active.Branch {
			conn.work.deactivate()
		}
	}
	return conn
}

func (i *Issues) apply(conn connection) {
	i.phase, i.phaseErr = conn.phase, conn.err
	i.local, i.be, i.repo, i.remotes, i.user, i.work = conn.local, conn.be, conn.repo, conn.remotes, conn.user, conn.work
	i.side = newSideState(i.settings.Queries)
	i.details = map[int]*issueData{}
	i.meta = metaState{}
	i.mention = mentionState{}
	i.working = workingRuntime{}
}

func (i *Issues) ready() bool {
	return i.phase == phaseReady
}

func (i *Issues) requireReady() bool {
	if i.ready() {
		return true
	}
	i.h.Notify(i.stateMessage().title)
	return false
}

func (i *Issues) refresh() {
	if !i.ready() {
		i.connect()
		return
	}
	i.markStale()
}

func (i *Issues) markStale() {
	for _, s := range i.side.all() {
		s.stale = true
	}
	for _, d := range i.details {
		d.stale = true
	}
}

func (i *Issues) authFailure(err error) {
	if isUnauthorized(err) {
		i.phase, i.phaseErr = phaseNotLoggedIn, err
	}
}

func (i *Issues) act(busy *string, status, failure string, work func(ctx context.Context) (func(), error)) {
	if *busy != "" {
		return
	}
	*busy = status
	applog.Go(func() {
		apply, err := work(i.ctx)
		i.post(func() {
			*busy = ""
			if err != nil {
				i.authFailure(err)
				i.h.ShowError(failure, err)
				return
			}
			if apply != nil {
				apply()
			}
		})
	})
}

func (i *Issues) allKnownIssues() []github.Issue {
	var out []github.Issue
	add := func(issue github.Issue) {
		if !slices.ContainsFunc(out, func(x github.Issue) bool { return x.Number == issue.Number }) {
			out = append(out, issue)
		}
	}
	if i.side != nil {
		for _, s := range i.side.all() {
			for _, issue := range s.items {
				add(issue)
			}
		}
	}
	for _, d := range i.details {
		if d.issue.Number > 0 {
			add(d.issue)
		}
	}
	return out
}
