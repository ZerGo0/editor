package issueui

import (
	"context"
	"time"

	"github.com/ZerGo0/editor/internal/ghshared"
	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/github"
	"github.com/egoist/mygo"
)

type Backend interface {
	Viewer(ctx context.Context) (github.User, error)
	SearchIssues(ctx context.Context, query string, page int) (github.IssuePage, error)
	GetIssue(ctx context.Context, repo github.Remote, number int) (github.Issue, error)
	IssueComments(ctx context.Context, repo github.Remote, number int) ([]github.Comment, error)
	AddIssueComment(ctx context.Context, repo github.Remote, number int, body string) (github.Comment, error)
	CreateIssue(ctx context.Context, repo github.Remote, in github.IssueInput) (github.Issue, error)
	EditIssue(ctx context.Context, repo github.Remote, number int, edit github.IssueEdit) (github.Issue, error)
	AssignableUsers(ctx context.Context, repo github.Remote, after string) (github.UserPage, error)
	MentionableUsers(ctx context.Context, repo github.Remote, after string) (github.UserPage, error)
	Milestones(ctx context.Context, repo github.Remote) ([]github.Milestone, error)
	Labels(ctx context.Context, repo github.Remote) ([]github.Label, error)
	CreateLinkedBranch(ctx context.Context, repo github.Remote, number int, name, baseSHA string) (github.LinkedBranch, error)
}

type Local interface {
	WorkDir() string
	Remotes(ctx context.Context) ([]git.Remote, error)
	Status(ctx context.Context) (*git.Status, error)
	Branches(ctx context.Context) ([]git.Branch, error)
	OutgoingCommits(ctx context.Context) ([]git.Commit, error)
	CreateBranch(ctx context.Context, name, start string, checkout bool) error
	Checkout(ctx context.Context, ref string, force bool) error
	CheckoutNewTracking(ctx context.Context, remoteBranch string) error
	Fetch(ctx context.Context, remote string, prune bool, progress func(line string)) error
}

type Env struct {
	Token      func(ctx context.Context, host string) (string, error)
	NewBackend func(host, token string) Backend
	OpenLocal  func(ctx context.Context, dir string) (Local, error)
	OpenURL    func(url string) error
	Copy       func(text string)
	Now        func() time.Time
}

type cliLocal struct{ *git.Repo }

func (l cliLocal) WorkDir() string { return l.Root }

func defaultEnv() Env {
	return Env{
		Token:      github.Token,
		NewBackend: func(host, token string) Backend { return github.New(host, token) },
		OpenLocal:  ghshared.LocalOpener(func(repo *git.Repo) Local { return cliLocal{repo} }),
		OpenURL:    func(url string) error { return mygo.Shell.OpenExternal(url) },
		Copy:       func(text string) { mygo.Clipboard.WriteText(text) },
		Now:        time.Now,
	}
}
