package prui

import (
	"context"
	"fmt"
	"log"
	"slices"
	"sync"
	"time"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/ghshared"
	"github.com/ZerGo0/editor/internal/github"
	"github.com/ZerGo0/editor/internal/host"
	"github.com/egoist/mygo/ui"
)

const (
	autoRefreshInterval = 5 * time.Minute
	pageSize            = 30
	maxUserPages        = 5
)

type PRs struct {
	ghshared.Session[connection]
	env Env

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
	p := &PRs{
		env:     env,
		checks:  map[int]checkState{},
		details: map[int]*prData{},
		diffs:   map[string]*reviewTab{},
		logs:    map[string]*logTab{},
	}
	p.Session = ghshared.NewSession(h, stateWording, p.resolveConnection, p.apply)
	p.Connect()
	return p
}

type connection struct {
	phase   ghshared.Phase
	err     error
	local   Local
	be      Backend
	repo    github.Remote
	remotes map[string]github.Remote
	branch  branchInfo
	locals  []string
	queries []savedQuery
}

func (p *PRs) resolveConnection(ctx context.Context, root, dataDir string) connection {
	res := ghshared.Resolve(ctx, p.env.OpenLocal, p.env.Token, root)
	if res.Phase != ghshared.PhaseReady {
		return connection{phase: res.Phase, repo: res.Repo, err: res.Err}
	}
	conn := connection{phase: ghshared.PhaseReady, local: res.Local, be: p.env.NewBackend(res.Repo.Host, res.Token), repo: res.Repo, remotes: res.Remotes}
	if branches, err := res.Local.Branches(ctx); err == nil {
		conn.branch = currentBranch(branches, res.Repo, res.Remotes)
		conn.locals = localBranchNames(branches)
	}
	queries, err := loadQueries(queriesPath(dataDir))
	if err != nil {
		log.Printf("load saved pull request queries: %v", err)
	}
	conn.queries = queries
	return conn
}

func (p *PRs) apply(conn connection) {
	p.Phase, p.PhaseErr = conn.phase, conn.err
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
	if conn.phase == ghshared.PhaseReady {
		p.rebuildCustom()
		p.refreshCurrent()
		p.loadViewer()
		p.loadNotifications()
	}
}

func (p *PRs) refresh() {
	if p.Phase != ghshared.PhaseReady {
		p.Connect()
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
	if p.Closed {
		return
	}
	switch p.Phase {
	case ghshared.PhaseNoRepo, ghshared.PhaseNoRemote, ghshared.PhaseError:
		p.Connect()
	case ghshared.PhaseReady:
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
	applog.Go(func() {
		branches, err := local.Branches(p.Ctx)
		var info branchInfo
		var names []string
		if err == nil {
			info = currentBranch(branches, repo, remotes)
			names = localBranchNames(branches)
		}
		p.Post(func() {
			if gen != p.branchGen {
				return
			}
			if err == nil {
				p.branch, p.locals = info, names
			}
			done()
		})
	})
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
	applog.Go(func() {
		pr, err := be.CurrentBranchPR(p.Ctx, repo, branch.HeadRef, branch.HeadOwner)
		var state checkState
		if err == nil && pr != nil {
			if checks, checkErr := be.Checks(p.Ctx, repo, pr.HeadSHA); checkErr == nil {
				state = summarizeChecks(checks)
			}
		}
		p.Post(func() {
			if gen != p.current.gen {
				return
			}
			p.current.loading, p.current.loaded, p.current.err = false, true, err
			if err != nil {
				p.AuthFailure(err)
				return
			}
			p.current.pr = pr
			if pr != nil {
				p.checks[pr.Number] = state
			}
		})
	})
}

func (p *PRs) openCurrent() bool {
	if p.current.pr == nil {
		p.Host.Notify("The current branch has no pull request.")
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
	p.Host.OpenView(detailID(p.repo, number), detailTitle(number, d.pr.Title), func(c *ui.Context) { p.detailView(c, d) })
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
	dir := p.Host.RootDir()
	checkout := p.env.Checkout
	p.Act(busy, fmt.Sprintf("Checking out #%d", number), fmt.Sprintf("Cannot check out #%d", number), func(ctx context.Context) (func(), error) {
		if err := checkout(ctx, dir, number); err != nil {
			return nil, err
		}
		return func() {
			p.Host.Notify(fmt.Sprintf("Checked out #%d", number))
			p.BranchChanged()
		}, nil
	})
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
	gen := p.ConnGen
	be := p.be
	applog.Go(func() {
		user, err := be.Viewer(p.Ctx)
		p.Post(func() {
			if err == nil && gen == p.ConnGen {
				p.viewer = user.Login
			}
		})
	})
}
