package prui

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/ZerGo0/editor/internal/applog"
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

type mergeChoice struct {
	label  string
	method string
}

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

func (d *prData) allowedMerges() []mergeChoice {
	if !d.hasOv {
		return mergeChoices
	}
	var out []mergeChoice
	for _, m := range mergeChoices {
		if m.method == github.MergeMethodMerge && d.ov.AllowMerge || m.method == github.MergeMethodSquash && d.ov.AllowSquash || m.method == github.MergeMethodRebase && d.ov.AllowRebase {
			out = append(out, m)
		}
	}
	return out
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
				res.partErrs = append(res.partErrs, fmt.Sprintf(checkNameFmt, name, errorText(err)))
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
			res.partErrs = append(res.partErrs, fmt.Sprintf(checkNameFmt, "changes since last review", errorText(err)))
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
	d.gen++
	gen := d.gen
	d.loading, d.stale = true, false
	be, repo, number := p.be, p.repo, d.number
	applog.Go(func() {
		res := fetchDetail(p.ctx, be, repo, number)
		p.post(func() {
			if gen == d.gen {
				p.applyDetail(d, res)
			}
		})
	})
}

func (p *PRs) applyDetail(d *prData, res detailResult) {
	d.loading = false
	d.loadedAt = p.env.Now()
	if res.err != nil {
		d.err = res.err
		p.authFailure(res.err)
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
		p.h.OpenView(detailID(p.repo, d.number), detailTitle(d.number, d.pr.Title), func(c *ui.Context) { p.detailView(c, d) })
	}
}

func (p *PRs) mutate(d *prData, status, failure string, work func(ctx context.Context, be Backend) error) {
	be := p.be
	p.act(&d.busy, status, failure, func(ctx context.Context) (func(), error) {
		if err := work(ctx, be); err != nil {
			return nil, err
		}
		return func() { p.loadDetail(d) }, nil
	})
}

