package issueui

import (
	"strconv"
	"strings"

	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/badge"
	"github.com/ZerGo0/editor/internal/design/detail"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/field"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/github"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
)

const (
	commentLines  = 6
	editBodyLines = 8
)

func (i *Issues) detailView(c *ui.Context, d *issueData) {
	if i.closed {
		return
	}
	detail.Page(c, func() {
		switch {
		case d.err != nil && !d.loaded && d.issue.Number == 0:
			detail.Centered(c, func() {
				feedback.Error(c, "Cannot load #"+strconv.Itoa(d.number), d.err, func() { i.loadDetail(d) })
			})
		case d.issue.Number == 0:
			detail.Centered(c, func() { feedback.Loading(c, "Loading #"+strconv.Itoa(d.number)+"...") })
		default:
			if d.stale && !d.loading {
				i.loadDetail(d)
			}
			i.detailHeader(c, d)
			detail.Body(c, "Issue", token.PageMaxWidth, func() { i.detailBody(c, d) })
		}
	})
}

func (i *Issues) detailHeader(c *ui.Context, d *issueData) {
	issue := d.issue
	o := detail.HeaderOptions{Title: issue.Title, Number: issue.Number, Busy: d.busy != "" || d.loading}
	if d.editing {
		o.Editor = func(c *ui.Context) {
			field.TextInput(c, "Title", &d.editTitle, field.InputOptions{AutoFocus: true, ID: "edit-title"})
		}
	}
	o.Actions = func(c *ui.Context) {
		if d.loading {
			feedback.Spinner(c, "Refreshing")
		}
		if action.IconButton(c, icons.Refresh, "Refresh", action.IconOptions{ID: "refresh"}).Clicked() {
			i.loadDetail(d)
		}
	}
	o.Meta = func(c *ui.Context) { i.detailMeta(c, d) }
	o.Toolbar = func(c *ui.Context) { i.detailActions(c, d) }
	detail.Header(c, o)
}

func (i *Issues) detailMeta(c *ui.Context, d *issueData) {
	issue := d.issue
	stateTag(c, issue)
	author := issue.Author
	if author == "" {
		author = "unknown"
	}
	badge.Avatar(c, author, token.AvatarSize)
	mutedText(c, author+" opened this "+relativeTime(i.env.Now(), issue.Created)).Shrink(0)
}

func (i *Issues) detailActions(c *ui.Context, d *issueData) {
	issue := d.issue
	idle := d.busy == ""
	if d.editing {
		primaryButton(c, "Save", !idle).OnClick(func() { i.saveEdit(d) })
		smallButton(c, "Cancel", !idle).OnClick(func() { d.editing = false })
		if d.editErr != "" {
			feedback.Notice(c, feedback.NoticeOptions{Tone: token.ToneError, Text: d.editErr})
		}
	} else {
		smallButton(c, "Edit", !idle).OnClick(func() { i.startEdit(d) })
		label := "Close Issue"
		if !isOpen(issue) {
			label = "Reopen Issue"
		}
		smallButton(c, label, !idle).OnClick(func() { i.toggleState(d) })
	}
	i.workingButton(c, d)
	smallButton(c, "Open on GitHub", issue.URL == "").OnClick(func() { openExternal(c, issue.URL) })
	if !idle {
		feedback.Spinner(c, d.busy)
		mutedText(c, d.busy+"...")
	}
}

func (i *Issues) workingButton(c *ui.Context, d *issueData) {
	idle := i.working.busy == ""
	active := i.work.Active
	switch {
	case active != nil && active.Number == d.number:
		smallButton(c, "Stop Working", !idle).OnClick(i.stopWorking)
	default:
		label := "Start Working"
		if item, ok := i.work.previous(d.number); ok {
			label = "Continue Working"
			smallButton(c, label, !idle).OnClick(func() { i.continueWorking(item) })
			return
		}
		primaryButton(c, label, !idle).OnClick(func() {
			issue := d.issue
			i.startWorking(d.number, &issue)
		})
	}
}

