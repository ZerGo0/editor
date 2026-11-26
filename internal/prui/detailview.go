package prui

import (
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ZerGo0/editor/internal/design"
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/badge"
	"github.com/ZerGo0/editor/internal/design/detail"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/field"
	"github.com/ZerGo0/editor/internal/design/list"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/github"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
)

const (
	moreActionsLabel = "More"
	commentLines     = 6
)

var reactionNames = []struct{ content, label string }{
	{"THUMBS_UP", "+1"}, {"THUMBS_DOWN", "-1"}, {"LAUGH", "Laugh"}, {"HOORAY", "Hooray"},
	{"CONFUSED", "Confused"}, {"HEART", "Heart"}, {"ROCKET", "Rocket"}, {"EYES", "Eyes"},
}

func reactionLabel(content string) string {
	for _, r := range reactionNames {
		if r.content == content {
			return r.label
		}
	}
	return strings.ToLower(content)
}

func openExternal(c *ui.Context, raw string) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return
	}
	c.OpenURL(raw)
}

func markdownLink(pr github.PullRequest) string {
	return fmt.Sprintf("[#%d %s](%s)", pr.Number, pr.Title, pr.URL)
}

func (p *PRs) detailView(c *ui.Context, d *prData) {
	if p.closed {
		return
	}
	p.detailTick(c, d)
	detail.Page(c, func() {
		switch {
		case d.err != nil && !d.loaded:
			detail.Centered(c, func() {
				feedback.Error(c, "Cannot load #"+strconv.Itoa(d.number), d.err, func() { p.loadDetail(d) })
			})
		case !d.loaded && d.pr.Title == "":
			detail.Centered(c, func() { feedback.Loading(c, "Loading #"+strconv.Itoa(d.number)+"...") })
		default:
			p.detailHeader(c, d)
			detail.Body(c, "Pull request", token.PageMaxWidth, func() { p.detailBody(c, d) })
		}
	})
}

func (p *PRs) detailHeader(c *ui.Context, d *prData) {
	pr := d.pr
	o := detail.HeaderOptions{Title: pr.Title, Number: pr.Number, Busy: d.busy != "" || d.loading}
	if d.editingTitle {
		o.Number = 0
		o.Editor = func(c *ui.Context) {
			title := field.TextInput(c, "Pull request title", &d.titleDraft, field.InputOptions{AutoFocus: true, ID: "title"})
			if title.Submitted() {
				p.saveTitle(d, d.titleDraft)
			}
			action.Button(c, "Save Title", action.ButtonOptions{Variant: action.Primary, Small: true}).OnClick(func() { p.saveTitle(d, d.titleDraft) })
			action.Button(c, "Cancel", action.ButtonOptions{Variant: action.Ghost, Small: true}).OnClick(func() { d.editingTitle = false })
		}
	}
	o.Actions = func(c *ui.Context) {
		if !d.editingTitle && (!d.hasOv || d.ov.CanUpdate) {
			if action.IconButton(c, icons.Edit, "Edit Title", action.IconOptions{Disabled: d.busy != ""}).Clicked() {
				d.editingTitle, d.titleDraft = true, pr.Title
			}
		}
		if d.loading {
			feedback.Spinner(c, "Refreshing")
		}
		if action.IconButton(c, icons.Refresh, "Refresh", action.IconOptions{ID: "refresh"}).Clicked() {
			p.loadDetail(d)
		}
	}
	o.Meta = func(c *ui.Context) {
		stateTag(c, pr)
		author := pr.Author
		if author == "" {
			author = "unknown"
		}
		badge.Avatar(c, author, token.AvatarSize)
		mutedText(c, author+" wants to merge").Shrink(0)
		branchChip(c, pr.HeadRef)
		mutedText(c, "into").Shrink(0)
		branchChip(c, pr.BaseRef)
		if when := relativeTime(p.env.Now(), pr.Updated); when != "" {
			mutedText(c, "Updated "+when)
		}
		if summary := mergeSummary(d); summary != "" && summary != prStateMerged && summary != prStateClosed {
			badge.Tag(c, summary, badge.TagOptions{Tone: mergeSummaryTone(d)})
		}
	}
	o.Toolbar = func(c *ui.Context) { p.detailActions(c, d) }
	detail.Header(c, o)
}

