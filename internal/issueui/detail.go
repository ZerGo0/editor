package issueui

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/ZerGo0/editor/internal/github"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/egoist/mygo/ui/markdown/parser"
)

type issueData struct {
	number      int
	issue       github.Issue
	comments    []github.Comment
	loaded      bool
	loading     bool
	stale       bool
	gen         int
	err         error
	partErr     string
	busy        string
	draft       string
	editing     bool
	editTitle   string
	editBody    string
	editErr     string
	openedTitle string
	md          markdown.Options
}

func (i *Issues) dataFor(number int) *issueData {
	d := i.details[number]
	if d == nil {
		d = &issueData{number: number}
		d.md.Resolve = i.resolveReference
		d.md.OnLink = i.openLink
		i.details[number] = d
	}
	return d
}

func (i *Issues) resolveReference(in parser.Inline) (string, bool) {
	switch in.Kind {
	case parser.Mention:
		login := in.Login()
		if login == "" || strings.Contains(login, "/") {
			return "", false
		}
		return userURL(i.repo, login), true
	case parser.Reference:
		repo, number := in.Ref()
		if number <= 0 {
			return "", false
		}
		if repo == "" {
			return issueURL(i.repo, number), true
		}
		owner, name, ok := strings.Cut(repo, "/")
		if !ok || owner == "" || name == "" {
			return "", false
		}
		return issueURL(github.Remote{Host: i.repo.Host, Owner: owner, Name: name}, number), true
	}
	return "", false
}

func safeURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != ""
}

func (i *Issues) openLink(raw string) {
	if !safeURL(raw) {
		return
	}
	if err := i.env.OpenURL(raw); err != nil {
		i.h.ShowError("Cannot open the link", err)
	}
}

func openExternal(c *ui.Context, raw string) {
	if safeURL(raw) {
		c.OpenURL(raw)
	}
}

type detailResult struct {
	issue    github.Issue
	comments []github.Comment
	partErr  string
	err      error
}

func fetchDetail(ctx context.Context, be Backend, repo github.Remote, number int) detailResult {
	var res detailResult
	var commentErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		res.comments, commentErr = be.IssueComments(ctx, repo, number)
	}()
	res.issue, res.err = be.GetIssue(ctx, repo, number)
	wg.Wait()
	if res.err == nil && commentErr != nil {
		res.partErr = "comments: " + errorText(commentErr)
	}
	return res
}

func (i *Issues) loadDetail(d *issueData) {
	d.gen++
	gen := d.gen
	d.loading, d.stale = true, false
	be, repo, number := i.be, i.repo, d.number
	go func() {
		res := fetchDetail(i.ctx, be, repo, number)
		i.post(func() {
			if gen == d.gen {
				i.applyDetail(d, res)
			}
		})
	}()
}

func (i *Issues) applyDetail(d *issueData, res detailResult) {
	d.loading = false
	if res.err != nil {
		d.err = res.err
		i.authFailure(res.err)
		return
	}
	d.err, d.partErr = nil, res.partErr
	d.loaded = true
	d.issue = res.issue
	if res.partErr == "" {
		d.comments = res.comments
	}
	i.retitle(d)
}

func (i *Issues) retitle(d *issueData) {
	if d.openedTitle == d.issue.Title {
		return
	}
	d.openedTitle = d.issue.Title
	i.h.OpenView(detailID(i.repo, d.number), detailTitle(d.number, d.issue.Title), func(c *ui.Context) { i.detailView(c, d) })
}

func (i *Issues) openIssue(number int, preload *github.Issue) {
	if !i.requireReady() {
		return
	}
	d := i.dataFor(number)
	if preload != nil && !d.loaded {
		d.issue = *preload
	}
	if !d.loaded && !d.loading {
		i.loadDetail(d)
	}
	d.openedTitle = d.issue.Title
	i.h.OpenView(detailID(i.repo, number), detailTitle(number, d.issue.Title), func(c *ui.Context) { i.detailView(c, d) })
}

func (i *Issues) postComment(d *issueData) {
	body := trimmed(d.draft)
	if body == "" {
		return
	}
	be, repo, number := i.be, i.repo, d.number
	i.act(&d.busy, "Posting comment", "Cannot post comment", func(ctx context.Context) (func(), error) {
		comment, err := be.AddIssueComment(ctx, repo, number, body)
		if err != nil {
			return nil, err
		}
		return func() {
			d.comments = append(d.comments, comment)
			d.issue.Comments++
			d.draft = ""
		}, nil
	})
}

func (i *Issues) editIssue(d *issueData, edit github.IssueEdit, status, failure string, done func()) {
	be, repo, number := i.be, i.repo, d.number
	i.act(&d.busy, status, failure, func(ctx context.Context) (func(), error) {
		updated, err := be.EditIssue(ctx, repo, number, edit)
		if err != nil {
			return nil, err
		}
		return func() {
			updated.LinkedPRs = d.issue.LinkedPRs
			d.issue = updated
			i.markListsStale()
			i.retitle(d)
			if done != nil {
				done()
			}
		}, nil
	})
}

func (i *Issues) markListsStale() {
	for _, s := range i.side.all() {
		s.stale = true
	}
}

func (i *Issues) toggleState(d *issueData) {
	next, notice := github.StateClosed, "Closed"
	if !isOpen(d.issue) {
		next, notice = github.StateOpen, "Reopened"
	}
	number := d.number
	i.editIssue(d, github.IssueEdit{State: &next}, notice+" issue", "Cannot change the issue state", func() {
		i.h.Notify(fmt.Sprintf("%s #%d", notice, number))
	})
}

func (i *Issues) startEdit(d *issueData) {
	d.editing, d.editErr = true, ""
	d.editTitle, d.editBody = d.issue.Title, d.issue.Body
}

func (i *Issues) saveEdit(d *issueData) {
	title := trimmed(d.editTitle)
	if title == "" {
		d.editErr = "Enter a title."
		return
	}
	var edit github.IssueEdit
	if title != d.issue.Title {
		edit.Title = &title
	}
	if d.editBody != d.issue.Body {
		body := d.editBody
		edit.Body = &body
	}
	if edit.Title == nil && edit.Body == nil {
		d.editing = false
		return
	}
	i.editIssue(d, edit, "Saving issue", "Cannot save the issue", func() { d.editing = false })
}
