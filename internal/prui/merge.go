package prui

import (
	"context"
	"fmt"
	"strings"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/github"
)

type mergeChoice struct {
	label  string
	method string
}

func (d *prData) allowedMerges() []mergeChoice {
	if !d.hasOv {
		return mergeChoices
	}
	var out []mergeChoice
	for _, m := range mergeChoices {
		if m.method == github.MergeMethodMerge && d.ov.AllowMerge || m.method == github.MergeMethodSquash && d.ov.AllowSquash || m.method == github.MergeMethodRebase && d.ov.AllowRebase {
			out = append(out, m)
		}
	}
	return out
}

func (p *PRs) updateBranch(d *prData) {
	repo, number, sha := p.repo, d.number, d.pr.HeadSHA
	p.mutate(d, "Updating branch", "Cannot update the branch", func(ctx context.Context, be Backend) error {
		return be.UpdateBranch(ctx, repo, number, sha)
	})
}

func (p *PRs) enqueue(d *prData) {
	id := d.pr.NodeID
	p.confirmThen(fmt.Sprintf("Add #%d to the merge queue?", d.number), "The pull request merges when it reaches the front of the queue.", "Add to Queue", func() {
		p.mutate(d, "Adding to the queue", "Cannot add to the merge queue", func(ctx context.Context, be Backend) error { return be.Enqueue(ctx, id) })
	})
}

func (p *PRs) dequeue(d *prData) {
	entry := d.ov.Queue.EntryID
	p.mutate(d, "Leaving the queue", "Cannot remove from the merge queue", func(ctx context.Context, be Backend) error { return be.Dequeue(ctx, entry) })
}

func (p *PRs) disableAutoMerge(d *prData) {
	id := d.pr.NodeID
	p.mutate(d, "Disabling auto-merge", "Cannot disable auto-merge", func(ctx context.Context, be Backend) error { return be.SetAutoMerge(ctx, id, "", false) })
}

func (p *PRs) openAutoMergeDialog(d *prData) {
	id := d.pr.NodeID
	methods := d.allowedMerges()
	if len(methods) == 0 {
		methods = mergeChoices
	}
	labels := make([]string, len(methods))
	for i, m := range methods {
		labels[i] = m.label
	}
	p.openDialog(dialog{
		title:   fmt.Sprintf("Enable Auto-Merge for #%d", d.number),
		hint:    "The pull request merges when all requirements are met.",
		action:  "Enable Auto-Merge",
		noBody:  true,
		choices: labels,
		choice:  defaultMergeIndex(methods, d.ov.DefaultMethod),
		submit: func(ctx context.Context, in dialogInput) (func(), error) {
			if err := p.be.SetAutoMerge(ctx, id, methods[in.choice].method, true); err != nil {
				return nil, err
			}
			return func() { p.loadDetail(d) }, nil
		},
	})
}

func defaultMergeIndex(methods []mergeChoice, def string) int {
	for i, m := range methods {
		if m.method == def {
			return i
		}
	}
	return 0
}

func (p *PRs) confirmThen(message, detail, action string, then func()) {
	confirm, win := p.env.Confirm, p.Host.Window()
	applog.Go(func() {
		if confirm(win, ConfirmOptions{Message: message, Detail: detail, Action: action}) {
			p.Post(then)
		}
	})
}