func mergeSummaryTone(d *prData) token.Tone {
	switch d.ov.MergeState {
	case mergeStateClean:
		return token.ToneSuccess
	case mergeStateDirty:
		return token.ToneError
	case mergeStateBehind, mergeStateBlocked, mergeStateUnstable:
		return token.ToneWarning
	}
	return token.ToneNeutral
}

func (p *PRs) detailActions(c *ui.Context, d *prData) {
	pr := d.pr
	idle := d.busy == ""
	checkedOut := p.current.pr != nil && p.current.pr.Number == d.number
	label := "Checkout"
	if checkedOut {
		label = "Checked Out"
	}
	smallButton(c, label, !idle || checkedOut || !d.isOpen()).OnClick(func() { p.checkoutPR(d.number, &d.busy) })
	smallButton(c, "Open on GitHub", pr.URL == "").OnClick(func() { openExternal(c, pr.URL) })
	smallButton(c, "Copy Link", pr.URL == "").OnClick(func() { c.WriteClipboard(pr.URL) })
	if pr.Draft && d.isOpen() {
		smallButton(c, "Mark Ready for Review", !idle).OnClick(func() { p.markReady(d) })
	}
	if d.isOpen() {
		p.mergeButton(c, d, idle)
		if d.pendingReview != "" {
			action.Button(c, "Submit Review", action.ButtonOptions{Variant: action.Primary, Small: true, Disabled: !idle}).OnClick(func() { p.openReviewDialog(d, 0) })
			smallButton(c, "Discard Review", !idle).OnClick(func() { p.discardReview(d) })
		}
		smallButton(c, "Approve", !idle).OnClick(func() { p.openReviewDialog(d, 1) })
		smallButton(c, "Request Changes", !idle).OnClick(func() { p.openReviewDialog(d, 2) })
		smallButton(c, "Comment", !idle).OnClick(func() { p.openReviewDialog(d, 0) })
	}
	action.MenuButton(c, moreActionsLabel, action.ButtonOptions{Small: true, Disabled: !idle, ID: "more"}, func(m *ui.Menu) { p.moreMenu(c, m, d) }).Label("More actions")
	if !idle {
		feedback.Spinner(c, d.busy)
		mutedText(c, d.busy+"...")
	}
}

func (p *PRs) mergeButton(c *ui.Context, d *prData, idle bool) {
	blocked := d.mergeBlocked()
	switch {
	case d.hasOv && d.ov.Queue.InQueue:
		smallButton(c, "Remove from Merge Queue", !idle).OnClick(func() { p.dequeue(d) })
	case d.hasOv && d.ov.Queue.Enabled:
		b := smallButton(c, "Add to Merge Queue", blocked != "" || !idle).OnClick(func() { p.enqueue(d) })
		if blocked != "" {
			b.Tooltip(blocked)
		}
	default:
		b := smallButton(c, "Merge", blocked != "" || !idle).OnClick(func() { p.openMergeDialog(d) })
		if blocked != "" {
			b.Tooltip(blocked)
		}
	}
}

