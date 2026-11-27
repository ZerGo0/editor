package prui

import (
	"context"
	"errors"
	"fmt"

	"github.com/ZerGo0/editor/internal/github"
)

func (p *PRs) openLabelPicker(d *prData) {
	repo, number := p.repo, d.number
	p.openPicker("Labels", "Apply", false, d.pr.Labels, p.loadLabelNames(repo), func(ctx context.Context, selected []string) (func(), error) {
		if selected == nil {
			selected = []string{}
		}
		if _, err := p.be.EditIssue(ctx, repo, number, github.IssueEdit{Labels: &selected}); err != nil {
			return nil, err
		}
		return func() { p.loadDetail(d) }, nil
	})
}

func (p *PRs) openAssigneePicker(d *prData) {
	repo, number := p.repo, d.number
	p.openPicker("Assignees", "Apply", false, d.ov.Assignees, p.loadUsers, func(ctx context.Context, selected []string) (func(), error) {
		if selected == nil {
			selected = []string{}
		}
		if _, err := p.be.EditIssue(ctx, repo, number, github.IssueEdit{Assignees: &selected}); err != nil {
			return nil, err
		}
		return func() { p.loadDetail(d) }, nil
	})
}

func (p *PRs) openReviewerPicker(d *prData) {
	repo, number := p.repo, d.number
	p.openPicker("Request Review", "Request", false, nil, p.loadUsers, func(ctx context.Context, selected []string) (func(), error) {
		if len(selected) == 0 {
			return nil, errors.New("choose at least one person")
		}
		if err := p.be.RequestReviewers(ctx, repo, number, selected, nil); err != nil {
			return nil, err
		}
		return func() { p.loadDetail(d) }, nil
	})
}

func (p *PRs) openMilestonePicker(d *prData) {
	repo, number := p.repo, d.number
	var current []string
	if d.ov.Milestone != nil {
		current = []string{d.ov.Milestone.Title}
	}
	numbers := map[string]int{}
	p.openPicker("Milestone", "Apply", true, current, p.loadMilestoneTitles(repo, numbers), func(ctx context.Context, selected []string) (func(), error) {
		none := 0
		milestone := &none
		if len(selected) > 0 {
			n, ok := numbers[selected[0]]
			if !ok {
				return nil, fmt.Errorf("unknown milestone %q", selected[0])
			}
			milestone = &n
		}
		if _, err := p.be.EditIssue(ctx, repo, number, github.IssueEdit{Milestone: milestone}); err != nil {
			return nil, err
		}
		return func() { p.loadDetail(d) }, nil
	})
}

func (p *PRs) loadUsers(ctx context.Context) ([]string, error) {
	var logins []string
	after := ""
	for range maxUserPages {
		page, err := p.be.AssignableUsers(ctx, p.repo, after)
		if err != nil {
			return logins, err
		}
		for _, u := range page.Users {
			logins = append(logins, u.Login)
		}
		if page.Next == "" {
			break
		}
		after = page.Next
	}
	return logins, nil
}

func (p *PRs) loadLabelNames(repo github.Remote) func(ctx context.Context) ([]string, error) {
	return func(ctx context.Context) ([]string, error) {
		labels, err := p.be.Labels(ctx, repo)
		names := make([]string, 0, len(labels))
		for _, l := range labels {
			names = append(names, l.Name)
		}
		return names, err
	}
}

func (p *PRs) loadMilestoneTitles(repo github.Remote, numbers map[string]int) func(ctx context.Context) ([]string, error) {
	return func(ctx context.Context) ([]string, error) {
		ms, err := p.be.Milestones(ctx, repo)
		titles := make([]string, 0, len(ms))
		for _, m := range ms {
			numbers[m.Title] = m.Number
			titles = append(titles, m.Title)
		}
		return titles, err
	}
}
