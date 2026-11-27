package prui

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/ghshared"
	"github.com/ZerGo0/editor/internal/github"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
)

const (
	checkNameFmt        = "%s: %s"
	mergeSubjectFmt     = "%s (#%d)"
	deleteBranchLabel   = "Delete branch after merge"
	resolveAfterLabel   = "Resolve thread after replying"
	confirmMergeAction  = "Merge"
	headRepoMissingNote = "The head repository is not available."
)

var mergeChoices = []mergeChoice{
	{"Create a merge commit", github.MergeMethodMerge},
	{"Squash and merge", github.MergeMethodSquash},
	{"Rebase and merge", github.MergeMethodRebase},
}

type checkRow struct {
	github.Check
	Required bool
	JobID    int64
	RunID    int64
}

func enrichChecks(checks []github.Check, runs []CheckRun) []checkRow {
	byName := make(map[string]CheckRun, len(runs))
	for _, r := range runs {
		byName[r.Name] = r
	}
	rows := make([]checkRow, 0, len(checks))
	for _, c := range checks {
		row := checkRow{Check: c}
		if r, ok := byName[c.Name]; ok {
			row.Required, row.JobID, row.RunID = r.Required, r.JobID, r.RunID
		}
		rows = append(rows, row)
	}
	return rows
}

type prData struct {
	number        int
	pr            github.PullRequest
	loaded        bool
	loading       bool
	stale         bool
	gen           int
	err           error
	partErrs      []string
	files         []github.PRFile
	checks        []checkRow
	timeline      []TimelineEvent
	reviews       []github.Review
	threads       []github.Thread
	commits       []github.Commit
	viewed        map[string]bool
	ov            Overview
	hasOv         bool
	since         []github.PRFile
	sinceSHA      string
	pendingReview string
	baseSHA       string
	busy          string
	draft         string
	openedTitle   string
	hideViewed    bool
	editingTitle  bool
	titleDraft    string
	loadedAt      time.Time
	tickAt        time.Time
	fileList      ui.ListState
	fileRow       int
	sinceList     ui.ListState
	sinceRow      int
	md            markdown.Options
}

func newPRData(number int) *prData {
	d := &prData{number: number, fileRow: -1, sinceRow: -1, viewed: map[string]bool{}}
	d.fileList.Selected = &d.fileRow
	d.sinceList.Selected = &d.sinceRow
	return d
}

type detailResult struct {
	pr       github.PullRequest
	files    []github.PRFile
	checks   []checkRow
	timeline []TimelineEvent
	reviews  []github.Review
	threads  []github.Thread
	commits  []github.Commit
	viewed   map[string]bool
	ov       Overview
	hasOv    bool
	since    []github.PRFile
	sinceSHA string
	partErrs []string
	err      error
}

func fetchDetail(ctx context.Context, be Backend, repo github.Remote, number int) detailResult {
	pr, err := be.Get(ctx, repo, number)
	if err != nil {
		return detailResult{err: err}
	}
	res := detailResult{pr: pr}
	var wg sync.WaitGroup
	var mu sync.Mutex
	var rest []github.Check
	var runs []CheckRun
	run := func(name string, fn func() error) {
		wg.Add(1)
		applog.Go(func() {
			defer wg.Done()
			if err := fn(); err != nil {
				mu.Lock()
				res.partErrs = append(res.partErrs, fmt.Sprintf(checkNameFmt, name, ghshared.ErrorText(err)))
				mu.Unlock()
			}
		})
	}
	run("files", func() (err error) { res.files, err = be.Files(ctx, repo, number); return })
	run("checks", func() (err error) { rest, err = be.Checks(ctx, repo, pr.HeadSHA); return })
	run("timeline", func() (err error) { res.timeline, err = be.Timeline(ctx, repo, number); return })
	run("reviews", func() (err error) { res.reviews, err = be.Reviews(ctx, repo, number); return })
	run("review threads", func() (err error) { res.threads, err = be.ReviewThreads(ctx, repo, number); return })
	run("commits", func() (err error) { res.commits, err = be.Commits(ctx, repo, number); return })
	run("viewed files", func() (err error) { res.viewed, err = be.ViewedFiles(ctx, repo, number); return })
	run("overview", func() (err error) {
		res.ov, err = be.Overview(ctx, repo, number, pr.BaseRef)
		res.hasOv = err == nil
		if err == nil {
			runs = res.ov.Checks
		}
		return
	})
	wg.Wait()
	res.checks = enrichChecks(rest, runs)
	if sha := lastReviewedSHA(res.reviews, res.ov.Viewer, pr.HeadSHA); sha != "" {
		cmp, err := be.Compare(ctx, repo, sha, pr.HeadSHA)
		if err != nil {
			res.partErrs = append(res.partErrs, fmt.Sprintf(checkNameFmt, "changes since last review", ghshared.ErrorText(err)))
		} else {
			res.since, res.sinceSHA = cmp.Files, sha
		}
	}
	slices.Sort(res.partErrs)
	slices.SortStableFunc(res.timeline, func(a, b TimelineEvent) int { return a.Time.Compare(b.Time) })
	return res
}

