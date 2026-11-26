package prui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/github"
	"github.com/egoist/mygo/ui"
)

const (
	createFormWidth  = 720
	createBodyHeight = 160
	templateMaxBytes = 64 << 10
	compareListRows  = 8
)

var templatePaths = []string{
	".github/pull_request_template.md",
	".github/PULL_REQUEST_TEMPLATE.md",
	"docs/pull_request_template.md",
	"pull_request_template.md",
	"PULL_REQUEST_TEMPLATE.md",
}

var autoMergeChoices = []string{"Do not enable auto-merge", "Auto-merge with a merge commit", "Auto-merge with squash", "Auto-merge with rebase"}

var autoMergeMethods = []string{"", github.MergeMethodMerge, github.MergeMethodSquash, github.MergeMethodRebase}

type createForm struct {
	title          string
	body           string
	base           string
	draft          bool
	assignMe       bool
	autoMergeLabel string
	reviewers      []string
	labels         []string
	assignees      []string
	milestone      string
	bases          []string
	loading        bool
	err            error
	busy           string
	gen            int
	branch         branchInfo
	pushNeed       bool
	typedBody      bool
	cmpFor         string
	cmpLoading     bool
	cmpErr         error
	cmpFiles       []github.PRFile
	cmpCommits     []string
	cmpGen         int
	milestones     map[string]int
}

func (p *PRs) openCreate() {
	if p.phase != phaseReady {
		p.h.Notify(p.stateMessage().title)
		return
	}
	if p.branch.Name == "" {
		p.h.Notify("Check out a branch before creating a pull request.")
		return
	}
	f := p.createForm
	if f == nil || f.branch.Name != p.branch.Name {
		p.createForm = p.newCreateForm()
	}
	p.h.OpenView(createViewID, "Create Pull Request", func(c *ui.Context) { p.createView(c) })
}

func (p *PRs) newCreateForm() *createForm {
	f := &createForm{title: p.branch.Subject, milestones: map[string]int{}}
	p.loadCreateBases(f)
	p.loadTemplate(f)
	return f
}

func (p *PRs) branchUpdated() {
	f := p.createForm
	if f == nil || f.branch.Name == p.branch.Name {
		return
	}
	p.createForm = nil
	if p.branch.Name != "" {
		p.createForm = p.newCreateForm()
	}
}

func readTemplate(root string) string {
	for _, rel := range templatePaths {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err == nil && len(data) <= templateMaxBytes {
			return string(data)
		}
	}
	return ""
}

func (p *PRs) loadTemplate(f *createForm) {
	root := p.h.RootDir()
	go func() {
		text := readTemplate(root)
		p.post(func() {
			if text != "" && !f.typedBody && f.body == "" {
				f.body = text
			}
		})
	}()
}

func (p *PRs) targetRemote() string {
	return remoteNameFor(p.remotes, p.repo)
}

func (p *PRs) loadCreateBases(f *createForm) {
	f.gen++
	gen := f.gen
	f.loading, f.err = true, nil
	f.branch = p.branch
	local, branch := p.local, p.branch
	remote := p.targetRemote()
	go func() {
		branches, err := local.Branches(p.ctx)
		var bases []string
		if err == nil {
			bases = remoteBranchNames(branches, remote, branch.HeadRef)
		}
		p.post(func() {
			if gen != f.gen {
				return
			}
			f.loading, f.err, f.bases = false, err, bases
			f.pushNeed = branch.Upstream == ""
			if len(bases) > 0 && !slices.Contains(bases, f.base) {
				f.base = bases[0]
			}
		})
	}()
}

