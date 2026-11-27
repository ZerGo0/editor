package prui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ZerGo0/editor/internal/design"
	"github.com/ZerGo0/editor/internal/design/badge"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/fsutil"
	"github.com/ZerGo0/editor/internal/github"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	modePR        = "pr"
	modeSince     = "since"
	modeLocal     = "local"
	noPRInTabHint = "This pull request is not open, so comments cannot be added."
)

type diffSpec struct {
	mode   string
	oldRef string
}

func (t *diffTab) selectedCode(r editor.DiffLineRange) string {
	if r.Side != editor.DiffNew || t.state == nil {
		return ""
	}
	lines := make([]string, 0, r.End-r.Start+1)
	for i := r.Start; i <= r.End && i < t.state.New.LineCount(); i++ {
		lines = append(lines, t.state.New.Line(i))
	}
	return strings.Join(lines, "\n")
}

func (p *PRs) ensureReview(ctx context.Context, d *prData, pending string) (string, error) {
	if pending != "" {
		return pending, nil
	}
	return p.be.StartReview(ctx, p.repo, d.number)
}

func (p *PRs) commentAt(t *diffTab, r editor.DiffLineRange) {
	d := t.data
	if !d.isOpen() {
		p.h.Notify(noPRInTabHint)
		return
	}
	if t.spec.mode != modePR {
		p.h.Notify("Open the file from the changed files of the pull request to comment on it.")
		return
	}
	pending := d.pendingReview
	opts := lineComment(r, t.file.Path, "")
	where := fmt.Sprintf("%s:%d", t.file.Path, opts.Line)
	p.openDialog(dialog{
		title:       "Comment on " + where,
		hint:        "The comment is added to your pending review. Submit the review from the pull request tab.",
		placeholder: "Leave a comment",
		action:      "Add Review Comment",
		suggest:     t.selectedCode(r),
		validate:    requireBody("Write a comment first."),
		submit: func(ctx context.Context, in dialogInput) (func(), error) {
			id, err := p.ensureReview(ctx, d, pending)
			if err != nil {
				return nil, err
			}
			o := opts
			o.Body = in.body
			thread, err := p.be.AddReviewComment(ctx, id, o)
			if err != nil {
				return nil, err
			}
			return func() {
				d.pendingReview = id
				d.threads = append(d.threads, thread)
			}, nil
		},
	})
}

func (p *PRs) commentOnFile(t *diffTab) {
	d := t.data
	if !d.isOpen() {
		p.h.Notify(noPRInTabHint)
		return
	}
	pending, filePath := d.pendingReview, t.file.Path
	p.openDialog(dialog{
		title:       "Comment on " + filePath,
		hint:        "The comment is added to your pending review and applies to the whole file.",
		placeholder: "Leave a comment",
		action:      "Add File Comment",
		validate:    requireBody("Write a comment first."),
		submit: func(ctx context.Context, in dialogInput) (func(), error) {
			id, err := p.ensureReview(ctx, d, pending)
			if err != nil {
				return nil, err
			}
			if err := p.be.AddFileThread(ctx, id, filePath, in.body); err != nil {
				return nil, err
			}
			return func() {
				d.pendingReview = id
				p.loadDetail(d)
			}, nil
		},
	})
}

func (p *PRs) replyTo(d *prData, th github.Thread) {
	last := ""
	if n := len(th.Comments); n > 0 {
		last = th.Comments[n-1].Author
	}
	var checks []dialogCheck
	if !th.Resolved {
		checks = []dialogCheck{{label: resolveAfterLabel}}
	}
	p.openDialog(dialog{
		title:       "Reply to " + last,
		placeholder: "Write a reply",
		action:      "Reply",
		checks:      checks,
		validate:    requireBody("Write a reply first."),
		submit: func(ctx context.Context, in dialogInput) (func(), error) {
			comment, err := p.be.Reply(ctx, th.ID, in.body)
			if err != nil {
				return nil, err
			}
			resolve := len(in.checks) > 0 && in.checks[0]
			var resolveErr error
			if resolve {
				resolveErr = p.be.ResolveThread(ctx, th.ID)
			}
			return func() {
				for i := range d.threads {
					if d.threads[i].ID == th.ID {
						d.threads[i].Comments = append(d.threads[i].Comments, comment)
						d.threads[i].Resolved = d.threads[i].Resolved || resolve && resolveErr == nil
					}
				}
				if resolveErr != nil {
					p.h.ShowError("The reply was sent, but the thread was not resolved", resolveErr)
				}
			}, nil
		},
	})
}

