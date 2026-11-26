package issueui

import (
	"context"
	"fmt"
	"strings"

	"github.com/ZerGo0/editor/internal/github"
	"github.com/egoist/mygo/ui"
)

const (
	createViewID      = "issue/new"
	createViewTitle   = "Create Issue"
	createFormWidth   = 640
	createBodyHeight  = 180
	noMilestoneOption = "No milestone"
	fenceChar         = "`"
	minFenceLength    = 3
	maxSnippetLines   = 60
)

type createDraft struct {
	title string
	body  string
}

type createForm struct {
	title     string
	body      string
	labels    []string
	assignees []string
	milestone string
	busy      string
	err       string
}

func (i *Issues) openCreate(draft createDraft) {
	if !i.requireReady() {
		return
	}
	if i.create == nil || draft != (createDraft{}) {
		i.create = &createForm{title: draft.title, body: draft.body, milestone: noMilestoneOption}
	}
	i.ensureMeta()
	i.h.OpenView(createViewID, createViewTitle, i.createView)
}

func (f *createForm) blocked() string {
	if trimmed(f.title) == "" {
		return "Enter a title."
	}
	return ""
}

func (i *Issues) submitCreate(f *createForm) {
	if f.blocked() != "" {
		return
	}
	in := github.IssueInput{
		Title:     trimmed(f.title),
		Body:      f.body,
		Labels:    f.labels,
		Assignees: f.assignees,
		Milestone: i.meta.milestoneNumber(f.milestone),
	}
	be, repo := i.be, i.repo
	f.err = ""
	i.act(&f.busy, "Creating issue", "Cannot create the issue", func(ctx context.Context) (func(), error) {
		issue, err := be.CreateIssue(ctx, repo, in)
		if err != nil {
			return nil, err
		}
		return func() {
			if i.create == f {
				i.create = nil
			}
			i.h.CloseView(createViewID)
			i.markListsStale()
			i.h.Notify(fmt.Sprintf("Created #%d", issue.Number))
			i.openIssue(issue.Number, &issue)
		}, nil
	})
}

func (i *Issues) createView(c *ui.Context) {
	if i.closed {
		return
	}
	f := i.create
	t := c.Theme()
	ui.Scroll(c).Fill().Background(t.Background).Children(func() {
		ui.Column(c).Gap(spacingLarge).Padding(messagePad).MaxWidth(createFormWidth).FillWidth().Children(func() {
			ui.Text(c, createViewTitle).FontSize(fontTitle).Bold()
			switch {
			case f == nil:
				mutedText(c, "Use Create Issue in the Issues view to start a new issue.")
			case !i.ready():
				i.stateView(c)
			default:
				i.createFields(c, f)
			}
		})
	})
}

func (i *Issues) createFields(c *ui.Context, f *createForm) {
	t := c.Theme()
	field := func(label string, body func()) {
		ui.Column(c).Gap(spacingSmall).FillWidth().Children(func() {
			mutedText(c, label)
			body()
		})
	}
	field("Title", func() {
		ui.TextInput(c.Key("title"), &f.title).AutoFocus().Placeholder("Issue title").FillWidth().Label("Title")
	})
	field("Description", func() {
		ui.TextArea(c.Key("body"), &f.body).Placeholder("Describe the issue (Markdown is supported)").MinHeight(createBodyHeight).FillWidth().Label("Description")
	})
	if i.meta.loading {
		ui.Row(c).Gap(spacingMedium).AlignItems(ui.Center).Children(func() {
			ui.Spinner(c).Label("Loading labels, people and milestones")
			mutedText(c, "Loading labels, people and milestones...")
		})
	}
	if i.meta.err != nil {
		ui.Row(c).Gap(spacingMedium).AlignItems(ui.Center).Wrap().Children(func() {
			ui.Text(c, "Some choices could not be loaded: "+errorText(i.meta.err)).FontSize(fontSmall).TextColor(t.Danger)
			ui.Button(c, "Retry").OnClick(i.loadMeta)
		})
	}
	field("Labels", func() {
		ui.TokenField(c.Key("labels"), &f.labels, i.meta.labelNames()).Placeholder("Add a label").Label("Labels")
	})
	field("Assignees", func() {
		ui.TokenField(c.Key("assignees"), &f.assignees, i.meta.userLogins()).Placeholder("Add a person").Label("Assignees")
	})
	field("Milestone", func() {
		ui.Select(c.Key("milestone"), &f.milestone, i.meta.milestoneOptions()).Label("Milestone")
	})
	blocked := f.blocked()
	if f.err != "" {
		ui.Text(c, f.err).FontSize(fontSmall).TextColor(t.Danger)
	}
	ui.Row(c).Gap(spacingMedium).AlignItems(ui.Center).Justify(ui.End).FillWidth().Children(func() {
		if f.busy != "" {
			ui.Spinner(c).Label(f.busy)
			mutedText(c, f.busy+"...")
		}
		if blocked != "" {
			mutedText(c, blocked)
		}
		ui.PrimaryButton(c, "Create").Label("Create issue").Disabled(blocked != "" || f.busy != "").OnClick(func() { i.submitCreate(f) })
	})
}

func codeFence(text string) string {
	longest, run := 0, 0
	for _, r := range text {
		if string(r) == fenceChar {
			run++
			longest = max(longest, run)
			continue
		}
		run = 0
	}
	return strings.Repeat(fenceChar, max(minFenceLength, longest+1))
}

func fencedSnippet(text string) string {
	lines := strings.Split(strings.TrimRight(text, "\r\n"), "\n")
	if len(lines) > maxSnippetLines {
		lines = append(lines[:maxSnippetLines], "...")
	}
	body := strings.Join(lines, "\n")
	fence := codeFence(body)
	return fence + "\n" + body + "\n" + fence
}

func locationBody(rel string, line int, snippet string) string {
	body := fmt.Sprintf("Found in `%s:%d`.", rel, line+1)
	if strings.TrimSpace(snippet) != "" {
		body += "\n\n" + fencedSnippet(snippet)
	}
	return body
}

func (i *Issues) createAt(title, path string, startLine, endLine int, snippet string) {
	if !i.requireReady() {
		return
	}
	root := i.h.RootDir()
	go func() {
		link, err := i.Permalink(path, startLine, endLine)
		rel := path
		if r, relErr := repoRelative(root, path); relErr == nil {
			rel = r
		}
		i.post(func() {
			body := link
			if err != nil {
				i.h.Notify(err.Error())
				body = locationBody(rel, startLine, snippet)
			}
			i.openCreate(createDraft{title: title, body: body})
		})
	}()
}

func (i *Issues) CreateFromTODO(path string, line int, text string) {
	i.createAt(trimmed(text), path, line, line, "")
}

func (i *Issues) CreateFromSelection(path string, startLine, endLine int, selected string) {
	i.createAt("", path, startLine, endLine, selected)
}
