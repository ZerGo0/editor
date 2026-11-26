package prui

import (
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ZerGo0/editor/internal/github"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
)

const (
	fileListRowHeight = 28
	fileListMaxRows   = 12
	commentBoxHeight  = 80
	statLetterWidth   = 16
	moreActionsLabel  = "More"
	statusLabelWidth  = 84
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
	t := c.Theme()
	ui.Column(c).Fill().Background(t.Background).Children(func() {
		switch {
		case d.err != nil && !d.loaded:
			ui.Column(c).Grow(1).FillWidth().Center().Gap(spacingMedium).Padding(messagePad).Children(func() {
				ui.Text(c, "Cannot load #"+strconv.Itoa(d.number)).Bold()
				ui.Text(c, errorText(d.err)).FontSize(fontSmall).TextColor(t.Danger)
				ui.Button(c, "Retry").OnClick(func() { p.loadDetail(d) })
			})
		case !d.loaded && d.pr.Title == "":
			ui.Column(c).Grow(1).FillWidth().Center().Children(func() { loadingRow(c, "Loading #"+strconv.Itoa(d.number)+"...") })
		default:
			p.detailHeader(c, d)
			ui.Scroll(c).Grow(1).FillWidth().Children(func() {
				ui.Column(c).Gap(spacingLarge).Padding(messagePad).MaxWidth(contentMax).FillWidth().Children(func() { p.detailBody(c, d) })
			})
		}
	})
}

func (p *PRs) detailHeader(c *ui.Context, d *prData) {
	t := c.Theme()
	pr := d.pr
	ui.Column(c).Gap(spacingMedium).Padding(spacingLarge, messagePad).Shrink(0).FillWidth().Background(t.Surface).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
		ui.Row(c).Gap(spacingMedium).AlignItems(ui.Center).FillWidth().Children(func() {
			if d.editingTitle {
				field := ui.TextInput(c.Key("title"), &d.titleDraft).AutoFocus().Grow(1).Label("Pull request title")
				if field.Submitted() {
					p.saveTitle(d, d.titleDraft)
				}
				ui.PrimaryButton(c, "Save Title").OnClick(func() { p.saveTitle(d, d.titleDraft) })
				ui.Button(c, "Cancel").OnClick(func() { d.editingTitle = false })
			} else {
				ui.Text(c, pr.Title).FontSize(fontTitle).Bold().Grow(1).MinWidth(0)
				ui.Text(c, "#"+strconv.Itoa(pr.Number)).FontSize(fontTitle).TextColor(t.TextMuted).Shrink(0)
				if !d.hasOv || d.ov.CanUpdate {
					ui.Button(c, "Edit Title").Disabled(d.busy != "").OnClick(func() { d.editingTitle, d.titleDraft = true, pr.Title })
				}
			}
			if d.loading {
				ui.Spinner(c).Label("Refreshing")
			}
			if iconButton(c, "refresh", "Refresh", icons.Refresh).Clicked() {
				p.loadDetail(d)
			}
		})
		ui.Row(c).Gap(spacingMedium).AlignItems(ui.Center).Wrap().FillWidth().Children(func() {
			pill(c, prStateLabel(pr), stateColor(t, pr))
			author := pr.Author
			if author == "" {
				author = "unknown"
			}
			mutedText(c, author+" wants to merge").Shrink(0)
			branchChip(c, pr.HeadRef)
			mutedText(c, "into").Shrink(0)
			branchChip(c, pr.BaseRef)
			if when := relativeTime(p.env.Now(), pr.Updated); when != "" {
				mutedText(c, "Updated "+when)
			}
		})
		if summary := mergeSummary(d); summary != "" && summary != prStateMerged && summary != prStateClosed {
			ui.Text(c, summary).FontSize(fontSmall).TextColor(mergeSummaryColor(t, d))
		}
		p.detailActions(c, d)
	})
}

func mergeSummaryColor(t *ui.Theme, d *prData) ui.Color {
	switch d.ov.MergeState {
	case mergeStateClean:
		return t.Success
	case mergeStateDirty:
		return t.Danger
	case mergeStateBehind, mergeStateBlocked, mergeStateUnstable:
		return t.Warning
	}
	return t.TextMuted
}

