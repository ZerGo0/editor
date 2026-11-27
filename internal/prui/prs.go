package prui

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/github"
	"github.com/ZerGo0/editor/internal/host"
	"github.com/egoist/mygo/ui"
)

const (
	autoRefreshInterval = 5 * time.Minute
	pageSize            = 30
	maxUserPages        = 5
)

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

type PRs struct {
	h      host.Host
	env    Env
	ctx    context.Context
	cancel context.CancelFunc
	closed bool

	phase    phase
	phaseErr error
	connGen  int

	local       Local
	be          Backend
	repo        github.Remote
	remotes     map[string]github.Remote
	branch      branchInfo
	branchGen   int
	locals      []string
	queries     []savedQuery
	queriesSeq  int
	queriesMu   sync.Mutex
	queriesDone int
	notifs      notifState
	current     currentPR
	checks      map[int]checkState
	refreshed   time.Time
	tickAt      time.Time
	side        *sideState
	details     map[int]*prData
	diffs       map[string]*reviewTab
	logs        map[string]*logTab
	createForm  *createForm
	dialog      dialog
	picker      picker
	viewer      string

	checkoutBusy string
}

type currentPR struct {
	pr      *github.PullRequest
	loading bool
	loaded  bool
	err     error
	gen     int
}

func New(h host.Host) *PRs {
	return NewWith(h, defaultEnv())
}

func NewWith(h host.Host, env Env) *PRs {
	ctx, cancel := context.WithCancel(context.Background())
	p := &PRs{
		h:       h,
		env:     env,
		ctx:     ctx,
		cancel:  cancel,
		checks:  map[int]checkState{},
		details: map[int]*prData{},
		diffs:   map[string]*reviewTab{},
		logs:    map[string]*logTab{},
	}
	p.connect()
	return p
}

func (p *PRs) Close() {
	p.closed = true
	p.cancel()
}