func lastReviewedSHA(reviews []github.Review, viewer, head string) string {
	if viewer == "" {
		return ""
	}
	var latest github.Review
	for _, r := range reviews {
		if r.Author == viewer && r.State != reviewPending && r.CommitID != "" && r.Submitted.After(latest.Submitted) {
			latest = r
		}
	}
	if latest.CommitID == head {
		return ""
	}
	return latest.CommitID
}

func (p *PRs) loadDetail(d *prData) {
	d.loading, d.stale = true, false
	be, repo, number := p.be, p.repo, d.number
	ghshared.Latest(&p.Session, &d.gen, func(ctx context.Context) detailResult {
		return fetchDetail(ctx, be, repo, number)
	}, func(res detailResult) { p.applyDetail(d, res) })
}

func (p *PRs) applyDetail(d *prData, res detailResult) {
	d.loading = false
	d.loadedAt = p.env.Now()
	if res.err != nil {
		d.err = res.err
		p.AuthFailure(res.err)
		return
	}
	d.err, d.partErrs = nil, res.partErrs
	d.loaded = true
	d.pr = res.pr
	d.files, d.checks, d.timeline, d.reviews, d.threads, d.commits = res.files, res.checks, res.timeline, res.reviews, res.threads, res.commits
	d.ov, d.hasOv, d.since, d.sinceSHA = res.ov, res.hasOv, res.since, res.sinceSHA
	if res.viewed != nil {
		d.viewed = res.viewed
	}
	if d.pr.NodeID == "" {
		d.pr.NodeID = res.ov.ID
	}
	if res.ov.Viewer != "" {
		p.viewer = res.ov.Viewer
	}
	d.pendingReview = pendingReviewID(res.reviews)
	d.baseSHA = ""
	var plain []github.Check
	for _, c := range res.checks {
		plain = append(plain, c.Check)
	}
	p.checks[d.number] = summarizeChecks(plain)
	if p.current.pr != nil && p.current.pr.Number == d.number {
		pr := res.pr
		p.current.pr = &pr
	}
	if d.openedTitle == "" && d.pr.Title != "" {
		d.openedTitle = d.pr.Title
		p.Host.OpenView(detailID(p.repo, d.number), detailTitle(d.number, d.pr.Title), func(c *ui.Context) { p.detailView(c, d) })
	}
}

func (p *PRs) mutate(d *prData, status, failure string, work func(ctx context.Context, be Backend) error) {
	be := p.be
	p.Act(&d.busy, status, failure, func(ctx context.Context) (func(), error) {
		if err := work(ctx, be); err != nil {
			return nil, err
		}
		return func() { p.loadDetail(d) }, nil
	})
}

func (p *PRs) postComment(d *prData) {
	be, repo, number := p.be, p.repo, d.number
	p.PostComment(&d.busy, d.draft, func(ctx context.Context, body string) (github.Comment, error) {
		return be.AddComment(ctx, repo, number, body)
	}, func(comment github.Comment) {
		d.timeline = append(d.timeline, TimelineEvent{Kind: "IssueComment", ID: comment.ID, DatabaseID: comment.DatabaseID, Actor: comment.Author, Time: comment.Created, Body: comment.Body, URL: comment.URL, CanEdit: true, CanDelete: true})
		d.draft = ""
	})
}

func (p *PRs) markReady(d *prData) {
	repo, number := p.repo, d.number
	p.mutate(d, "Marking ready", "Cannot mark pull request ready", func(ctx context.Context, be Backend) error {
		return be.MarkReady(ctx, repo, number)
	})
}

func (p *PRs) convertToDraft(d *prData) {
	id := d.pr.NodeID
	p.mutate(d, "Converting to draft", "Cannot convert to draft", func(ctx context.Context, be Backend) error {
		return be.ConvertToDraft(ctx, id)
	})
}

func (p *PRs) setState(d *prData, state string) {
	repo, number := p.repo, d.number
	label, failure := "Closing", "Cannot close pull request"
	if state == github.StateOpen {
		label, failure = "Reopening", "Cannot reopen pull request"
	}
	p.mutate(d, label, failure, func(ctx context.Context, be Backend) error {
		_, err := be.EditIssue(ctx, repo, number, github.IssueEdit{State: &state})
		return err
	})
}

func (p *PRs) saveTitle(d *prData, title string) {
	title = trimmed(title)
	if title == "" || title == d.pr.Title {
		d.editingTitle = false
		return
	}
	repo, number := p.repo, d.number
	d.editingTitle = false
	p.mutate(d, "Saving title", "Cannot change the title", func(ctx context.Context, be Backend) error {
		_, err := be.EditIssue(ctx, repo, number, github.IssueEdit{Title: &title})
		return err
	})
}

