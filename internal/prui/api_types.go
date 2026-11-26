package prui

import (
	"context"
	"time"

	"github.com/ZerGo0/editor/internal/github"
)

const (
	commentIssue  = "issue"
	commentReview = "review"

	actionRerun       = "rerun"
	actionRerunFailed = "rerun-failed-jobs"
	actionCancel      = "cancel"
	actionApprove     = "approve"

	mergeStateBehind   = "BEHIND"
	mergeStateDirty    = "DIRTY"
	mergeStateBlocked  = "BLOCKED"
	mergeStateClean    = "CLEAN"
	mergeStateUnstable = "UNSTABLE"

	decisionChanges  = "CHANGES_REQUESTED"
	decisionRequired = "REVIEW_REQUIRED"
)

type Reaction struct {
	Content string
	Count   int
	Viewer  bool
}

type TimelineEvent struct {
	Kind       string
	ID         string
	DatabaseID int64
	Actor      string
	Time       time.Time
	Body       string
	URL        string
	State      string
	Detail     string
	Reactions  []Reaction
	CanEdit    bool
	CanDelete  bool
}

func (e TimelineEvent) isComment() bool { return e.Kind == "IssueComment" }
func (e TimelineEvent) isReview() bool  { return e.Kind == "PullRequestReview" }

type CheckRun struct {
	Name       string
	Status     string
	Conclusion string
	URL        string
	Required   bool
	JobID      int64
	RunID      int64
}

type MilestoneRef struct {
	Number int
	Title  string
}

type LinkedIssue struct {
	Number int
	Title  string
	URL    string
}

type AutoMerge struct {
	Method string
	By     string
}

type MergeQueue struct {
	Enabled  bool
	InQueue  bool
	EntryID  string
	State    string
	Position int
}

type Overview struct {
	ID                  string
	Mergeable           string
	MergeState          string
	ReviewDecision      string
	BaseSHA             string
	HeadSHA             string
	Viewer              string
	Assignees           []string
	Milestone           *MilestoneRef
	Linked              []LinkedIssue
	Checks              []CheckRun
	AllowMerge          bool
	AllowSquash         bool
	AllowRebase         bool
	DeleteBranchOnMerge bool
	DefaultMethod       string
	CanUpdate           bool
	CanEnableAutoMerge  bool
	CanDisableAutoMerge bool
	AutoMerge           *AutoMerge
	Queue               MergeQueue
	MaintainerCanModify bool
}

type Comparison struct {
	Files   []github.PRFile
	Commits []github.Commit
	BaseSHA string
}

type Notification struct {
	ID      string
	Title   string
	Number  int
	Reason  string
	Unread  bool
	Updated time.Time
}

type NotificationPage struct {
	Items       []Notification
	Modified    string
	Poll        time.Duration
	NotModified bool
}

type Extras interface {
	Overview(ctx context.Context, repo github.Remote, number int, base string) (Overview, error)
	Timeline(ctx context.Context, repo github.Remote, number int) ([]TimelineEvent, error)
	ViewedFiles(ctx context.Context, repo github.Remote, number int) (map[string]bool, error)
	SetViewed(ctx context.Context, prID, path string, viewed bool) error
	Compare(ctx context.Context, repo github.Remote, base, head string) (Comparison, error)
	UpdateBranch(ctx context.Context, repo github.Remote, number int, headSHA string) error
	ConvertToDraft(ctx context.Context, prID string) error
	SetAutoMerge(ctx context.Context, prID, method string, enable bool) error
	Enqueue(ctx context.Context, prID string) error
	Dequeue(ctx context.Context, entryID string) error
	DeleteBranch(ctx context.Context, repo github.Remote, ref string) error
	Revert(ctx context.Context, prID string) (int, error)
	React(ctx context.Context, subjectID, content string, add bool) error
	EditComment(ctx context.Context, kind, id, body string) error
	DeleteComment(ctx context.Context, kind, id string) error
	DeleteReview(ctx context.Context, reviewID string) error
	AddFileThread(ctx context.Context, reviewID, path, body string) error
	JobLogs(ctx context.Context, repo github.Remote, jobID int64) (string, error)
	RunAction(ctx context.Context, repo github.Remote, runID int64, action string) error
	Notifications(ctx context.Context, repo github.Remote, modifiedSince string) (NotificationPage, error)
	MarkNotification(ctx context.Context, id string, done bool) error
	LocalBranchPRs(ctx context.Context, repo github.Remote, branches []string) ([]github.PullRequest, error)
}