func (p *PRs) loadCompare(f *createForm) {
	f.cmpGen++
	gen := f.cmpGen
	base := f.base
	f.cmpFor, f.cmpLoading, f.cmpErr = base, true, nil
	local, be, repo, branch := p.local, p.be, p.repo, f.branch
	remote := p.targetRemote()
	pushed := branch.Upstream != ""
	go func() {
		var subjects []string
		var files []github.PRFile
		var err error
		if cmp, cmpErr := local.CompareBranches(p.ctx, branch.Name, remote+"/"+base); cmpErr == nil {
			for _, c := range cmp.AOnly {
				subjects = append(subjects, c.Subject)
			}
		} else {
			err = cmpErr
		}
		if err == nil && pushed {
			var c Comparison
			if c, err = be.Compare(p.ctx, repo, base, headSpec(repo, branch)); err == nil {
				files = c.Files
			}
		}
		p.post(func() {
			if gen != f.cmpGen {
				return
			}
			f.cmpLoading, f.cmpErr, f.cmpCommits, f.cmpFiles = false, err, subjects, files
		})
	}()
}

func (f *createForm) blocked() string {
	switch {
	case f.loading:
		return "Loading branches."
	case trimmed(f.title) == "":
		return "Enter a title."
	case f.base == "":
		return "Choose a base branch."
	case f.base == f.branch.HeadRef:
		return "The base branch must differ from the branch being compared."
	}
	return ""
}

func (p *PRs) submitCreate(f *createForm) {
	if f.blocked() != "" {
		return
	}
	be, local, repo, branch := p.be, p.local, p.repo, f.branch
	remote := branch.Remote
	if remote == "" {
		remote = p.targetRemote()
	}
	opts := github.CreateOptions{Title: trimmed(f.title), Body: f.body, Head: headSpec(repo, branch), Base: f.base, Draft: f.draft}
	push := f.pushNeed
	reviewers, labels, assignees := slices.Clone(f.reviewers), slices.Clone(f.labels), slices.Clone(f.assignees)
	if f.assignMe && p.viewer != "" && !slices.Contains(assignees, p.viewer) {
		assignees = append(assignees, p.viewer)
	}
	milestone := f.milestones[f.milestone]
	autoMethod := autoMergeMethodFor(f.autoMergeLabel)
	p.act(&f.busy, "Creating pull request", "Cannot create pull request", func(ctx context.Context) (func(), error) {
		if push {
			if err := local.Push(ctx, git.PushOptions{Remote: remote, Branch: branch.Name, SetUpstream: true}); err != nil {
				return nil, fmt.Errorf("push %s: %w", branch.Name, err)
			}
		}
		pr, err := be.Create(ctx, repo, opts)
		if err != nil {
			return nil, err
		}
		var warnings []string
		warn := func(what string, err error) {
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("%s: %s", what, errorText(err)))
			}
		}
		if len(reviewers) > 0 {
			warn("reviewers", be.RequestReviewers(ctx, repo, pr.Number, reviewers, nil))
		}
		if len(labels) > 0 || len(assignees) > 0 || milestone > 0 {
			edit := github.IssueEdit{}
			if len(labels) > 0 {
				edit.Labels = &labels
			}
			if len(assignees) > 0 {
				edit.Assignees = &assignees
			}
			if milestone > 0 {
				edit.Milestone = &milestone
			}
			_, err := be.EditIssue(ctx, repo, pr.Number, edit)
			warn("labels, assignees or milestone", err)
		}
		if autoMethod != "" {
			warn("auto-merge", be.SetAutoMerge(ctx, pr.NodeID, autoMethod, true))
		}
		return func() {
			p.createForm = nil
			p.h.CloseView(createViewID)
			p.current = currentPR{pr: &pr, loaded: true, gen: p.current.gen + 1}
			p.branch.Upstream = remote + "/" + branch.Name
			p.h.Notify(fmt.Sprintf("Created #%d", pr.Number))
			p.openPR(pr.Number, &pr)
			if len(warnings) > 0 {
				p.h.ShowError(fmt.Sprintf("#%d was created, but some settings failed", pr.Number), fmt.Errorf("%s", strings.Join(warnings, "; ")))
			}
		}, nil
	})
}

func (p *PRs) createView(c *ui.Context) {
	if p.closed {
		return
	}
	f := p.createForm
	t := c.Theme()
	ui.Scroll(c).Fill().Background(t.Background).Children(func() {
		ui.Column(c).Gap(spacingLarge).Padding(messagePad).MaxWidth(createFormWidth).FillWidth().Children(func() {
			ui.Text(c, "Create Pull Request").FontSize(fontTitle).Bold()
			if f == nil {
				mutedText(c, "Check out a branch, then open this view again.")
				return
			}
			p.createFields(c, f)
		})
	})
}

