package prui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/design"
	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/badge"
	"github.com/ZerGo0/editor/internal/design/detail"
	"github.com/ZerGo0/editor/internal/design/feedback"
	"github.com/ZerGo0/editor/internal/design/field"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/ghshared"
	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/github"
	"github.com/egoist/mygo/ui"
)

const (
	createViewTitle  = "Create Pull Request"
	createBodyLines  = 8
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
	if p.Phase != ghshared.PhaseReady {
		p.Host.Notify(p.StateMessage(p.repo.Host).Title)
		return
	}
	if p.branch.Name == "" {
		p.Host.Notify("Check out a branch before creating a pull request.")
		return
	}
	f := p.createForm
	if f == nil || f.branch.Name != p.branch.Name {
		p.createForm = p.newCreateForm()
	}
	p.Host.OpenView(createViewID, createViewTitle, func(c *ui.Context) { p.createView(c) })
}

func (p *PRs) newCreateForm() *createForm {
	f := &createForm{title: p.branch.Subject, milestones: map[string]int{}, autoMergeLabel: autoMergeChoices[0]}
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
	root := p.Host.RootDir()
	applog.Go(func() {
		text := readTemplate(root)
		p.Post(func() {
			if text != "" && !f.typedBody && f.body == "" {
				f.body = text
			}
		})
	})
}

func (p *PRs) targetRemote() string {
	return ghshared.RemoteNameFor(p.remotes, p.repo)
}

func (p *PRs) loadCreateBases(f *createForm) {
	f.gen++
	gen := f.gen
	f.loading, f.err = true, nil
	f.branch = p.branch
	local, branch := p.local, p.branch
	remote := p.targetRemote()
	applog.Go(func() {
		branches, err := local.Branches(p.Ctx)
		var bases []string
		if err == nil {
			bases = remoteBranchNames(branches, remote, branch.HeadRef)
		}
		p.Post(func() {
			if gen != f.gen {
				return
			}
			f.loading, f.err, f.bases = false, err, bases
			f.pushNeed = branch.Upstream == ""
			if len(bases) > 0 && !slices.Contains(bases, f.base) {
				f.base = bases[0]
			}
		})
	})
}

func (p *PRs) loadCompare(f *createForm) {
	f.cmpGen++
	gen := f.cmpGen
	base := f.base
	f.cmpFor, f.cmpLoading, f.cmpErr = base, true, nil
	local, be, repo, branch := p.local, p.be, p.repo, f.branch
	remote := p.targetRemote()
	pushed := branch.Upstream != ""
	applog.Go(func() {
		var subjects []string
		var files []github.PRFile
		var err error
		if cmp, cmpErr := local.CompareBranches(p.Ctx, branch.Name, remote+"/"+base); cmpErr == nil {
			for _, c := range cmp.AOnly {
				subjects = append(subjects, c.Subject)
			}
		} else {
			err = cmpErr
		}
		if err == nil && pushed {
			var c Comparison
			if c, err = be.Compare(p.Ctx, repo, base, headSpec(repo, branch)); err == nil {
				files = c.Files
			}
		}
		p.Post(func() {
			if gen != f.cmpGen {
				return
			}
			f.cmpLoading, f.cmpErr, f.cmpCommits, f.cmpFiles = false, err, subjects, files
		})
	})
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
	p.Act(&f.busy, "Creating pull request", "Cannot create pull request", func(ctx context.Context) (func(), error) {
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
				warnings = append(warnings, fmt.Sprintf("%s: %s", what, ghshared.ErrorText(err)))
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
			p.Host.CloseView(createViewID)
			p.current = currentPR{pr: &pr, loaded: true, gen: p.current.gen + 1}
			p.branch.Upstream = remote + "/" + branch.Name
			p.Host.Notify(fmt.Sprintf("Created #%d", pr.Number))
			p.openPR(pr.Number, &pr)
			if len(warnings) > 0 {
				p.Host.ShowError(fmt.Sprintf("#%d was created, but some settings failed", pr.Number), fmt.Errorf("%s", strings.Join(warnings, "; ")))
			}
		}, nil
	})
}

func (p *PRs) createView(c *ui.Context) {
	if p.Closed {
		return
	}
	f := p.createForm
	detail.Page(c, func() {
		detail.Body(c, createViewTitle, token.FormMaxWidth, func() {
			design.Text(c, createViewTitle, token.TextHeading)
			if f == nil {
				ghshared.MutedText(c, "Check out a branch, then open this view again.")
				return
			}
			p.createFields(c, f)
		})
	})
}

func (p *PRs) createFields(c *ui.Context, f *createForm) {
	head := f.branch.Name
	ui.Row(c).FillWidth().AlignItems(ui.Center).Wrap().Gap(token.GapControl.Px()).Children(func() {
		ghshared.MutedText(c, "Compare").Shrink(0)
		branchChip(c, head)
		ghshared.MutedText(c, "into").Shrink(0)
		if f.loading {
			feedback.Spinner(c, "Loading branches")
		} else if len(f.bases) > 0 {
			action.Select(c, "Base branch", &f.base, ghshared.Options(f.bases), action.SelectOptions{ID: "base"})
		}
	})
	if f.err != nil {
		ghshared.ErrorNotice(c, f.err, func() { p.loadCreateBases(f) })
	} else if !f.loading && len(f.bases) == 0 {
		feedback.Notice(c, feedback.NoticeOptions{Tone: token.ToneWarning, Text: "No remote branches found. Fetch the remote, then retry.", Action: ghshared.RetryLabel, OnAction: func() { p.loadCreateBases(f) }})
	}
	detail.Field(c, "Title", func() {
		field.TextInput(c, "Title", &f.title, field.InputOptions{Placeholder: "Pull request title", ID: "title"})
	})
	detail.Field(c, "Description", func() {
		if field.TextArea(c, "Description", &f.body, field.AreaOptions{Placeholder: "Describe the changes (Markdown is supported)", MinLines: createBodyLines, ID: "body"}).Changed() {
			f.typedBody = true
		}
	})
	p.createMetadata(c, f)
	p.createCompare(c, f)
	if f.pushNeed {
		ghshared.MutedText(c, "This branch has no upstream. It will be pushed to "+p.targetRemote()+" first.")
	}
	blocked := f.blocked()
	ui.Row(c).FillWidth().AlignItems(ui.Center).Justify(ui.End).Gap(token.GapControl.Px()).Children(func() {
		ghshared.CreateFooter(c, f.busy, blocked).OnClick(func() { p.submitCreate(f) })
	})
}

