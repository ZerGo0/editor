package prui

import (
	"context"
	"time"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/github"
	"github.com/egoist/mygo"
)

type Backend interface {
	List(ctx context.Context, repo github.Remote, opts github.ListOptions) ([]github.PullRequest, error)
	Get(ctx context.Context, repo github.Remote, number int) (github.PullRequest, error)
	CurrentBranchPR(ctx context.Context, repo github.Remote, branch, headOwner string) (*github.PullRequest, error)
	Files(ctx context.Context, repo github.Remote, number int) ([]github.PRFile, error)
	Checks(ctx context.Context, repo github.Remote, sha string) ([]github.Check, error)
	Comments(ctx context.Context, repo github.Remote, number int) ([]github.Comment, error)
	Reviews(ctx context.Context, repo github.Remote, number int) ([]github.Review, error)
	ReviewThreads(ctx context.Context, repo github.Remote, number int) ([]github.Thread, error)
	AddComment(ctx context.Context, repo github.Remote, number int, body string) (github.Comment, error)
	Reply(ctx context.Context, threadID, body string) (github.Comment, error)
	StartReview(ctx context.Context, repo github.Remote, number int) (string, error)
	AddReviewComment(ctx context.Context, reviewID string, opts github.ReviewCommentOptions) (github.Thread, error)
	SubmitReview(ctx context.Context, reviewID, event, body string) error
	ResolveThread(ctx context.Context, threadID string) error
	UnresolveThread(ctx context.Context, threadID string) error
	Merge(ctx context.Context, repo github.Remote, number int, method, title, message string) error
	Create(ctx context.Context, repo github.Remote, opts github.CreateOptions) (github.PullRequest, error)
	MarkReady(ctx context.Context, repo github.Remote, number int) error
	Commits(ctx context.Context, repo github.Remote, number int) ([]github.Commit, error)
	EditIssue(ctx context.Context, repo github.Remote, number int, edit github.IssueEdit) (github.Issue, error)
	Viewer(ctx context.Context) (github.User, error)
	Labels(ctx context.Context, repo github.Remote) ([]github.Label, error)
	Milestones(ctx context.Context, repo github.Remote) ([]github.Milestone, error)
	AssignableUsers(ctx context.Context, repo github.Remote, after string) (github.UserPage, error)
	RequestReviewers(ctx context.Context, repo github.Remote, number int, users, teams []string) error
	MergeBase(ctx context.Context, repo github.Remote, base, head string) (string, error)
	FileAt(ctx context.Context, repo github.Remote, ref, path string) ([]byte, error)
	Extras
}

type ghBackend struct {
	*github.Client
	*restClient
}

type Local interface {
	Remotes(ctx context.Context) ([]git.Remote, error)
	Branches(ctx context.Context) ([]git.Branch, error)
	Show(ctx context.Context, rev, path string) ([]byte, error)
	Push(ctx context.Context, o git.PushOptions) error
	Checkout(ctx context.Context, ref string, force bool) error
	CompareBranches(ctx context.Context, a, b string) (git.BranchComparison, error)
}

type Env struct {
	Token      func(ctx context.Context, host string) (string, error)
	NewBackend func(host, token string) Backend
	OpenLocal  func(ctx context.Context, dir string) (Local, error)
	Checkout   func(ctx context.Context, dir string, number int) error
	Confirm    func(win *mygo.Window, opts ConfirmOptions) bool
	Now        func() time.Time
	OpenURL    func(url string) error
	CopyText   func(text string)
}

type ConfirmOptions struct {
	Message string
	Detail  string
	Action  string
}