func (p *PRs) moreMenu(c *ui.Context, m *ui.Menu, d *prData) {
	pr := d.pr
	open := d.isOpen()
	canEdit := !d.hasOv || d.ov.CanUpdate
	if m.Item("Copy Pull Request Link").Chosen() {
		c.WriteClipboard(pr.URL)
	}
	if m.Item("Copy as Markdown Link").Chosen() {
		c.WriteClipboard(markdownLink(pr))
	}
	m.Separator()
	if m.Item("Edit Description...").Disabled(!canEdit).Chosen() {
		p.openBodyEditor(d)
	}
	if m.Item("Request Review...").Disabled(!open).Chosen() {
		p.openReviewerPicker(d)
	}
	if m.Item("Edit Assignees...").Disabled(!canEdit).Chosen() {
		p.openAssigneePicker(d)
	}
	if m.Item("Edit Labels...").Disabled(!canEdit).Chosen() {
		p.openLabelPicker(d)
	}
	if m.Item("Set Milestone...").Disabled(!canEdit).Chosen() {
		p.openMilestonePicker(d)
	}
	m.Separator()
	if m.Item("Convert to Draft").Disabled(!open || pr.Draft).Chosen() {
		p.convertToDraft(d)
	}
	if m.Item("Update Branch").Disabled(!open || !d.hasOv || d.ov.MergeState != mergeStateBehind).Chosen() {
		p.updateBranch(d)
	}
	switch {
	case d.hasOv && d.ov.AutoMerge != nil:
		if m.Item("Disable Auto-Merge").Disabled(!d.ov.CanDisableAutoMerge).Chosen() {
			p.disableAutoMerge(d)
		}
	default:
		if m.Item("Enable Auto-Merge...").Disabled(!open || !d.hasOv || !d.ov.CanEnableAutoMerge).Chosen() {
			p.openAutoMergeDialog(d)
		}
	}
	m.Separator()
	if open {
		if m.Item("Close Pull Request").Chosen() {
			p.setState(d, github.StateClosed)
		}
	} else if !pr.Merged {
		if m.Item("Reopen Pull Request").Chosen() {
			p.setState(d, github.StateOpen)
		}
	}
	if m.Item("Revert Pull Request").Disabled(!pr.Merged).Chosen() {
		p.revert(d)
	}
	if m.Item("Delete Branch").Disabled(open).Chosen() {
		p.deleteBranch(d)
	}
}

func (p *PRs) detailBody(c *ui.Context, d *prData) {
	if len(d.partErrs) > 0 {
		feedback.Notice(c, feedback.NoticeOptions{Tone: token.ToneWarning, Text: "Some details could not be loaded: " + strings.Join(d.partErrs, "; "), Action: retryLabel, OnAction: func() { p.loadDetail(d) }})
	}
	p.descriptionCard(c, d)
	p.peopleCard(c, d)
	p.checksCard(c, d)
	p.filesCard(c, d)
	if len(d.since) > 0 {
		p.sinceCard(c, d)
	}
	p.commitsCard(c, d)
	p.timelineCard(c, d)
	p.commentBox(c, d)
}

func (p *PRs) descriptionCard(c *ui.Context, d *prData) {
	var actions func(c *ui.Context)
	if !d.hasOv || d.ov.CanUpdate {
		actions = func(c *ui.Context) {
			smallButton(c, "Edit Description", d.busy != "").OnClick(func() { p.openBodyEditor(d) })
		}
	}
	detail.Card(c, "Description", detail.CardOptions{Actions: actions}, func() {
		if strings.TrimSpace(d.pr.Body) == "" {
			mutedText(c, "No description provided.")
			return
		}
		markdown.View(c.Key("description"), d.pr.Body, &d.md)
	})
}

func (p *PRs) peopleCard(c *ui.Context, d *prData) {
	states := latestReviewStates(d.reviews)
	reviewers := slices.Clone(d.pr.Reviewers)
	for author := range states {
		if !slices.Contains(reviewers, author) {
			reviewers = append(reviewers, author)
		}
	}
	slices.Sort(reviewers)
	canEdit := !d.hasOv || d.ov.CanUpdate
	edit := func(label string, onClick func()) func(c *ui.Context) {
		if !canEdit || !d.isOpen() {
			return nil
		}
		return func(c *ui.Context) { smallButton(c, label, d.busy != "").OnClick(onClick) }
	}
	none := func(c *ui.Context, text string) { mutedText(c, text) }
	detail.Card(c, "Reviewers and Labels", detail.CardOptions{}, func() {
		detail.Property(c, "Reviewers", func(c *ui.Context) {
			if len(reviewers) == 0 {
				none(c, "None requested")
			}
			for _, name := range reviewers {
				text := name + " requested"
				if state, ok := states[name]; ok {
					text = name + " " + reviewStateLabel(state)
				}
				mutedTag(c, text)
			}
		}, edit("Request Review", func() { p.openReviewerPicker(d) }))
		detail.Property(c, "Assignees", func(c *ui.Context) {
			if len(d.ov.Assignees) == 0 {
				none(c, "None")
			}
			for _, a := range d.ov.Assignees {
				mutedTag(c, a)
			}
		}, edit("Edit Assignees", func() { p.openAssigneePicker(d) }))
		detail.Property(c, "Labels", func(c *ui.Context) {
			if len(d.pr.Labels) == 0 {
				none(c, "None")
			}
			for _, name := range d.pr.Labels {
				mutedTag(c, name)
			}
		}, edit("Edit Labels", func() { p.openLabelPicker(d) }))
		detail.Property(c, "Milestone", func(c *ui.Context) {
			if d.ov.Milestone == nil {
				none(c, "None")
				return
			}
			mutedTag(c, d.ov.Milestone.Title)
		}, edit("Set Milestone", func() { p.openMilestonePicker(d) }))
		if len(d.ov.Linked) > 0 {
			detail.Property(c, "Closes", func(c *ui.Context) {
				for _, issue := range d.ov.Linked {
					ghostButton(c, fmt.Sprintf("#%d %s", issue.Number, issue.Title), "linked").Label(fmt.Sprintf("Open issue #%d", issue.Number)).OnClick(func() { openExternal(c, issue.URL) })
				}
			}, nil)
		}
	})
}