func (p *PRs) editThreadComment(d *prData, th github.Thread, cm github.Comment) {
	p.openDialog(dialog{
		title:       "Edit Comment",
		placeholder: "Comment",
		action:      "Save",
		body:        cm.Body,
		validate:    requireBody("The comment cannot be empty."),
		submit: func(ctx context.Context, in dialogInput) (func(), error) {
			if err := p.be.EditComment(ctx, commentReview, cm.ID, in.body); err != nil {
				return nil, err
			}
			return func() {
				for i := range d.threads {
					for j := range d.threads[i].Comments {
						if d.threads[i].Comments[j].ID == cm.ID {
							d.threads[i].Comments[j].Body = in.body
						}
					}
				}
			}, nil
		},
	})
}

func (p *PRs) deleteThreadComment(d *prData, cm github.Comment) {
	p.confirmThen("Delete this comment?", "This cannot be undone.", "Delete", func() {
		be := p.be
		p.act(&d.busy, "Deleting comment", "Cannot delete the comment", func(ctx context.Context) (func(), error) {
			if err := be.DeleteComment(ctx, commentReview, cm.ID); err != nil {
				return nil, err
			}
			return func() {
				for i := range d.threads {
					d.threads[i].Comments = slices.DeleteFunc(d.threads[i].Comments, func(x github.Comment) bool { return x.ID == cm.ID })
				}
				d.threads = slices.DeleteFunc(d.threads, func(t github.Thread) bool { return len(t.Comments) == 0 })
			}, nil
		})
	})
}

func (p *PRs) setResolved(d *prData, th github.Thread, resolved bool) {
	label, failure := "Resolving thread", "Cannot resolve thread"
	if !resolved {
		label, failure = "Reopening thread", "Cannot unresolve thread"
	}
	be := p.be
	p.act(&d.busy, label, failure, func(ctx context.Context) (func(), error) {
		var err error
		if resolved {
			err = be.ResolveThread(ctx, th.ID)
		} else {
			err = be.UnresolveThread(ctx, th.ID)
		}
		if err != nil {
			return nil, err
		}
		return func() {
			for i := range d.threads {
				if d.threads[i].ID == th.ID {
					d.threads[i].Resolved = resolved
				}
			}
		}, nil
	})
}

func (p *PRs) applySuggestion(d *prData, th github.Thread, code string) {
	root := p.h.RootDir()
	target := filepath.Join(root, filepath.FromSlash(th.Path))
	switch {
	case p.branch.OID == "" || p.branch.OID != d.pr.HeadSHA:
		p.h.Notify("Check out this pull request at its latest commit to apply the suggestion.")
		return
	case !fsutil.Within(root, target):
		p.h.Notify("The suggestion is outside the project folder.")
		return
	}
	if buf := p.h.Buffer(target); buf != nil && buf.Modified() {
		p.h.Notify("Save or revert " + th.Path + " first, then apply the suggestion.")
		return
	}
	start, end := th.StartLine, th.Line
	if start == 0 {
		start = end
	}
	p.act(&d.busy, "Applying suggestion", "Cannot apply the suggestion", func(context.Context) (func(), error) {
		info, err := os.Stat(target)
		if err != nil {
			return nil, err
		}
		data, err := os.ReadFile(target)
		if err != nil {
			return nil, err
		}
		text, err := applySuggestionText(string(data), start, end, code)
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(target, []byte(text), info.Mode().Perm()); err != nil {
			return nil, err
		}
		return func() { p.h.Notify("Applied the suggestion to " + th.Path) }, nil
	})
}

func (t *diffTab) lineRange() (int, int) {
	if t.state == nil {
		return 0, 0
	}
	r, ok := t.state.SelectedLines()
	if !ok {
		return 0, 0
	}
	return r.Start + 1, r.End + 1
}

func (p *PRs) linkTarget(t *diffTab) (github.Remote, string) {
	repo := p.repo
	sha := t.data.pr.HeadSHA
	if r, ok := t.state.SelectedLines(); ok && r.Side == editor.DiffOld && t.oldSHA != "" {
		sha = t.oldSHA
	}
	return repo, sha
}

