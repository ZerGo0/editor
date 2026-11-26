package issueui

import (
	"strconv"
	"strings"

	"github.com/ZerGo0/editor/internal/github"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
)

const (
	commentBoxHeight = 80
	editBodyHeight   = 160
)

func (i *Issues) detailView(c *ui.Context, d *issueData) {
	if i.closed {
		return
	}
	t := c.Theme()
	ui.Column(c).Fill().Background(t.Background).Children(func() {
		switch {
		case d.err != nil && !d.loaded && d.issue.Number == 0:
			ui.Column(c).Grow(1).FillWidth().Center().Gap(spacingMedium).Padding(messagePad).Children(func() {
				ui.Text(c, "Cannot load #"+strconv.Itoa(d.number)).Bold()
				ui.Text(c, errorText(d.err)).FontSize(fontSmall).TextColor(t.Danger)
				ui.Button(c, "Retry").OnClick(func() { i.loadDetail(d) })
			})
		case d.issue.Number == 0:
			ui.Column(c).Grow(1).FillWidth().Center().Children(func() { loadingRow(c, "Loading #"+strconv.Itoa(d.number)+"...") })
		default:
			if d.stale && !d.loading {
				i.loadDetail(d)
			}
			i.detailHeader(c, d)
			ui.Scroll(c).Grow(1).FillWidth().Children(func() {
				ui.Column(c).Gap(spacingLarge).Padding(messagePad).MaxWidth(contentMax).FillWidth().Children(func() { i.detailBody(c, d) })
			})
		}
	})
}

func (i *Issues) detailHeader(c *ui.Context, d *issueData) {
	t := c.Theme()
	issue := d.issue
	ui.Column(c).Gap(spacingMedium).Padding(spacingLarge, messagePad).Shrink(0).FillWidth().Background(t.Surface).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
		ui.Row(c).Gap(spacingMedium).AlignItems(ui.Center).FillWidth().Children(func() {
			if d.editing {
				ui.TextInput(c.Key("edit-title"), &d.editTitle).AutoFocus().FillWidth().Grow(1).MinWidth(0).Label("Title")
			} else {
				ui.Text(c, issue.Title).FontSize(fontTitle).Bold().Grow(1).MinWidth(0)
			}
			ui.Text(c, "#"+strconv.Itoa(issue.Number)).FontSize(fontTitle).TextColor(t.TextMuted).Shrink(0)
			if d.loading {
				ui.Spinner(c).Label("Refreshing")
			}
			if iconButton(c, "refresh", "Refresh", icons.Refresh).Clicked() {
				i.loadDetail(d)
			}
		})
		i.detailMeta(c, d)
		i.detailActions(c, d)
	})
}

func (i *Issues) detailMeta(c *ui.Context, d *issueData) {
	t := c.Theme()
	issue := d.issue
	ui.Row(c).Gap(spacingMedium).AlignItems(ui.Center).Wrap().FillWidth().Children(func() {
		pill(c, stateLabel(issue), stateColor(t, issue))
		author := issue.Author
		if author == "" {
			author = "unknown"
		}
		mutedText(c, author+" opened this "+relativeTime(i.env.Now(), issue.Created)).Shrink(0)
		if issue.Milestone != nil {
			ui.Row(c).Gap(spacingSmall).AlignItems(ui.Center).Background(t.SurfacePressed).Radius(cardRadius).Padding(1, spacingMedium).Shrink(0).Children(func() {
				mutedText(c, "Milestone").Shrink(0)
				ui.Text(c, issue.Milestone.Title).FontSize(fontSmall).SingleLine()
			})
		}
	})
	ui.Row(c).Gap(spacingMedium).AlignItems(ui.Center).Wrap().FillWidth().Children(func() {
		mutedText(c, "Assignees").Shrink(0)
		if len(issue.Assignees) == 0 {
			mutedText(c, "Nobody")
		}
		for _, name := range issue.Assignees {
			mutedPill(c, name)
		}
	})
	ui.Row(c).Gap(spacingMedium).AlignItems(ui.Center).Wrap().FillWidth().Children(func() {
		mutedText(c, "Labels").Shrink(0)
		if len(issue.Labels) == 0 {
			mutedText(c, "None")
		}
		for _, l := range issue.Labels {
			labelPill(c, l)
		}
	})
}

