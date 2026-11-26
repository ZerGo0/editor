package issueui

import (
	"context"
	"fmt"
	"strings"

	"github.com/ZerGo0/editor/internal/design"
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/detail"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/field"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/github"
	"github.com/egoist/mygo/ui"
)

const (
	createViewID      = "issue/new"
	createViewTitle   = "Create Issue"
	createBodyLines   = 8
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
	detail.Page(c, func() {
		detail.Body(c, createViewTitle, token.FormMaxWidth, func() {
			design.Text(c, createViewTitle, token.TextHeading)
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
	detail.Field(c, "Title", func() {
		field.TextInput(c, "Title", &f.title, field.InputOptions{Placeholder: "Issue title", AutoFocus: true, ID: "title"})
	})
	detail.Field(c, "Description", func() {
		field.TextArea(c, "Description", &f.body, field.AreaOptions{Placeholder: "Describe the issue (Markdown is supported)", MinLines: createBodyLines, ID: "body"})
	})
	if i.meta.loading {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(token.GapControl.Px()).Children(func() {
			feedback.Spinner(c, "Loading labels, people and milestones")
			mutedText(c, "Loading labels, people and milestones...")
		})
	}
	if i.meta.err != nil {
		errorNotice(c, fmt.Errorf("Some choices could not be loaded: %s", errorText(i.meta.err)), i.loadMeta)
	}
	detail.Field(c, "Labels", func() {
		ui.TokenField(c.Key("labels"), &f.labels, i.meta.labelNames()).Placeholder("Add a label").Label("Labels")
	})
	detail.Field(c, "Assignees", func() {
		ui.TokenField(c.Key("assignees"), &f.assignees, i.meta.userLogins()).Placeholder("Add a person").Label("Assignees")
	})
	detail.Field(c, "Milestone", func() {
		action.Select(c, "Milestone", &f.milestone, options(i.meta.milestoneOptions()), action.SelectOptions{ID: "milestone"})
	})
	blocked := f.blocked()
	if f.err != "" {
		feedback.Notice(c, feedback.NoticeOptions{Tone: token.ToneError, Text: f.err})
	}
	ui.Row(c).FillWidth().AlignItems(ui.Center).Justify(ui.End).Gap(token.GapControl.Px()).Children(func() {
		if f.busy != "" {
			feedback.Spinner(c, f.busy)
			mutedText(c, f.busy+"...")
		}
		if blocked != "" {
			mutedText(c, blocked)
		}
		action.Button(c, "Create", action.ButtonOptions{Variant: action.Primary, Disabled: blocked != "" || f.busy != ""}).Label("Create issue").OnClick(func() { i.submitCreate(f) })
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