func branchChip(c *ui.Context, name string) {
	t := c.Theme()
	ui.Row(c).Gap(spacingSmall).AlignItems(ui.Center).Background(t.SurfacePressed).Radius(cardRadius).Padding(1, spacingMedium).Shrink(0).Children(func() {
		ui.Icon(c, icons.GitBranch).Size(iconSmall, iconSmall).TextColor(t.TextMuted)
		ui.Text(c, name).FontSize(fontSmall).SingleLine()
	})
}

func (p *PRs) detailActions(c *ui.Context, d *prData) {
	pr := d.pr
	idle := d.busy == ""
	ui.Row(c).Gap(spacingMedium).AlignItems(ui.Center).Wrap().FillWidth().Children(func() {
		checkedOut := p.current.pr != nil && p.current.pr.Number == d.number
		label := "Checkout"
		if checkedOut {
			label = "Checked Out"
		}
		ui.Button(c, label).Disabled(!idle || checkedOut || !d.isOpen()).OnClick(func() { p.checkoutPR(d.number, &d.busy) })
		ui.Button(c, "Open on GitHub").Disabled(pr.URL == "").OnClick(func() { openExternal(c, pr.URL) })
		ui.Button(c, "Copy Link").Disabled(pr.URL == "").OnClick(func() { c.WriteClipboard(pr.URL) })
		if pr.Draft && d.isOpen() {
			ui.Button(c, "Mark Ready for Review").Disabled(!idle).OnClick(func() { p.markReady(d) })
		}
		if d.isOpen() {
			p.mergeButton(c, d, idle)
			if d.pendingReview != "" {
				ui.PrimaryButton(c, "Submit Review").Disabled(!idle).OnClick(func() { p.openReviewDialog(d, 0) })
				ui.Button(c, "Discard Review").Disabled(!idle).OnClick(func() { p.discardReview(d) })
			}
			ui.Button(c, "Approve").Disabled(!idle).OnClick(func() { p.openReviewDialog(d, 1) })
			ui.Button(c, "Request Changes").Disabled(!idle).OnClick(func() { p.openReviewDialog(d, 2) })
			ui.Button(c, "Comment").Disabled(!idle).OnClick(func() { p.openReviewDialog(d, 0) })
		}
		ui.MenuButton(c, moreActionsLabel, func(m *ui.Menu) { p.moreMenu(c, m, d) }).Label("More actions").Disabled(!idle)
		if !idle {
			ui.Row(c).Gap(spacingSmall).AlignItems(ui.Center).Children(func() {
				ui.Spinner(c).Label(d.busy)
				mutedText(c, d.busy+"...")
			})
		}
	})
}