func (p *PRs) checksCard(c *ui.Context, d *prData) {
	plain := make([]github.Check, len(d.checks))
	for i, chk := range d.checks {
		plain[i] = chk.Check
	}
	detail.Card(c, "Checks", detail.CardOptions{Count: len(d.checks)}, func() {
		if len(d.checks) == 0 {
			mutedText(c, "No checks reported for this commit.")
			return
		}
		mutedText(c, fmt.Sprintf("%d of %d checks passed", countChecks(plain), len(d.checks)))
		for i, chk := range d.checks {
			ui.Row(c.Key(i)).FillWidth().AlignItems(ui.Center).Gap(token.GapControl.Px()).Children(func() {
				checkIcon(c, checkOutcome(chk.Check))
				design.Text(c, chk.Name, token.TextBody).SingleLine().Grow(1).MinWidth(0)
				if chk.Required {
					mutedTag(c, "Required")
				}
				if chk.Conclusion != "" {
					mutedText(c, chk.Conclusion).Shrink(0)
				} else {
					mutedText(c, chk.Status).Shrink(0)
				}
				p.checkButtons(c, d, chk)
			})
		}
	})
}

func (p *PRs) checkButtons(c *ui.Context, d *prData, chk checkRow) {
	if chk.URL != "" {
		ghostButton(c, "Details", "details-"+chk.Name).Label("Details for " + chk.Name).OnClick(func() { openExternal(c, chk.URL) })
	}
	if chk.JobID > 0 && chk.Status != "queued" {
		ghostButton(c, "View Logs", "logs-"+chk.Name).Label("View logs of " + chk.Name).OnClick(func() { p.openLogs(d, chk) })
	}
	if chk.RunID <= 0 {
		return
	}
	switch outcome := checkOutcome(chk.Check); {
	case outcome == checkFailure && chk.Conclusion == "action_required":
		ghostButton(c, "Approve Run", "approve-"+chk.Name).Label("Approve run of " + chk.Name).OnClick(func() { p.runAction(d, chk, actionApprove) })
	case outcome == checkFailure:
		ghostButton(c, "Re-run", "rerun-"+chk.Name).Label("Re-run " + chk.Name).OnClick(func() { p.runAction(d, chk, actionRerunFailed) })
	case outcome == checkPending:
		ghostButton(c, "Cancel", "cancel-"+chk.Name).Label("Cancel " + chk.Name).OnClick(func() { p.runAction(d, chk, actionCancel) })
	}
}

func (p *PRs) filesCard(c *ui.Context, d *prData) {
	files := d.visibleFiles()
	var actions func(c *ui.Context)
	if d.viewedCount() > 0 || d.hideViewed {
		actions = func(c *ui.Context) {
			action.Checkbox(c, "Hide viewed", &d.hideViewed, action.ChoiceOptions{ID: "hide-viewed"})
		}
	}
	detail.Card(c, "Changed files", detail.CardOptions{Count: len(d.files), Actions: actions}, func() {
		if len(d.files) == 0 {
			mutedText(c, "No changed files.")
			return
		}
		mutedText(c, fmt.Sprintf("+%d -%d across %d files, %d viewed. Select a file to open its diff.", d.pr.Additions, d.pr.Deletions, len(d.files), d.viewedCount()))
		p.fileList(c, d, &d.fileList, &d.fileRow, files, modePR, "Changed files")
	})
}