func (p *PRs) linkMenu(c *ui.Context, m *ui.Menu, t *diffTab) {
	d := t.data
	start, end := t.lineRange()
	repo, sha := p.linkTarget(t)
	permalink := blobURL(repo, sha, t.file.Path, start, end)
	headRepo := repo
	if d.pr.HeadOwner != "" && d.pr.HeadRepo != "" {
		headRepo = github.Remote{Host: repo.Host, Owner: d.pr.HeadOwner, Name: d.pr.HeadRepo}
	}
	if m.Item("Copy GitHub Permalink").Chosen() {
		c.WriteClipboard(permalink)
	}
	if m.Item("Copy Link to Branch").Disabled(d.pr.HeadRef == "").Chosen() {
		c.WriteClipboard(blobURL(headRepo, d.pr.HeadRef, t.file.Path, start, end))
	}
	if m.Item("Copy as Markdown").Chosen() {
		c.WriteClipboard(markdownPermalink(t.file.Path, start, end, permalink))
	}
	if m.Item("Copy Pull Request Link").Disabled(d.pr.URL == "").Chosen() {
		c.WriteClipboard(d.pr.URL)
	}
	m.Separator()
	if m.Item("Open Permalink on GitHub").Chosen() {
		openExternal(c, permalink)
	}
	dev, ok := devURL(repo, sha, t.file.Path, start, end)
	if m.Item("Open in github.dev").Disabled(!ok).Chosen() {
		openExternal(c, dev)
	}
}

func (p *PRs) threadView(c *ui.Context, t *diffTab, a editor.DiffAnnotation) {
	id, _ := a.Key.(string)
	d := t.data
	var th *github.Thread
	for i := range d.threads {
		if d.threads[i].ID == id {
			th = &d.threads[i]
		}
	}
	if th == nil {
		return
	}
	pal := token.Of(c)
	now := p.env.Now()
	box := ui.Column(c.Key(id)).FillWidth().Height(a.Height).ClipY().Padding(token.GapControl.Px(), token.PadPanel.Px()).Gap(token.GapInline.Px()).
		Background(pal.Layer.Panel).Border(token.BorderWidth.Px(), pal.Border.Default).Radius(token.RadiusCard.Px())
	thread := *th
	box.Children(func() {
		collapsed := thread.Resolved && !t.expanded[id]
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(token.GapControl.Px()).Children(func() {
			if thread.Resolved {
				badge.Tag(c, "Resolved", badge.TagOptions{Tone: token.ToneSuccess})
			}
			if thread.Outdated {
				mutedTag(c, "Outdated")
			}
			if collapsed {
				mutedText(c, fmt.Sprintf("%d comments", len(thread.Comments))).Grow(1)
				ghostButton(c, "Show", "show-"+id).Label("Show thread").OnClick(func() { t.expanded[id] = true })
				return
			}
			design.Spacer(c)
			if thread.Resolved {
				ghostButton(c, "Hide", "hide-"+id).Label("Hide thread").OnClick(func() { delete(t.expanded, id) })
			}
		})
		if collapsed {
			return
		}
		for i, cm := range thread.Comments {
			p.threadComment(c, d, thread, i, cm, now)
		}
		ui.Row(c).FillWidth().Gap(token.GapControl.Px()).Children(func() {
			smallButton(c, "Reply", d.busy != "").OnClick(func() { p.replyTo(d, thread) })
			if thread.Resolved {
				smallButton(c, "Unresolve", d.busy != "").OnClick(func() { p.setResolved(d, thread, false) })
			} else {
				smallButton(c, "Resolve", d.busy != "").OnClick(func() { p.setResolved(d, thread, true) })
			}
		})
	})
}

func (p *PRs) threadComment(c *ui.Context, d *prData, th github.Thread, i int, cm github.Comment, now time.Time) {
	own := p.viewer != "" && cm.Author == p.viewer
	suggestion, hasSuggestion := suggestionOf(cm.Body)
	ui.Column(c.Key(i)).FillWidth().Gap(token.GapTight.Px()).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(token.GapRowIcon.Px()).Children(func() {
			badge.Avatar(c, cm.Author, token.AvatarSize)
			design.Text(c, cm.Author, token.TextStrong)
			mutedText(c, relativeTime(now, cm.Created))
			design.Spacer(c)
			if cm.URL != "" {
				ghostButton(c, "Copy Link", "copy-"+strconv.Itoa(i)).Label("Copy link to comment by " + cm.Author).OnClick(func() { c.WriteClipboard(cm.URL) })
			}
			if hasSuggestion && th.Side == github.SideRight && !th.Outdated {
				ghostButton(c, "Apply Suggestion", "apply-"+strconv.Itoa(i)).Disabled(d.busy != "").OnClick(func() { p.applySuggestion(d, th, suggestion) })
			}
			if own {
				ghostButton(c, "Edit", "edit-"+strconv.Itoa(i)).Label("Edit comment by " + cm.Author).Disabled(d.busy != "").OnClick(func() { p.editThreadComment(d, th, cm) })
				ghostButton(c, "Delete", "delete-"+strconv.Itoa(i)).Label("Delete comment by " + cm.Author).Disabled(d.busy != "").OnClick(func() { p.deleteThreadComment(d, cm) })
			}
		})
		design.Text(c, cm.Body, token.TextBody).Selectable()
	})
}