func (p *PRs) createFields(c *ui.Context, f *createForm) {
	t := c.Theme()
	head := f.branch.Name
	ui.Row(c).Gap(spacingMedium).AlignItems(ui.Center).Wrap().FillWidth().Children(func() {
		mutedText(c, "Compare").Shrink(0)
		branchChip(c, head)
		mutedText(c, "into").Shrink(0)
		if f.loading {
			ui.Spinner(c).Label("Loading branches")
		} else if len(f.bases) > 0 {
			ui.Select(c.Key("base"), &f.base, f.bases).Label("Base branch")
		}
	})
	if f.err != nil {
		ui.Row(c).Gap(spacingMedium).AlignItems(ui.Center).Children(func() {
			ui.Text(c, errorText(f.err)).FontSize(fontSmall).TextColor(t.Danger)
			ui.Button(c, "Retry").OnClick(func() { p.loadCreateBases(f) })
		})
	} else if !f.loading && len(f.bases) == 0 {
		mutedText(c, "No remote branches found. Fetch the remote, then retry.")
		ui.Button(c, "Retry").OnClick(func() { p.loadCreateBases(f) })
	}
	ui.Column(c).Gap(spacingSmall).FillWidth().Children(func() {
		mutedText(c, "Title")
		ui.TextInput(c.Key("title"), &f.title).Placeholder("Pull request title").FillWidth().Label("Title")
	})
	ui.Column(c).Gap(spacingSmall).FillWidth().Children(func() {
		mutedText(c, "Description")
		if ui.TextArea(c.Key("body"), &f.body).Placeholder("Describe the changes (Markdown is supported)").MinHeight(createBodyHeight).FillWidth().Label("Description").Changed() {
			f.typedBody = true
		}
	})
	p.createMetadata(c, f)
	p.createCompare(c, f)
	if f.pushNeed {
		mutedText(c, "This branch has no upstream. It will be pushed to "+p.targetRemote()+" first.")
	}
	blocked := f.blocked()
	ui.Row(c).Gap(spacingMedium).AlignItems(ui.Center).Justify(ui.End).FillWidth().Children(func() {
		if f.busy != "" {
			ui.Spinner(c).Label(f.busy)
			mutedText(c, f.busy+"...")
		}
		if blocked != "" {
			mutedText(c, blocked)
		}
		ui.PrimaryButton(c, "Create").Disabled(blocked != "" || f.busy != "").OnClick(func() { p.submitCreate(f) })
	})
}

func (p *PRs) createMetadata(c *ui.Context, f *createForm) {
	line := func(title, edit string, action func(), values []string) {
		ui.Row(c).Gap(spacingMedium).AlignItems(ui.Center).Wrap().FillWidth().Children(func() {
			mutedText(c, title).Width(statusLabelWidth).Shrink(0)
			if len(values) == 0 {
				mutedText(c, "None")
			}
			for _, v := range values {
				mutedPill(c, v)
			}
			ui.Button(c, edit).OnClick(action)
		})
	}
	line("Reviewers", "Choose Reviewers", func() { p.pickFormUsers(f, "Reviewers", &f.reviewers) }, f.reviewers)
	line("Assignees", "Choose Assignees", func() { p.pickFormUsers(f, "Assignees", &f.assignees) }, f.assignees)
	line("Labels", "Choose Labels", func() { p.pickFormLabels(f) }, f.labels)
	var milestone []string
	if f.milestone != "" {
		milestone = []string{f.milestone}
	}
	line("Milestone", "Choose Milestone", func() { p.pickFormMilestone(f) }, milestone)
	ui.Row(c).Gap(spacingMedium).AlignItems(ui.Center).Wrap().Children(func() {
		ui.Switch(c.Key("draft"), &f.draft).Label("Create as draft")
		ui.Text(c, "Create as draft").FontSize(fontBody)
		ui.Checkbox(c.Key("assign-me"), &f.assignMe, "Assign to me")
		ui.Select(c.Key("auto-merge"), &f.autoMergeLabel, autoMergeChoices).Label("Auto-merge")
	})
}