func (i *Issues) detailActions(c *ui.Context, d *issueData) {
	issue := d.issue
	idle := d.busy == ""
	ui.Row(c).Gap(spacingMedium).AlignItems(ui.Center).Wrap().FillWidth().Children(func() {
		if d.editing {
			ui.PrimaryButton(c, "Save").Disabled(!idle).OnClick(func() { i.saveEdit(d) })
			ui.Button(c, "Cancel").Disabled(!idle).OnClick(func() { d.editing = false })
			if d.editErr != "" {
				ui.Text(c, d.editErr).FontSize(fontSmall).TextColor(c.Theme().Danger)
			}
		} else {
			ui.Button(c, "Edit").Disabled(!idle).OnClick(func() { i.startEdit(d) })
			label := "Close Issue"
			if !isOpen(issue) {
				label = "Reopen Issue"
			}
			ui.Button(c, label).Disabled(!idle).OnClick(func() { i.toggleState(d) })
		}
		i.workingButton(c, d)
		ui.Button(c, "Open on GitHub").Disabled(issue.URL == "").OnClick(func() { openExternal(c, issue.URL) })
		if !idle {
			ui.Row(c).Gap(spacingSmall).AlignItems(ui.Center).Children(func() {
				ui.Spinner(c).Label(d.busy)
				mutedText(c, d.busy+"...")
			})
		}
	})
}

func (i *Issues) workingButton(c *ui.Context, d *issueData) {
	idle := i.working.busy == ""
	active := i.work.Active
	switch {
	case active != nil && active.Number == d.number:
		ui.Button(c, "Stop Working").Disabled(!idle).OnClick(i.stopWorking)
	default:
		label := "Start Working"
		if item, ok := i.work.previous(d.number); ok {
			label = "Continue Working"
			ui.Button(c, label).Disabled(!idle).OnClick(func() { i.continueWorking(item) })
			return
		}
		ui.PrimaryButton(c, label).Disabled(!idle).OnClick(func() {
			issue := d.issue
			i.startWorking(d.number, &issue)
		})
	}
}

func (i *Issues) detailBody(c *ui.Context, d *issueData) {
	t := c.Theme()
	if d.partErr != "" {
		ui.Column(c).Gap(spacingSmall).FillWidth().Children(func() {
			ui.Text(c, "Some details could not be loaded:").FontSize(fontSmall).TextColor(t.Warning)
			ui.Text(c, d.partErr).FontSize(fontSmall).TextColor(t.TextMuted)
			ui.Button(c, "Retry").OnClick(func() { i.loadDetail(d) })
		})
	}
	card(c, "Description", 0, func() {
		switch {
		case d.editing:
			ui.TextArea(c.Key("edit-body"), &d.editBody).Placeholder("Describe the issue (Markdown is supported)").MinHeight(editBodyHeight).FillWidth().Label("Description")
		case strings.TrimSpace(d.issue.Body) == "":
			mutedText(c, "No description provided.").Italic()
		default:
			markdown.View(c.Key("description"), d.issue.Body, &d.md)
		}
	})
	i.linkedCard(c, d)
	i.timelineCard(c, d)
	i.commentBox(c, d)
}

func (i *Issues) linkedCard(c *ui.Context, d *issueData) {
	if len(d.issue.LinkedPRs) == 0 {
		return
	}
	t := c.Theme()
	card(c, "Linked pull requests", len(d.issue.LinkedPRs), func() {
		for n, pr := range d.issue.LinkedPRs {
			ui.Row(c.Key(n)).Gap(spacingMedium).AlignItems(ui.Center).FillWidth().Children(func() {
				ui.Icon(c, icons.PullRequest).Size(iconMedium, iconMedium).TextColor(t.TextMuted).Shrink(0)
				ui.Text(c, "#"+strconv.Itoa(pr.Number)+" "+pr.Title).FontSize(fontBody).SingleLine().Grow(1).MinWidth(0)
				mutedPill(c, pr.State)
				ui.Button(c, "Open").Label("Open pull request #" + strconv.Itoa(pr.Number)).Disabled(pr.URL == "").OnClick(func() { openExternal(c, pr.URL) })
			})
		}
	})
}

func (i *Issues) timelineCard(c *ui.Context, d *issueData) {
	card(c, "Conversation", len(d.comments), func() {
		if len(d.comments) == 0 {
			mutedText(c, "No comments yet.")
			return
		}
		t := c.Theme()
		now := i.env.Now()
		for n, cm := range d.comments {
			ui.Column(c.Key(n)).Gap(spacingSmall).Padding(spacingMedium).FillWidth().Radius(cardRadius).Border(1, t.Border).Children(func() {
				ui.Row(c).Gap(spacingSmall).AlignItems(ui.Center).Wrap().Children(func() {
					ui.Text(c, commentAuthor(cm)).FontSize(fontBody).Bold()
					mutedText(c, "commented "+relativeTime(now, cm.Created))
				})
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
	card(c, "Add a comment", 0, func() {
		ui.TextArea(c.Key("comment"), &d.draft).Placeholder("Leave a comment (Markdown is supported)").MinHeight(commentBoxHeight).FillWidth().Label("Comment")
		blank := trimmed(d.draft) == ""
		ui.Row(c).Justify(ui.End).FillWidth().Children(func() {
			ui.PrimaryButton(c, "Add Comment").Disabled(blank || d.busy != "").OnClick(func() { i.postComment(d) })
		})
	})
}