func (p *PRs) post(fn func()) {
	p.h.Update(func() {
		if !p.closed {
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
	branch  branchInfo
	locals  []string
	queries []savedQuery
}

func (p *PRs) connect() {
	p.connGen++
	gen := p.connGen
	p.phase, p.phaseErr = phaseLoading, nil
	root, dataDir := p.h.RootDir(), p.h.UserDataDir()
	env := p.env
	go func() {
		conn := resolveConnection(p.ctx, env, root, dataDir)
		p.post(func() {
			if gen == p.connGen {
				p.apply(conn)
			}
		})
	}()
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
	conn := connection{phase: phaseReady, local: local, be: env.NewBackend(repo.Host, token), repo: repo, remotes: byName}
	if branches, err := local.Branches(ctx); err == nil {
		conn.branch = currentBranch(branches, repo, byName)
		conn.locals = localBranchNames(branches)
	}
	conn.queries, _ = loadQueries(queriesPath(dataDir))
	return conn
}

func (p *PRs) apply(conn connection) {
	p.phase, p.phaseErr = conn.phase, conn.err
	p.local, p.be, p.repo, p.remotes, p.branch = conn.local, conn.be, conn.repo, conn.remotes, conn.branch
	p.locals, p.queries = conn.locals, conn.queries
	p.viewer = ""
	p.notifs = notifState{}
	p.side = newSideState()
	p.checks = map[int]checkState{}
	p.details = map[int]*prData{}
	p.diffs = map[string]*reviewTab{}
	p.logs = map[string]*logTab{}
	p.current = currentPR{}
	p.refreshed = p.env.Now()
	if conn.phase == phaseReady {
		p.rebuildCustom()
		p.refreshCurrent()
		p.loadViewer()
		p.loadNotifications()
	}
}

func (p *PRs) refresh() {
	if p.phase != phaseReady {
		p.connect()
		return
	}
	p.refreshed = p.env.Now()
	for _, s := range p.side.sections() {
		s.stale = true
	}
	for _, d := range p.details {
		d.stale = true
	}
	p.notifs.fetchedAt = time.Time{}
	p.readBranch(p.refreshCurrent)
}

func (p *PRs) BranchChanged() {
	if p.closed {
		return
	}
	switch p.phase {
	case phaseNoRepo, phaseNoRemote, phaseError:
		p.connect()
	case phaseReady:
		p.readBranch(func() {
			p.refreshCurrent()
			p.branchUpdated()
		})
	}
}

func (p *PRs) readBranch(done func()) {
	p.branchGen++
	gen := p.branchGen
	local, repo, remotes := p.local, p.repo, p.remotes
	go func() {
		branches, err := local.Branches(p.ctx)
		var info branchInfo
		var names []string
		if err == nil {
			info = currentBranch(branches, repo, remotes)
			names = localBranchNames(branches)
		}
		p.post(func() {
			if gen != p.branchGen {
				return
			}
			if err == nil {
				p.branch, p.locals = info, names
			}
			done()
		})
	}()
}

func (p *PRs) refreshCurrent() {
	p.current.gen++
	gen := p.current.gen
	if p.branch.Name == "" {
		p.current = currentPR{gen: gen, loaded: true}
		return
	}
	p.current.loading = true
	be, repo, branch := p.be, p.repo, p.branch
	go func() {
		pr, err := be.CurrentBranchPR(p.ctx, repo, branch.HeadRef, branch.HeadOwner)
		var state checkState
		if err == nil && pr != nil {
			if checks, checkErr := be.Checks(p.ctx, repo, pr.HeadSHA); checkErr == nil {
				state = summarizeChecks(checks)
			}
		}
		p.post(func() {
			if gen != p.current.gen {
				return
			}
			p.current.loading, p.current.loaded, p.current.err = false, true, err
			if err != nil {
				p.authFailure(err)
				return
			}
			p.current.pr = pr
			if pr != nil {
				p.checks[pr.Number] = state
			}
		})
	}()
}

func (p *PRs) authFailure(err error) {
	var api *github.APIError
	if errors.As(err, &api) && api.Status == 401 {
		p.phase, p.phaseErr = phaseNotLoggedIn, err
	}
}

func (p *PRs) openCurrent() bool {
	if p.current.pr == nil {
		p.h.Notify("The current branch has no pull request.")
		return false
	}
	p.openPR(p.current.pr.Number, p.current.pr)
	return true
}

func (p *PRs) openPR(number int, preload *github.PullRequest) {
	d := p.dataFor(number)
	if preload != nil && !d.loaded {
		d.pr = *preload
	}
	if !d.loaded && !d.loading {
		p.loadDetail(d)
	}
	d.openedTitle = d.pr.Title
	p.h.OpenView(detailID(p.repo, number), detailTitle(number, d.pr.Title), func(c *ui.Context) { p.detailView(c, d) })
}

func (p *PRs) dataFor(number int) *prData {
	d := p.details[number]
	if d == nil {
		d = newPRData(number)
		p.details[number] = d
	}
	return d
}

func (p *PRs) checkoutPR(number int, busy *string) {
	dir := p.h.RootDir()
	checkout := p.env.Checkout
	p.act(busy, fmt.Sprintf("Checking out #%d", number), fmt.Sprintf("Cannot check out #%d", number), func(ctx context.Context) (func(), error) {
		if err := checkout(ctx, dir, number); err != nil {
			return nil, err
		}
		return func() {
			p.h.Notify(fmt.Sprintf("Checked out #%d", number))
			p.BranchChanged()
		}, nil
	})
}

func (p *PRs) act(busy *string, status, failure string, work func(ctx context.Context) (func(), error)) {
	if *busy != "" {
		return
	}
	*busy = status
	go func() {
		apply, err := work(p.ctx)
		p.post(func() {
			*busy = ""
			if err != nil {
				p.authFailure(err)
				p.h.ShowError(failure, err)
			}
			if apply != nil {
				apply()
			}
		})
	}()
}

func errorText(err error) string {
	var limit *github.RateLimitError
	var api *github.APIError
	switch {
	case errors.As(err, &limit):
		if limit.Reset.IsZero() {
			return "GitHub rate limit reached. Try again later."
		}
		return "GitHub rate limit reached. Try again after " + limit.Reset.Format("15:04") + "."
	case errors.As(err, &api):
		return fmt.Sprintf("GitHub answered %d: %s", api.Status, api.Message)
	}
	return strings.TrimPrefix(err.Error(), "github: ")
}

func appendUnique(items, more []github.PullRequest) []github.PullRequest {
	for _, pr := range more {
		if !slices.ContainsFunc(items, func(x github.PullRequest) bool { return x.Number == pr.Number }) {
			items = append(items, pr)
		}
	}
	return items
}

func (p *PRs) loadViewer() {
	gen := p.connGen
	be := p.be
	go func() {
		user, err := be.Viewer(p.ctx)
		p.post(func() {
			if err == nil && gen == p.connGen {
				p.viewer = user.Login
			}
		})
	}()
}