func (p *PRs) sinceCard(c *ui.Context, d *prData) {
	detail.Card(c, "Changes since your last review", detail.CardOptions{Count: len(d.since)}, func() {
		mutedText(c, "Compared with commit "+shortSHA(d.sinceSHA)+", the one you last reviewed.")
		p.fileList(c, d, &d.sinceList, &d.sinceRow, d.since, modeSince, "Changes since last review")
	})
}

func (p *PRs) fileList(c *ui.Context, d *prData, state *ui.ListState, row *int, files []github.PRFile, mode, label string) {
	if len(files) == 0 {
		mutedText(c, "Every file is marked as viewed.")
		return
	}
	state.Key = func(i int) any { return files[i].Path }
	state.Label = func(i int) string { return files[i].Path }
	state.Selected = row
	rows := min(len(files), token.ListRowsMax)
	spec := diffSpec{mode: mode, oldRef: d.sinceSHA}
	var el ui.Element
	ui.Column(c).FillWidth().Height(float32(rows) * token.RowHeight.Px()).Shrink(0).Children(func() {
		el = list.View(c, state, len(files), list.ViewOptions{Label: label}, func(i int) list.Item {
			f := files[i]
			it := list.Item{
				Label:       f.Path,
				Git:         fileGitStatus(f.Status),
				OnClick:     func() { p.openDiff(d, f, spec) },
				Description: fmt.Sprintf("+%d -%d", f.Additions, f.Deletions),
			}
			if n := d.commentCount(f.Path); n > 0 && mode == modePR {
				it.Count = n
			}
			if mode == modePR && d.pr.NodeID != "" {
				it.Leading = func(c *ui.Context) { p.viewedBox(c, d, f.Path) }
			}
			return it
		})
	})
	if el.Submitted() && *row >= 0 && *row < len(files) {
		p.openDiff(d, files[*row], spec)
	}
}

func (p *PRs) commitsCard(c *ui.Context, d *prData) {
	detail.Card(c, "Commits", detail.CardOptions{Count: len(d.commits)}, func() {
		if len(d.commits) == 0 {
			mutedText(c, "No commits.")
			return
		}
		now := p.env.Now()
		for i, cm := range d.commits {
			ui.Row(c.Key(i)).FillWidth().AlignItems(ui.Center).Gap(token.GapControl.Px()).Children(func() {
				token.TextMono.Apply(ui.Text(c, shortSHA(cm.SHA))).TextColor(token.Of(c).Text.Secondary).Shrink(0)
				design.Text(c, firstLine(cm.Message), token.TextBody).SingleLine().Grow(1).MinWidth(0)
				mutedText(c, cm.Author).SingleLine().Shrink(0)
				mutedText(c, relativeTime(now, cm.Date)).Shrink(0)
				if cm.URL != "" {
					ghostButton(c, "Open", "commit-"+cm.SHA).Label("Open commit " + shortSHA(cm.SHA)).OnClick(func() { openExternal(c, cm.URL) })
				}
			})
		}
	})
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}

func eventSentence(e TimelineEvent) string {
	switch e.Kind {
	case "LabeledEvent":
		return "added the " + e.Detail + " label"
	case "UnlabeledEvent":
		return "removed the " + e.Detail + " label"
	case "AssignedEvent":
		return "assigned " + e.Detail
	case "UnassignedEvent":
		return "unassigned " + e.Detail
	case "ReviewRequestedEvent":
		return "requested a review from " + e.Detail
	case "ReviewRequestRemovedEvent":
		return "removed the review request for " + e.Detail
	case "HeadRefForcePushedEvent":
		return "force-pushed the branch (" + e.Detail + ")"
	case "MergedEvent":
		return "merged commit " + e.Detail
	case "ClosedEvent":
		return "closed this pull request"
	case "ReopenedEvent":
		return "reopened this pull request"
	case "ReadyForReviewEvent":
		return "marked this pull request as ready for review"
	case "ConvertToDraftEvent":
		return "marked this pull request as a draft"
	case "AutoMergeEnabledEvent":
		return "enabled auto-merge"
	case "AutoMergeDisabledEvent":
		return "disabled auto-merge"
	case "RenamedTitleEvent":
		return "changed the title from " + e.Detail
	case "HeadRefDeletedEvent":
		return "deleted the branch " + e.Detail
	}
	return strings.ToLower(e.Kind)
}