func (p *PRs) createMetadata(c *ui.Context, f *createForm) {
	line := func(title, edit string, onClick func(), values []string) {
		detail.Property(c, title, func(c *ui.Context) {
			if len(values) == 0 {
				ghshared.MutedText(c, "None")
			}
			for _, v := range values {
				mutedTag(c, v)
			}
		}, func(c *ui.Context) { ghshared.SmallButton(c, edit, false).OnClick(onClick) })
	}
	line("Reviewers", "Choose Reviewers", func() { p.pickFormUsers(f, "Reviewers", &f.reviewers) }, f.reviewers)
	line("Assignees", "Choose Assignees", func() { p.pickFormUsers(f, "Assignees", &f.assignees) }, f.assignees)
	line("Labels", "Choose Labels", func() { p.pickFormLabels(f) }, f.labels)
	var milestone []string
	if f.milestone != "" {
		milestone = []string{f.milestone}
	}
	line("Milestone", "Choose Milestone", func() { p.pickFormMilestone(f) }, milestone)
	ui.Row(c).FillWidth().AlignItems(ui.Center).Wrap().Gap(token.GapSection.Px()).Children(func() {
		action.Switch(c, "Create as draft", &f.draft, action.ChoiceOptions{ID: "draft"})
		action.Checkbox(c, "Assign to me", &f.assignMe, action.ChoiceOptions{ID: "assign-me"})
	})
	detail.Property(c, "Auto-merge", func(c *ui.Context) {
		action.Select(c, "Auto-merge", &f.autoMergeLabel, ghshared.Options(autoMergeChoices), action.SelectOptions{ID: "auto-merge"})
	}, nil)
}

func (p *PRs) pickFormUsers(f *createForm, title string, target *[]string) {
	p.openPicker(title, "Choose", false, *target, p.loadUsers, func(_ context.Context, selected []string) (func(), error) {
		return func() { *target = selected }, nil
	})
}

func (p *PRs) pickFormLabels(f *createForm) {
	repo := p.repo
	p.openPicker("Labels", "Choose", false, f.labels, p.loadLabelNames(repo), func(_ context.Context, selected []string) (func(), error) {
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
	p.openPicker("Milestone", "Choose", true, current, p.loadMilestoneTitles(repo, numbers), func(_ context.Context, selected []string) (func(), error) {
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
	detail.Card(c, "Changes", detail.CardOptions{Count: len(f.cmpFiles)}, func() {
		switch {
		case f.base == "":
			ghshared.MutedText(c, "Choose a base branch to see the changes.")
			return
		case f.cmpLoading:
			compareSkeleton(c)
			return
		case f.cmpErr != nil:
			ghshared.ErrorNotice(c, f.cmpErr, func() { p.loadCompare(f) })
			return
		}
		ghshared.MutedText(c, fmt.Sprintf("%d commits ahead of %s", len(f.cmpCommits), f.base))
		for i, s := range f.cmpCommits[:min(len(f.cmpCommits), compareListRows)] {
			design.Text(c.Key(i), s, token.TextBody).SingleLine()
		}
		if f.pushNeed {
			ghshared.MutedText(c, "Push the branch to list the changed files.")
			return
		}
		for i, file := range f.cmpFiles[:min(len(f.cmpFiles), compareListRows)] {
			compareFileRow(c.Key("f"+fmt.Sprint(i)), file)
		}
		if len(f.cmpFiles) > compareListRows {
			ghshared.MutedText(c, fmt.Sprintf("and %d more files", len(f.cmpFiles)-compareListRows))
		}
	})
}

func compareFileRow(c *ui.Context, file github.PRFile) {
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(token.GapControl.Px()).Children(func() {
		badge.Decoration(c, fileGitStatus(file.Status))
		design.Text(c, file.Path, token.TextBody).SingleLine().Grow(1).MinWidth(0)
		ghshared.MutedText(c, fmt.Sprintf("+%d -%d", file.Additions, file.Deletions)).Shrink(0)
	})
}

const compareSkeletonWords = 4

func compareSkeleton(c *ui.Context) {
	feedback.Skeleton(c, "Comparing", func() {
		ghshared.MutedText(c, feedback.Placeholder(0, compareSkeletonWords))
		for i := range compareListRows {
			design.Text(c.Key(i), feedback.Placeholder(i, compareSkeletonWords), token.TextBody).SingleLine()
		}
		for i := range compareListRows {
			compareFileRow(c.Key("f"+fmt.Sprint(i)), github.PRFile{Path: feedback.Placeholder(i, 1) + "/" + feedback.Placeholder(i+1, 1), Additions: i + 1, Deletions: i})
		}
	})
}

func autoMergeMethodFor(label string) string {
	if i := slices.Index(autoMergeChoices, label); i > 0 {
		return autoMergeMethods[i]
	}
	return ""
}