func (i *Issues) detailBody(c *ui.Context, d *issueData) {
	if d.partErr != "" {
		feedback.Notice(c, feedback.NoticeOptions{Tone: token.ToneWarning, Text: "Some details could not be loaded: " + d.partErr, Action: retryLabel, OnAction: func() { i.loadDetail(d) }})
	}
	i.peopleCard(c, d)
	detail.Card(c, "Description", detail.CardOptions{}, func() {
		switch {
		case d.editing:
			field.TextArea(c, "Description", &d.editBody, field.AreaOptions{Placeholder: "Describe the issue (Markdown is supported)", MinLines: editBodyLines, ID: "edit-body"})
		case strings.TrimSpace(d.issue.Body) == "":
			mutedText(c, "No description provided.")
		default:
			markdown.View(c.Key("description"), d.issue.Body, &d.md)
		}
	})
	i.linkedCard(c, d)
	i.timelineCard(c, d)
	i.commentBox(c, d)
}

func (i *Issues) peopleCard(c *ui.Context, d *issueData) {
	issue := d.issue
	detail.Card(c, "Details", detail.CardOptions{}, func() {
		detail.Property(c, "Assignees", func(c *ui.Context) {
			if len(issue.Assignees) == 0 {
				mutedText(c, "Nobody")
			}
			for _, name := range issue.Assignees {
				badge.Avatar(c, name, token.AvatarSize)
				mutedTag(c, name)
			}
		}, nil)
		detail.Property(c, "Labels", func(c *ui.Context) {
			if len(issue.Labels) == 0 {
				mutedText(c, "None")
			}
			for _, l := range issue.Labels {
				labelTag(c, l)
			}
		}, nil)
		detail.Property(c, "Milestone", func(c *ui.Context) {
			if issue.Milestone == nil {
				mutedText(c, "None")
				return
			}
			mutedTag(c, issue.Milestone.Title)
		}, nil)
	})
}

func (i *Issues) linkedCard(c *ui.Context, d *issueData) {
	if len(d.issue.LinkedPRs) == 0 {
		return
	}
	detail.Card(c, "Linked pull requests", detail.CardOptions{Count: len(d.issue.LinkedPRs)}, func() {
		for n, pr := range d.issue.LinkedPRs {
			ui.Row(c.Key(n)).FillWidth().AlignItems(ui.Center).Gap(token.GapControl.Px()).Children(func() {
				ui.Icon(c, icons.PullRequest).Size(token.IconSize.Px(), token.IconSize.Px()).TextColor(token.Of(c).Icon).Shrink(0).AccessHidden(true)
				ui.Text(c, "#"+strconv.Itoa(pr.Number)+" "+pr.Title).FontSize(token.TextBody.Px()).TextColor(token.Of(c).Text.Primary).SingleLine().Grow(1).MinWidth(0)
				mutedTag(c, pr.State)
				ghostButton(c, "Open", "open-pr-"+strconv.Itoa(pr.Number)).Label("Open pull request #" + strconv.Itoa(pr.Number)).Disabled(pr.URL == "").OnClick(func() { openExternal(c, pr.URL) })
			})
		}
	})
}

func (i *Issues) timelineCard(c *ui.Context, d *issueData) {
	detail.Card(c, "Conversation", detail.CardOptions{Count: len(d.comments)}, func() {
		if len(d.comments) == 0 {
			mutedText(c, "No comments yet.")
			return
		}
		now := i.env.Now()
		for n, cm := range d.comments {
			detail.Comment(c.Key(n), detail.CommentOptions{Author: commentAuthor(cm), Verb: "commented", Time: relativeTime(now, cm.Created)}, func() {
				if strings.TrimSpace(cm.Body) != "" {
					markdown.View(c.Key("body"), cm.Body, &d.md)
				}
			})
		}
	})
}

func commentAuthor(cm github.Comment) string {
	if cm.Author == "" {
		return "unknown"
	}
	return cm.Author
}

func (i *Issues) commentBox(c *ui.Context, d *issueData) {
	detail.Card(c, "Add a comment", detail.CardOptions{}, func() {
		field.TextArea(c, "Comment", &d.draft, field.AreaOptions{Placeholder: "Leave a comment (Markdown is supported)", MinLines: commentLines, ID: "comment"})
		blank := trimmed(d.draft) == ""
		ui.Row(c).Justify(ui.End).FillWidth().Children(func() {
			action.Button(c, "Add Comment", action.ButtonOptions{Variant: action.Primary, Disabled: blank || d.busy != ""}).OnClick(func() { i.postComment(d) })
		})
	})
}