func (p *PRs) pickFormUsers(f *createForm, title string, target *[]string) {
	p.openPicker(title, "Choose", false, *target, p.loadUsers, func(_ context.Context, selected []string) (func(), error) {
		return func() { *target = selected }, nil
	})
}

func (p *PRs) pickFormLabels(f *createForm) {
	repo := p.repo
	p.openPicker("Labels", "Choose", false, f.labels, func(ctx context.Context) ([]string, error) {
		labels, err := p.be.Labels(ctx, repo)
		names := make([]string, 0, len(labels))
		for _, l := range labels {
			names = append(names, l.Name)
		}
		return names, err
	}, func(_ context.Context, selected []string) (func(), error) {
		return func() { f.labels = selected }, nil
	})
}

func (p *PRs) pickFormMilestone(f *createForm) {
	repo := p.repo
	numbers := map[string]int{}
	var current []string
	if f.milestone != "" {
		current = []string{f.milestone}
	}
	p.openPicker("Milestone", "Choose", true, current, func(ctx context.Context) ([]string, error) {
		ms, err := p.be.Milestones(ctx, repo)
		titles := make([]string, 0, len(ms))
		for _, m := range ms {
			numbers[m.Title] = m.Number
			titles = append(titles, m.Title)
		}
		return titles, err
	}, func(_ context.Context, selected []string) (func(), error) {
		return func() {
			f.milestone = ""
			if len(selected) > 0 {
				f.milestone = selected[0]
				f.milestones[selected[0]] = numbers[selected[0]]
			}
		}, nil
	})
}

func (p *PRs) createCompare(c *ui.Context, f *createForm) {
	if f.base != "" && f.base != f.cmpFor && !f.cmpLoading && f.base != f.branch.HeadRef {
		p.loadCompare(f)
	}
	t := c.Theme()
	card(c, "Changes", len(f.cmpFiles), func() {
		switch {
		case f.base == "":
			mutedText(c, "Choose a base branch to see the changes.")
			return
		case f.cmpLoading:
			loadingRow(c, "Comparing...")
			return
		case f.cmpErr != nil:
			ui.Text(c, errorText(f.cmpErr)).FontSize(fontSmall).TextColor(t.Danger)
			return
		}
		mutedText(c, fmt.Sprintf("%d commits ahead of %s", len(f.cmpCommits), f.base))
		for i, s := range f.cmpCommits[:min(len(f.cmpCommits), compareListRows)] {
			ui.Text(c.Key(i), s).FontSize(fontBody).SingleLine()
		}
		if f.pushNeed {
			mutedText(c, "Push the branch to list the changed files.")
			return
		}
		for i, file := range f.cmpFiles[:min(len(f.cmpFiles), compareListRows)] {
			ui.Row(c.Key("f" + fmt.Sprint(i))).Gap(spacingMedium).AlignItems(ui.Center).FillWidth().Children(func() {
				ui.Text(c, fileStatusLetter(file.Status)).FontSize(fontSmall).Bold().Width(statLetterWidth).TextColor(fileStatusColor(t, file.Status)).Shrink(0)
				ui.Text(c, file.Path).FontSize(fontBody).SingleLine().Grow(1).MinWidth(0)
				ui.Text(c, fmt.Sprintf("+%d -%d", file.Additions, file.Deletions)).FontSize(fontSmall).TextColor(t.TextMuted).Shrink(0)
			})
		}
		if len(f.cmpFiles) > compareListRows {
			mutedText(c, fmt.Sprintf("and %d more files", len(f.cmpFiles)-compareListRows))
		}
	})
}

func autoMergeMethodFor(label string) string {
	if i := slices.Index(autoMergeChoices, label); i > 0 {
		return autoMergeMethods[i]
	}
	return ""
}
