package issueui

import (
	"context"
	"log"
	"slices"

	"github.com/ZerGo0/editor/internal/ghshared"
	"github.com/ZerGo0/editor/internal/github"
	"github.com/ZerGo0/editor/internal/host"
)

const sideViewTitle = "Issues"

var stateWording = ghshared.StateWording{
	NoRepoDetail: "Open a folder that is a Git repository to see its issues.",
	ErrorTitle:   "Cannot connect to GitHub",
}

type Issues struct {
	ghshared.Session[connection]
	env Env

	settings Settings

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
	i := &Issues{
		env:      env,
		settings: DefaultSettings(),
		details:  map[int]*issueData{},
	}
	i.Session = ghshared.NewSession(h, stateWording, i.resolveConnection, i.apply)
	i.Connect()
	return i
}

func (i *Issues) Title() string {
	return sideViewTitle
}

func (i *Issues) Badge() int {
	if i.Phase != ghshared.PhaseReady || i.side == nil || len(i.side.sections) == 0 {
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
	if i.Phase == ghshared.PhaseReady {
		i.side = newSideState(i.settings.Queries)
	}
}

type connection struct {
	phase   ghshared.Phase
	err     error
	local   Local
	be      Backend
	repo    github.Remote
	remotes map[string]github.Remote
	user    string
	work    workState
}

func (i *Issues) resolveConnection(ctx context.Context, root, dataDir string) connection {
	res := ghshared.Resolve(ctx, i.env.OpenLocal, i.env.Token, root)
	if res.Phase != ghshared.PhaseReady {
		return connection{phase: res.Phase, repo: res.Repo, err: res.Err}
	}
	be := i.env.NewBackend(res.Repo.Host, res.Token)
	viewer, err := be.Viewer(ctx)
	switch {
	case ghshared.IsUnauthorized(err):
		return connection{phase: ghshared.PhaseNotLoggedIn, repo: res.Repo, err: err}
	case err != nil:
		return connection{phase: ghshared.PhaseError, repo: res.Repo, err: err}
	}
	conn := connection{phase: ghshared.PhaseReady, local: res.Local, be: be, repo: res.Repo, remotes: res.Remotes, user: viewer.Login}
	work, err := loadWork(dataDir, res.Repo)
	if err != nil {
		log.Printf("load working issue: %v", err)
	}
	conn.work = work
	if conn.work.Active != nil {
		if status, err := res.Local.Status(ctx); err == nil && status.Branch != conn.work.Active.Branch {
			conn.work.deactivate()
		}
	}
	return conn
}

func (i *Issues) apply(conn connection) {
	i.Phase, i.PhaseErr = conn.phase, conn.err
	i.local, i.be, i.repo, i.remotes, i.user, i.work = conn.local, conn.be, conn.repo, conn.remotes, conn.user, conn.work
	i.side = newSideState(i.settings.Queries)
	i.details = map[int]*issueData{}
	i.meta = metaState{}
	i.mention = mentionState{}
	i.working = workingRuntime{}
}

func (i *Issues) ready() bool {
	return i.Phase == ghshared.PhaseReady
}

func (i *Issues) requireReady() bool {
	if i.ready() {
		return true
	}
	i.Host.Notify(i.StateMessage(i.repo.Host).Title)
	return false
}

func (i *Issues) refresh() {
	if !i.ready() {
		i.Connect()
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