func (p *PRs) openBodyEditor(d *prData) {
	repo, number := p.repo, d.number
	p.openDialog(dialog{
		title:       fmt.Sprintf("Edit Description of #%d", number),
		placeholder: "Describe the changes (Markdown is supported)",
		action:      "Save",
		body:        d.pr.Body,
		submit: func(ctx context.Context, in dialogInput) (func(), error) {
			body := in.body
			if _, err := p.be.EditIssue(ctx, repo, number, github.IssueEdit{Body: &body}); err != nil {
				return nil, err
			}
			return func() { p.loadDetail(d) }, nil
		},
	})
}

func (p *PRs) revert(d *prData) {
	id := d.pr.NodeID
	number := d.number
	p.confirmThen(fmt.Sprintf("Revert #%d?", number), "GitHub opens a new pull request that reverts the changes.", "Revert", func() {
		be := p.be
		p.Act(&d.busy, "Reverting", "Cannot revert the pull request", func(ctx context.Context) (func(), error) {
			n, err := be.Revert(ctx, id)
			if err != nil {
				return nil, err
			}
			return func() { p.openPR(n, nil) }, nil
		})
	})
}

func (p *PRs) discardReview(d *prData) {
	id := d.pendingReview
	if id == "" {
		return
	}
	p.confirmThen("Discard your pending review?", "Its draft comments are deleted.", "Discard", func() {
		p.mutate(d, "Discarding review", "Cannot discard the review", func(ctx context.Context, be Backend) error { return be.DeleteReview(ctx, id) })
	})
}

func (p *PRs) setViewed(d *prData, path string, viewed bool) {
	d.viewed[path] = viewed
	id, be := d.pr.NodeID, p.be
	p.Act(&d.busy, "Saving viewed state", "Cannot save the viewed state", func(ctx context.Context) (func(), error) {
		if err := be.SetViewed(ctx, id, path, viewed); err != nil {
			return func() { d.viewed[path] = !viewed }, err
		}
		return nil, nil
	})
}

func (p *PRs) react(d *prData, e TimelineEvent, content string) {
	on := true
	for _, r := range e.Reactions {
		if r.Content == content && r.Viewer {
			on = false
		}
	}
	id := e.ID
	p.mutate(d, "Saving reaction", "Cannot save the reaction", func(ctx context.Context, be Backend) error { return be.React(ctx, id, content, on) })
}

func (p *PRs) editComment(d *prData, e TimelineEvent) {
	kind := commentIssue
	p.openDialog(dialog{
		title:       "Edit Comment",
		placeholder: "Comment",
		action:      "Save",
		body:        e.Body,
		validate:    requireBody("The comment cannot be empty."),
		submit: func(ctx context.Context, in dialogInput) (func(), error) {
			if err := p.be.EditComment(ctx, kind, e.ID, in.body); err != nil {
				return nil, err
			}
			return func() {
				for i := range d.timeline {
					if d.timeline[i].ID == e.ID {
						d.timeline[i].Body = in.body
					}
				}
			}, nil
		},
	})
}

func (p *PRs) deleteComment(d *prData, e TimelineEvent) {
	p.confirmThen("Delete this comment?", "This cannot be undone.", "Delete", func() {
		id := e.ID
		be := p.be
		p.Act(&d.busy, "Deleting comment", "Cannot delete the comment", func(ctx context.Context) (func(), error) {
			if err := be.DeleteComment(ctx, commentIssue, id); err != nil {
				return nil, err
			}
			return func() {
				d.timeline = slices.DeleteFunc(d.timeline, func(x TimelineEvent) bool { return x.ID == id })
			}, nil
		})
	})
}

func (p *PRs) detailTick(c *ui.Context, d *prData) {
	if d.loading {
		return
	}
	now := p.env.Now()
	if d.stale || d.loaded && now.Sub(d.loadedAt) >= autoRefreshInterval {
		p.loadDetail(d)
		return
	}
	if !d.loaded {
		return
	}
	due := d.loadedAt.Add(autoRefreshInterval)
	if !d.tickAt.Equal(due) {
		d.tickAt = due
		c.After(due.Sub(now))
	}
}

func (d *prData) isOpen() bool {
	return d.pr.State == github.StateOpen && !d.pr.Merged
}

func (d *prData) commentCount(path string) int {
	n := 0
	for _, t := range d.threads {
		if t.Path == path {
			n += len(t.Comments)
		}
	}
	return n
}

func (d *prData) visibleFiles() []github.PRFile {
	if !d.hideViewed {
		return d.files
	}
	var out []github.PRFile
	for _, f := range d.files {
		if !d.viewed[f.Path] {
			out = append(out, f)
		}
	}
	return out
}

func (d *prData) viewedCount() int {
	n := 0
	for _, f := range d.files {
		if d.viewed[f.Path] {
			n++
		}
	}
	return n
}