func (p *PRs) openMergeDialog(d *prData) {
	methods := d.allowedMerges()
	if len(methods) == 0 {
		p.Host.Notify("This repository allows no merge method.")
		return
	}
	labels := make([]string, len(methods))
	for i, m := range methods {
		labels[i] = m.label
	}
	repo, number, base := p.repo, d.number, d.pr.BaseRef
	headRepo := github.Remote{Host: repo.Host, Owner: d.pr.HeadOwner, Name: d.pr.HeadRepo}
	headRef := d.pr.HeadRef
	canDelete := d.pr.HeadRepo != "" && d.pr.HeadOwner != ""
	var checks []dialogCheck
	if canDelete {
		checks = []dialogCheck{{label: deleteBranchLabel, on: d.ov.DeleteBranchOnMerge}}
	}
	confirm, win := p.env.Confirm, p.Host.Window()
	p.openDialog(dialog{
		title:        fmt.Sprintf("Merge #%d into %s", number, base),
		hint:         "Edit the commit message, or leave the fields empty for GitHub's default.",
		placeholder:  "Commit message (optional)",
		action:       confirmMergeAction,
		subjectLabel: "Commit title (optional)",
		subject:      fmt.Sprintf(mergeSubjectFmt, d.pr.Title, number),
		choices:      labels,
		choice:       defaultMergeIndex(methods, d.ov.DefaultMethod),
		checks:       checks,
		submit: func(ctx context.Context, in dialogInput) (func(), error) {
			choice := methods[in.choice]
			ok := confirm(win, ConfirmOptions{
				Message: fmt.Sprintf("Merge #%d into %s?", number, base),
				Detail:  choice.label + ". This cannot be undone from the editor.",
				Action:  confirmMergeAction,
			})
			if !ok {
				return nil, errCancelled
			}
			if err := p.be.Merge(ctx, repo, number, choice.method, in.subject, in.body); err != nil {
				return nil, err
			}
			var warning error
			if len(in.checks) > 0 && in.checks[0] {
				warning = p.be.DeleteBranch(ctx, headRepo, headRef)
			}
			return func() {
				p.Host.Notify(fmt.Sprintf("Merged #%d", number))
				if warning != nil {
					p.Host.ShowError("Merged, but the branch was not deleted", warning)
				}
				p.loadDetail(d)
				p.refresh()
			}, nil
		},
	})
}

func (p *PRs) deleteBranch(d *prData) {
	headRepo := github.Remote{Host: p.repo.Host, Owner: d.pr.HeadOwner, Name: d.pr.HeadRepo}
	ref := d.pr.HeadRef
	if headRepo.Owner == "" || headRepo.Name == "" {
		p.Host.Notify(headRepoMissingNote)
		return
	}
	p.confirmThen(fmt.Sprintf("Delete the branch %s?", ref), "The branch is removed from "+headRepo.Owner+"/"+headRepo.Name+".", "Delete Branch", func() {
		p.mutate(d, "Deleting branch", "Cannot delete the branch", func(ctx context.Context, be Backend) error { return be.DeleteBranch(ctx, headRepo, ref) })
	})
}

func (d *prData) mergeBlocked() string {
	switch {
	case !d.isOpen():
		return "This pull request is not open."
	case d.pr.Draft:
		return "Mark the pull request ready for review first."
	case d.pr.Mergeable != nil && !*d.pr.Mergeable:
		return "This pull request has merge conflicts."
	case d.hasOv && d.ov.MergeState == mergeStateDirty:
		return "This pull request has merge conflicts."
	}
	return ""
}

func mergeSummary(d *prData) string {
	switch {
	case d.pr.Merged:
		return "Merged"
	case d.pr.State == github.StateClosed:
		return "Closed"
	case !d.hasOv:
		return ""
	case d.ov.Queue.InQueue:
		return fmt.Sprintf("In the merge queue (position %d, %s)", d.ov.Queue.Position, strings.ToLower(d.ov.Queue.State))
	case d.ov.AutoMerge != nil:
		return fmt.Sprintf("Auto-merge enabled (%s) by %s", d.ov.AutoMerge.Method, d.ov.AutoMerge.By)
	case d.pr.Draft:
		return "Draft: mark it ready for review to merge"
	}
	switch d.ov.MergeState {
	case mergeStateDirty:
		return "This branch has conflicts with the base branch"
	case mergeStateBehind:
		return "This branch is out of date with the base branch"
	case mergeStateUnstable:
		return "Some checks are not passing, but the pull request can merge"
	case mergeStateBlocked:
		switch d.ov.ReviewDecision {
		case decisionChanges:
			return "Changes were requested"
		case decisionRequired:
			return "Review required before merging"
		}
		return "Merging is blocked by the repository rules"
	case mergeStateClean:
		return "Ready to merge"
	}
	return ""
}