func (p *PRs) postComment(d *prData) {
	body := trimmed(d.draft)
	if body == "" {
		return
	}
	be, repo, number := p.be, p.repo, d.number
	p.act(&d.busy, "Posting comment", "Cannot post comment", func(ctx context.Context) (func(), error) {
		comment, err := be.AddComment(ctx, repo, number, body)
		if err != nil {
			return nil, err
		}
		return func() {
			d.timeline = append(d.timeline, TimelineEvent{Kind: "IssueComment", ID: comment.ID, DatabaseID: comment.DatabaseID, Actor: comment.Author, Time: comment.Created, Body: comment.Body, URL: comment.URL, CanEdit: true, CanDelete: true})
			d.draft = ""
		}, nil
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

func (p *PRs) updateBranch(d *prData) {
	repo, number, sha := p.repo, d.number, d.pr.HeadSHA
	p.mutate(d, "Updating branch", "Cannot update the branch", func(ctx context.Context, be Backend) error {
		return be.UpdateBranch(ctx, repo, number, sha)
	})
}

func (p *PRs) enqueue(d *prData) {
	id := d.pr.NodeID
	p.confirmThen(fmt.Sprintf("Add #%d to the merge queue?", d.number), "The pull request merges when it reaches the front of the queue.", "Add to Queue", func() {
		p.mutate(d, "Adding to the queue", "Cannot add to the merge queue", func(ctx context.Context, be Backend) error { return be.Enqueue(ctx, id) })
	})
}

func (p *PRs) dequeue(d *prData) {
	entry := d.ov.Queue.EntryID
	p.mutate(d, "Leaving the queue", "Cannot remove from the merge queue", func(ctx context.Context, be Backend) error { return be.Dequeue(ctx, entry) })
}

func (p *PRs) disableAutoMerge(d *prData) {
	id := d.pr.NodeID
	p.mutate(d, "Disabling auto-merge", "Cannot disable auto-merge", func(ctx context.Context, be Backend) error { return be.SetAutoMerge(ctx, id, "", false) })
}

func (p *PRs) openAutoMergeDialog(d *prData) {
	id := d.pr.NodeID
	methods := d.allowedMerges()
	if len(methods) == 0 {
		methods = mergeChoices
	}
	labels := make([]string, len(methods))
	for i, m := range methods {
		labels[i] = m.label
	}
	p.openDialog(dialog{
		title:   fmt.Sprintf("Enable Auto-Merge for #%d", d.number),
		hint:    "The pull request merges when all requirements are met.",
		action:  "Enable Auto-Merge",
		noBody:  true,
		choices: labels,
		choice:  defaultMergeIndex(methods, d.ov.DefaultMethod),
		submit: func(ctx context.Context, in dialogInput) (func(), error) {
			if err := p.be.SetAutoMerge(ctx, id, methods[in.choice].method, true); err != nil {
				return nil, err
			}
			return func() { p.loadDetail(d) }, nil
		},
	})
}

func defaultMergeIndex(methods []mergeChoice, def string) int {
	for i, m := range methods {
		if m.method == def {
			return i
		}
	}
	return 0
}

func (p *PRs) confirmThen(message, detail, action string, then func()) {
	confirm, win := p.env.Confirm, p.h.Window()
	applog.Go(func() {
		if confirm(win, ConfirmOptions{Message: message, Detail: detail, Action: action}) {
			p.post(then)
		}
	})
}

func (p *PRs) openMergeDialog(d *prData) {
	methods := d.allowedMerges()
	if len(methods) == 0 {
		p.h.Notify("This repository allows no merge method.")
		return
	}
	labels := make([]string, len(methods))
	for i, m := range methods {
		labels[i] = m.label
	}
	repo, number, base := p.repo, d.number, d.pr.BaseRef
	headRepo := github.Remote{Host: repo.Host, Owner: d.pr.HeadOwner, Name: d.pr.HeadRepo}
	headRef := d.pr.HeadRef
	canDelete := d.pr.HeadRepo != "" && d.pr.HeadOwner != ""
	var checks []dialogCheck
	if canDelete {
		checks = []dialogCheck{{label: deleteBranchLabel, on: d.ov.DeleteBranchOnMerge}}
	}
	confirm, win := p.env.Confirm, p.h.Window()
	p.openDialog(dialog{
		title:        fmt.Sprintf("Merge #%d into %s", number, base),
		hint:         "Edit the commit message, or leave the fields empty for GitHub's default.",
		placeholder:  "Commit message (optional)",
		action:       confirmMergeAction,
		subjectLabel: "Commit title (optional)",
		subject:      fmt.Sprintf(mergeSubjectFmt, d.pr.Title, number),
		choices:      labels,
		choice:       defaultMergeIndex(methods, d.ov.DefaultMethod),
		checks:       checks,
		submit: func(ctx context.Context, in dialogInput) (func(), error) {
			choice := methods[in.choice]
			ok := confirm(win, ConfirmOptions{
				Message: fmt.Sprintf("Merge #%d into %s?", number, base),
				Detail:  choice.label + ". This cannot be undone from the editor.",
				Action:  confirmMergeAction,
			})
			if !ok {
				return nil, errCancelled
			}
			if err := p.be.Merge(ctx, repo, number, choice.method, in.subject, in.body); err != nil {
				return nil, err
			}
			var warning error
			if len(in.checks) > 0 && in.checks[0] {
				warning = p.be.DeleteBranch(ctx, headRepo, headRef)
			}
			return func() {
				p.h.Notify(fmt.Sprintf("Merged #%d", number))
				if warning != nil {
					p.h.ShowError("Merged, but the branch was not deleted", warning)
				}
				p.loadDetail(d)
				p.refresh()
			}, nil
		},
	})
}

func (p *PRs) deleteBranch(d *prData) {
	headRepo := github.Remote{Host: p.repo.Host, Owner: d.pr.HeadOwner, Name: d.pr.HeadRepo}
	ref := d.pr.HeadRef
	if headRepo.Owner == "" || headRepo.Name == "" {
		p.h.Notify(headRepoMissingNote)
		return
	}
	p.confirmThen(fmt.Sprintf("Delete the branch %s?", ref), "The branch is removed from "+headRepo.Owner+"/"+headRepo.Name+".", "Delete Branch", func() {
		p.mutate(d, "Deleting branch", "Cannot delete the branch", func(ctx context.Context, be Backend) error { return be.DeleteBranch(ctx, headRepo, ref) })
	})
}

func (p *PRs) revert(d *prData) {
	id := d.pr.NodeID
	number := d.number
	p.confirmThen(fmt.Sprintf("Revert #%d?", number), "GitHub opens a new pull request that reverts the changes.", "Revert", func() {
		be := p.be
		p.act(&d.busy, "Reverting", "Cannot revert the pull request", func(ctx context.Context) (func(), error) {
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
	p.act(&d.busy, "Saving viewed state", "Cannot save the viewed state", func(ctx context.Context) (func(), error) {
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
		p.act(&d.busy, "Deleting comment", "Cannot delete the comment", func(ctx context.Context) (func(), error) {
			if err := be.DeleteComment(ctx, commentIssue, id); err != nil {
				return nil, err
			}
			return func() {
				d.timeline = slices.DeleteFunc(d.timeline, func(x TimelineEvent) bool { return x.ID == id })
			}, nil
		})
	})
}

func (p *PRs) openLabelPicker(d *prData) {
	repo, number := p.repo, d.number
	p.openPicker("Labels", "Apply", false, d.pr.Labels, func(ctx context.Context) ([]string, error) {
		labels, err := p.be.Labels(ctx, repo)
		names := make([]string, 0, len(labels))
		for _, l := range labels {
			names = append(names, l.Name)
		}
		return names, err
	}, func(ctx context.Context, selected []string) (func(), error) {
		if selected == nil {
			selected = []string{}
		}
		if _, err := p.be.EditIssue(ctx, repo, number, github.IssueEdit{Labels: &selected}); err != nil {
			return nil, err
		}
		return func() { p.loadDetail(d) }, nil
	})
}

func (p *PRs) openAssigneePicker(d *prData) {
	repo, number := p.repo, d.number
	p.openPicker("Assignees", "Apply", false, d.ov.Assignees, p.loadUsers, func(ctx context.Context, selected []string) (func(), error) {
		if selected == nil {
			selected = []string{}
		}
		if _, err := p.be.EditIssue(ctx, repo, number, github.IssueEdit{Assignees: &selected}); err != nil {
			return nil, err
		}
		return func() { p.loadDetail(d) }, nil
	})
}

func (p *PRs) openReviewerPicker(d *prData) {
	repo, number := p.repo, d.number
	p.openPicker("Request Review", "Request", false, nil, p.loadUsers, func(ctx context.Context, selected []string) (func(), error) {
		if len(selected) == 0 {
			return nil, errors.New("choose at least one person")
		}
		if err := p.be.RequestReviewers(ctx, repo, number, selected, nil); err != nil {
			return nil, err
		}
		return func() { p.loadDetail(d) }, nil
	})
}

func (p *PRs) openMilestonePicker(d *prData) {
	repo, number := p.repo, d.number
	var current []string
	if d.ov.Milestone != nil {
		current = []string{d.ov.Milestone.Title}
	}
	numbers := map[string]int{}
	p.openPicker("Milestone", "Apply", true, current, func(ctx context.Context) ([]string, error) {
		ms, err := p.be.Milestones(ctx, repo)
		titles := make([]string, 0, len(ms))
		for _, m := range ms {
			numbers[m.Title] = m.Number
			titles = append(titles, m.Title)
		}
		return titles, err
	}, func(ctx context.Context, selected []string) (func(), error) {
		none := 0
		milestone := &none
		if len(selected) > 0 {
			n, ok := numbers[selected[0]]
			if !ok {
				return nil, fmt.Errorf("unknown milestone %q", selected[0])
			}
			milestone = &n
		}
		if _, err := p.be.EditIssue(ctx, repo, number, github.IssueEdit{Milestone: milestone}); err != nil {
			return nil, err
		}
		return func() { p.loadDetail(d) }, nil
	})
}

func (p *PRs) loadUsers(ctx context.Context) ([]string, error) {
	var logins []string
	after := ""
	for range maxUserPages {
		page, err := p.be.AssignableUsers(ctx, p.repo, after)
		if err != nil {
			return logins, err
		}
		for _, u := range page.Users {
			logins = append(logins, u.Login)
		}
		if page.Next == "" {
			break
		}
		after = page.Next
	}
	return logins, nil
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

func (d *prData) mergeBlocked() string {
	switch {
	case !d.isOpen():
		return "This pull request is not open."
	case d.pr.Draft:
		return "Mark the pull request ready for review first."
	case d.pr.Mergeable != nil && !*d.pr.Mergeable:
		return "This pull request has merge conflicts."
	case d.hasOv && d.ov.MergeState == mergeStateDirty:
		return "This pull request has merge conflicts."
	}
	return ""
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

func mergeSummary(d *prData) string {
	switch {
	case d.pr.Merged:
		return "Merged"
	case d.pr.State == github.StateClosed:
		return "Closed"
	case !d.hasOv:
		return ""
	case d.ov.Queue.InQueue:
		return fmt.Sprintf("In the merge queue (position %d, %s)", d.ov.Queue.Position, strings.ToLower(d.ov.Queue.State))
	case d.ov.AutoMerge != nil:
		return fmt.Sprintf("Auto-merge enabled (%s) by %s", d.ov.AutoMerge.Method, d.ov.AutoMerge.By)
	case d.pr.Draft:
		return "Draft: mark it ready for review to merge"
	}
	switch d.ov.MergeState {
	case mergeStateDirty:
		return "This branch has conflicts with the base branch"
	case mergeStateBehind:
		return "This branch is out of date with the base branch"
	case mergeStateUnstable:
		return "Some checks are not passing, but the pull request can merge"
	case mergeStateBlocked:
		switch d.ov.ReviewDecision {
		case decisionChanges:
			return "Changes were requested"
		case decisionRequired:
			return "Review required before merging"
		}
		return "Merging is blocked by the repository rules"
	case mergeStateClean:
		return "Ready to merge"
	}
	return ""
}