func (p *PRs) timelineCard(c *ui.Context, d *prData) {
	detail.Card(c, "Conversation", detail.CardOptions{Count: len(d.timeline)}, func() {
		if len(d.timeline) == 0 {
			mutedText(c, "No activity yet.")
			return
		}
		now := p.env.Now()
		for i, e := range d.timeline {
			if !e.isComment() && !e.isReview() {
				detail.Activity(c, e.Actor, eventSentence(e), relativeTime(now, e.Time))
				continue
			}
			p.commentView(c, d, i, e, now)
		}
	})
}

func (p *PRs) commentView(c *ui.Context, d *prData, i int, e TimelineEvent, now time.Time) {
	verb := "commented"
	if e.isReview() {
		verb = reviewStateLabel(e.State)
	}
	o := detail.CommentOptions{Author: e.Actor, Verb: verb, Time: relativeTime(now, e.Time)}
	o.Actions = func(c *ui.Context) {
		if e.URL != "" {
			ghostButton(c, "Copy Link", "copy-"+strconv.Itoa(i)).Label("Copy link to comment by " + e.Actor).OnClick(func() { c.WriteClipboard(e.URL) })
		}
		if e.isComment() && e.CanEdit {
			ghostButton(c, "Edit", "edit-"+strconv.Itoa(i)).Label("Edit comment by " + e.Actor).Disabled(d.busy != "").OnClick(func() { p.editComment(d, e) })
		}
		if e.isComment() && e.CanDelete {
			ghostButton(c, "Delete", "delete-"+strconv.Itoa(i)).Label("Delete comment by " + e.Actor).Disabled(d.busy != "").OnClick(func() { p.deleteComment(d, e) })
		}
	}
	detail.Comment(c.Key(i), o, func() {
		if strings.TrimSpace(e.Body) != "" {
			markdown.View(c.Key("body"), e.Body, &d.md)
		}
		if e.isComment() {
			p.reactionBar(c, d, e)
		}
	})
}

func (p *PRs) reactionBar(c *ui.Context, d *prData, e TimelineEvent) {
	ui.Row(c).FillWidth().AlignItems(ui.Center).Wrap().Gap(token.GapInline.Px()).Children(func() {
		for _, r := range e.Reactions {
			if r.Count == 0 {
				continue
			}
			variant := action.Secondary
			if r.Viewer {
				variant = action.Primary
			}
			label := reactionLabel(r.Content)
			action.Button(c, fmt.Sprintf("%s %d", label, r.Count), action.ButtonOptions{Variant: variant, Small: true, Disabled: d.busy != "", ID: "reaction-" + r.Content}).
				Label(label + " reaction").OnClick(func() { p.react(d, e, r.Content) })
		}
		action.MenuButton(c, "React", action.ButtonOptions{Variant: action.Ghost, Small: true, Disabled: d.busy != "", ID: "react"}, func(m *ui.Menu) {
			for _, r := range reactionNames {
				if action.Item(m, r.label, action.ItemOptions{}).Chosen() {
					p.react(d, e, r.content)
				}
			}
		}).Label("Add reaction")
	})
}

func (p *PRs) commentBox(c *ui.Context, d *prData) {
	detail.Card(c, "Add a comment", detail.CardOptions{}, func() {
		field.TextArea(c, "Comment", &d.draft, field.AreaOptions{Placeholder: "Leave a comment (Markdown is supported)", MinLines: commentLines, ID: "comment"})
		blank := trimmed(d.draft) == ""
		ui.Row(c).Justify(ui.End).FillWidth().Children(func() {
			action.Button(c, "Add Comment", action.ButtonOptions{Variant: action.Primary, Disabled: blank || d.busy != ""}).OnClick(func() { p.postComment(d) })
		})
	})
}