func (p *PRs) mergeButton(c *ui.Context, d *prData, idle bool) {
	blocked := d.mergeBlocked()
	switch {
	case d.hasOv && d.ov.Queue.InQueue:
		ui.Button(c, "Remove from Merge Queue").Disabled(!idle).OnClick(func() { p.dequeue(d) })
	case d.hasOv && d.ov.Queue.Enabled:
		b := ui.Button(c, "Add to Merge Queue").Disabled(blocked != "" || !idle).OnClick(func() { p.enqueue(d) })
		if blocked != "" {
			b.Tooltip(blocked)
		}
	default:
		b := ui.Button(c, "Merge").Disabled(blocked != "" || !idle).OnClick(func() { p.openMergeDialog(d) })
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
	t := c.Theme()
	if len(d.partErrs) > 0 {
		ui.Column(c).Gap(spacingSmall).FillWidth().Children(func() {
			ui.Text(c, "Some details could not be loaded:").FontSize(fontSmall).TextColor(t.Warning)
			for _, msg := range d.partErrs {
				ui.Text(c, msg).FontSize(fontSmall).TextColor(t.TextMuted)
			}
			ui.Button(c, "Retry").OnClick(func() { p.loadDetail(d) })
		})
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
	card(c, "Description", 0, func() {
		if strings.TrimSpace(d.pr.Body) == "" {
			mutedText(c, "No description provided.").Italic()
		} else {
			markdown.View(c.Key("description"), d.pr.Body, &d.md)
		}
		if !d.hasOv || d.ov.CanUpdate {
			ui.Row(c).FillWidth().Children(func() {
				ui.Button(c, "Edit Description").Disabled(d.busy != "").OnClick(func() { p.openBodyEditor(d) })
			})
		}
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
	card(c, "Reviewers and Labels", 0, func() {
		t := c.Theme()
		line := func(title string, edit string, action func(), items func()) {
			ui.Row(c).Gap(spacingMedium).AlignItems(ui.Center).Wrap().FillWidth().Children(func() {
				mutedText(c, title).Width(statusLabelWidth).Shrink(0)
				items()
				if action != nil && canEdit && d.isOpen() {
					ui.Button(c, edit).Disabled(d.busy != "").OnClick(action)
				}
			})
		}
		line("Reviewers", "Request Review", func() { p.openReviewerPicker(d) }, func() {
			if len(reviewers) == 0 {
				mutedText(c, "None requested")
			}
			for _, name := range reviewers {
				text := name + " requested"
				if state, ok := states[name]; ok {
					text = name + " " + reviewStateLabel(state)
				}
				mutedPill(c, text)
			}
		})
		line("Assignees", "Edit Assignees", func() { p.openAssigneePicker(d) }, func() {
			if len(d.ov.Assignees) == 0 {
				mutedText(c, "None")
			}
			for _, a := range d.ov.Assignees {
				mutedPill(c, a)
			}
		})
		line("Labels", "Edit Labels", func() { p.openLabelPicker(d) }, func() {
			if len(d.pr.Labels) == 0 {
				ui.Text(c, "None").FontSize(fontSmall).TextColor(t.TextMuted)
			}
			for _, name := range d.pr.Labels {
				mutedPill(c, name)
			}
		})
		line("Milestone", "Set Milestone", func() { p.openMilestonePicker(d) }, func() {
			if d.ov.Milestone == nil {
				mutedText(c, "None")
				return
			}
			mutedPill(c, d.ov.Milestone.Title)
		})
		if len(d.ov.Linked) > 0 {
			line("Closes", "", nil, func() {
				for _, issue := range d.ov.Linked {
					ui.Button(c, fmt.Sprintf("#%d %s", issue.Number, issue.Title)).Label(fmt.Sprintf("Open issue #%d", issue.Number)).OnClick(func() { openExternal(c, issue.URL) })
				}
			})
		}
	})
}

func (p *PRs) checksCard(c *ui.Context, d *prData) {
	t := c.Theme()
	plain := make([]github.Check, len(d.checks))
	for i, chk := range d.checks {
		plain[i] = chk.Check
	}
	card(c, "Checks", len(d.checks), func() {
		if len(d.checks) == 0 {
			mutedText(c, "No checks reported for this commit.")
			return
		}
		mutedText(c, fmt.Sprintf("%d of %d checks passed", countChecks(plain), len(d.checks)))
		for i, chk := range d.checks {
			ui.Row(c.Key(i)).Gap(checkRowGap).AlignItems(ui.Center).FillWidth().Children(func() {
				checkIcon(c, checkOutcome(chk.Check))
				ui.Text(c, chk.Name).FontSize(fontBody).SingleLine().Grow(1).MinWidth(0)
				if chk.Required {
					mutedPill(c, "Required")
				}
				if chk.Conclusion != "" {
					mutedText(c, chk.Conclusion).Shrink(0)
				} else {
					mutedText(c, chk.Status).Shrink(0)
				}
				p.checkButtons(c, d, chk, t)
			})
		}
	})
}

func (p *PRs) checkButtons(c *ui.Context, d *prData, chk checkRow, t *ui.Theme) {
	if chk.URL != "" {
		ui.Button(c, "Details").Label("Details for " + chk.Name).TextColor(t.Accent).OnClick(func() { openExternal(c, chk.URL) })
	}
	if chk.JobID > 0 && chk.Status != "queued" {
		ui.Button(c, "View Logs").Label("View logs of " + chk.Name).OnClick(func() { p.openLogs(d, chk) })
	}
	if chk.RunID <= 0 {
		return
	}
	switch outcome := checkOutcome(chk.Check); {
	case outcome == checkFailure && chk.Conclusion == "action_required":
		ui.Button(c, "Approve Run").Label("Approve run of " + chk.Name).OnClick(func() { p.runAction(d, chk, actionApprove) })
	case outcome == checkFailure:
		ui.Button(c, "Re-run").Label("Re-run " + chk.Name).OnClick(func() { p.runAction(d, chk, actionRerunFailed) })
	case outcome == checkPending:
		ui.Button(c, "Cancel").Label("Cancel " + chk.Name).OnClick(func() { p.runAction(d, chk, actionCancel) })
	}
}

func (p *PRs) filesCard(c *ui.Context, d *prData) {
	files := d.visibleFiles()
	card2 := func() {
		if d.viewedCount() > 0 || d.hideViewed {
			ui.Toggle(c.Key("hide-viewed"), &d.hideViewed, "Hide viewed")
		}
	}
	cardWith(c, "Changed files", len(d.files), card2, func() {
		if len(d.files) == 0 {
			mutedText(c, "No changed files.")
			return
		}
		mutedText(c, fmt.Sprintf("+%d -%d across %d files, %d viewed. Select a file to open its diff.", d.pr.Additions, d.pr.Deletions, len(d.files), d.viewedCount()))
		p.fileList(c, d, &d.fileList, &d.fileRow, files, modePR, "Changed files")
	})
}

func (p *PRs) sinceCard(c *ui.Context, d *prData) {
	cardWith(c, "Changes since your last review", len(d.since), nil, func() {
		mutedText(c, "Compared with commit "+shortSHA(d.sinceSHA)+", the one you last reviewed.")
		p.fileList(c, d, &d.sinceList, &d.sinceRow, d.since, modeSince, "Changes since last review")
	})
}

func (p *PRs) fileList(c *ui.Context, d *prData, list *ui.ListState, row *int, files []github.PRFile, mode, label string) {
	t := c.Theme()
	if len(files) == 0 {
		mutedText(c, "Every file is marked as viewed.")
		return
	}
	list.Key = func(i int) any { return files[i].Path }
	list.Label = func(i int) string { return files[i].Path }
	rows := min(len(files), fileListMaxRows)
	spec := diffSpec{mode: mode, oldRef: d.sinceSHA}
	filesEl := ui.List(c, list, len(files), func(i int) {
		f := files[i]
		line := ui.Row(c).Gap(spacingMedium).AlignItems(ui.Center).Padding(0, spacingMedium).Height(fileListRowHeight).FillWidth()
		line.Children(func() {
			if mode == modePR && d.pr.NodeID != "" {
				viewed := d.viewed[f.Path]
				if ui.Checkbox(c.Key("viewed"), &viewed, "").Label("Viewed " + f.Path).Changed() {
					p.setViewed(d, f.Path, viewed)
				}
			}
			ui.Text(c, fileStatusLetter(f.Status)).FontSize(fontSmall).Bold().Width(statLetterWidth).TextColor(fileStatusColor(t, f.Status)).Shrink(0)
			ui.Text(c, f.Path).FontSize(fontBody).SingleLine().Grow(1).MinWidth(0)
			if n := d.commentCount(f.Path); n > 0 && mode == modePR {
				ui.Icon(c, icons.Comment).Size(iconSmall, iconSmall).TextColor(t.TextMuted).Shrink(0).Label(fmt.Sprintf("%d comments", n))
				mutedText(c, strconv.Itoa(n)).Shrink(0)
			}
			ui.Text(c, "+"+strconv.Itoa(f.Additions)).FontSize(fontSmall).TextColor(t.Success).Shrink(0)
			ui.Text(c, "-"+strconv.Itoa(f.Deletions)).FontSize(fontSmall).TextColor(t.Danger).Shrink(0)
		})
		if line.RowClicked() {
			p.openDiff(d, f, spec)
		}
	}).FillWidth().Height(float32(rows * fileListRowHeight)).Label(label)
	if filesEl.Submitted() && *row >= 0 && *row < len(files) {
		p.openDiff(d, files[*row], spec)
	}
}

func fileStatusColor(t *ui.Theme, status string) ui.Color {
	switch status {
	case statusAdded:
		return t.Success
	case statusRemoved:
		return t.Danger
	}
	return t.Warning
}

func (p *PRs) commitsCard(c *ui.Context, d *prData) {
	card(c, "Commits", len(d.commits), func() {
		if len(d.commits) == 0 {
			mutedText(c, "No commits.")
			return
		}
		now := p.env.Now()
		for i, cm := range d.commits {
			ui.Row(c.Key(i)).Gap(spacingMedium).AlignItems(ui.Center).FillWidth().Children(func() {
				mutedText(c, shortSHA(cm.SHA)).Font("monospace").Shrink(0)
				ui.Text(c, firstLine(cm.Message)).FontSize(fontBody).SingleLine().Grow(1).MinWidth(0)
				mutedText(c, cm.Author).SingleLine().Shrink(0)
				mutedText(c, relativeTime(now, cm.Date)).Shrink(0)
				if cm.URL != "" {
					ui.Button(c, "Open").Label("Open commit " + shortSHA(cm.SHA)).OnClick(func() { openExternal(c, cm.URL) })
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
	card(c, "Conversation", len(d.timeline), func() {
		if len(d.timeline) == 0 {
			mutedText(c, "No activity yet.")
			return
		}
		now := p.env.Now()
		for i, e := range d.timeline {
			if !e.isComment() && !e.isReview() {
				ui.Row(c.Key(i)).Gap(spacingSmall).AlignItems(ui.Center).Wrap().FillWidth().Children(func() {
					ui.Text(c, e.Actor).FontSize(fontSmall).Bold()
					mutedText(c, eventSentence(e))
					mutedText(c, relativeTime(now, e.Time))
				})
				continue
			}
			p.commentView(c, d, i, e, now)
		}
	})
}

func (p *PRs) commentView(c *ui.Context, d *prData, i int, e TimelineEvent, now time.Time) {
	t := c.Theme()
	ui.Column(c.Key(i)).Gap(spacingSmall).Padding(spacingMedium).FillWidth().Radius(cardRadius).Border(1, t.Border).Children(func() {
		ui.Row(c).Gap(spacingSmall).AlignItems(ui.Center).Wrap().FillWidth().Children(func() {
			ui.Text(c, e.Actor).FontSize(fontBody).Bold()
			verb := "commented"
			if e.isReview() {
				verb = reviewStateLabel(e.State)
			}
			mutedText(c, verb)
			mutedText(c, relativeTime(now, e.Time))
			ui.Box(c).Grow(1)
			if e.URL != "" {
				ui.Button(c, "Copy Link").Label("Copy link to comment by " + e.Actor).OnClick(func() { c.WriteClipboard(e.URL) })
			}
			if e.isComment() && e.CanEdit {
				ui.Button(c, "Edit").Label("Edit comment by " + e.Actor).Disabled(d.busy != "").OnClick(func() { p.editComment(d, e) })
			}
			if e.isComment() && e.CanDelete {
				ui.Button(c, "Delete").Label("Delete comment by " + e.Actor).Disabled(d.busy != "").OnClick(func() { p.deleteComment(d, e) })
			}
		})
		if strings.TrimSpace(e.Body) != "" {
			markdown.View(c.Key("body"), e.Body, &d.md)
		}
		if e.isComment() {
			p.reactionBar(c, d, e)
		}
	})
}

func (p *PRs) reactionBar(c *ui.Context, d *prData, e TimelineEvent) {
	t := c.Theme()
	ui.Row(c).Gap(spacingSmall).AlignItems(ui.Center).Wrap().FillWidth().Children(func() {
		for _, r := range e.Reactions {
			if r.Count == 0 {
				continue
			}
			b := ui.Button(c, fmt.Sprintf("%s %d", reactionLabel(r.Content), r.Count)).Label(reactionLabel(r.Content) + " reaction").Disabled(d.busy != "").OnClick(func() { p.react(d, e, r.Content) })
			if r.Viewer {
				b.Background(t.Selection)
			}
		}
		ui.MenuButton(c, "React", func(m *ui.Menu) {
			for _, r := range reactionNames {
				if m.Item(r.label).Chosen() {
					p.react(d, e, r.content)
				}
			}
		}).Label("Add reaction").Disabled(d.busy != "")
	})
}

func (p *PRs) commentBox(c *ui.Context, d *prData) {
	card(c, "Add a comment", 0, func() {
		ui.TextArea(c.Key("comment"), &d.draft).Placeholder("Leave a comment (Markdown is supported)").MinHeight(commentBoxHeight).FillWidth().Label("Comment")
		blank := trimmed(d.draft) == ""
		ui.Row(c).Justify(ui.End).FillWidth().Children(func() {
			ui.PrimaryButton(c, "Add Comment").Disabled(blank || d.busy != "").OnClick(func() { p.postComment(d) })
		})
	})
}
