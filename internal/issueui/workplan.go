package issueui

import (
	"context"
	"fmt"
	"slices"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/github"
)

type workPlan struct {
	be       Backend
	local    Local
	repo     github.Remote
	remote   string
	user     string
	settings Settings
	prevBase string
}

type startOutcome struct {
	item     workItem
	issue    github.Issue
	warnings []string
}

func (p workPlan) vars(issue github.Issue) templateVars {
	return templateVars{User: p.user, Owner: p.repo.Owner, Repo: p.repo.Name, Number: issue.Number, Title: issue.Title}
}

func (p workPlan) start(ctx context.Context, issue github.Issue) (startOutcome, error) {
	status, err := p.local.Status(ctx)
	if err != nil {
		return startOutcome{}, fmt.Errorf("read Git status: %w", err)
	}
	out := startOutcome{issue: issue}
	if p.settings.AssignWhenWorking && p.user != "" && !slices.Contains(issue.Assignees, p.user) {
		assignees := append(slices.Clone(issue.Assignees), p.user)
		updated, err := p.be.EditIssue(ctx, p.repo, issue.Number, github.IssueEdit{Assignees: &assignees})
		if err != nil {
			out.warnings = append(out.warnings, "Could not assign you: "+errorText(err))
		} else {
			updated.LinkedPRs = issue.LinkedPRs
			out.issue = updated
		}
	}
	name := branchName(p.settings.IssueBranchTitle, p.vars(issue))
	warnings, err := p.ensureBranch(ctx, name, issue.Number, p.settings.LinkBranchOnGitHub)
	out.warnings = append(out.warnings, warnings...)
	if err != nil {
		return out, err
	}
	base := status.Branch
	if base == name {
		base = p.prevBase
	}
	out.item = workItem{Number: issue.Number, Title: issue.Title, URL: issue.URL, Branch: name, Base: base}
	return out, nil
}

func (p workPlan) ensureBranch(ctx context.Context, name string, number int, link bool) ([]string, error) {
	status, err := p.local.Status(ctx)
	if err != nil {
		return nil, fmt.Errorf("read Git status: %w", err)
	}
	if status.Branch == name {
		return nil, nil
	}
	branches, err := p.local.Branches(ctx)
	if err != nil {
		return nil, fmt.Errorf("list branches: %w", err)
	}
	remoteName := p.remote + "/" + name
	localExists := slices.ContainsFunc(branches, func(b git.Branch) bool { return !b.Remote && b.Name == name })
	remoteExists := slices.ContainsFunc(branches, func(b git.Branch) bool { return b.Remote && b.Name == remoteName })
	switch {
	case localExists:
		return nil, wrap("check out "+name, p.local.Checkout(ctx, name, false))
	case remoteExists:
		return nil, wrap("check out "+remoteName, p.local.CheckoutNewTracking(ctx, remoteName))
	case link:
		return p.linkedBranch(ctx, name, remoteName, number, status.OID)
	}
	return nil, wrap("create branch "+name, p.local.CreateBranch(ctx, name, "", true))
}

func (p workPlan) linkedBranch(ctx context.Context, name, remoteName string, number int, head string) ([]string, error) {
	base := ""
	if outgoing, err := p.local.OutgoingCommits(ctx); err == nil && len(outgoing) == 0 {
		base = head
	}
	if _, err := p.be.CreateLinkedBranch(ctx, p.repo, number, name, base); err != nil {
		warning := "Could not create the linked branch: " + errorText(err)
		return []string{warning}, wrap("create branch "+name, p.local.CreateBranch(ctx, name, "", true))
	}
	if err := p.local.Fetch(ctx, p.remote, false, nil); err != nil {
		return nil, wrap("fetch "+p.remote, err)
	}
	return nil, wrap("check out "+remoteName, p.local.CheckoutNewTracking(ctx, remoteName))
}

func wrap(action string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("cannot %s: %w", action, err)
}
