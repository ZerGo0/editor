package issueui

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/ZerGo0/editor/internal/github"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	maxCompletions      = 50
	completionDocLength = 300
	recentIssuesQuery   = "repo:${owner}/${repository} sort:updated-desc"
	statusGap           = 4
)

type workingRuntime struct {
	busy string
}

type mentionState struct {
	users         []github.User
	usersLoading  bool
	usersLoaded   bool
	issues        []github.Issue
	issuesLoading bool
	issuesLoaded  bool
}

func (i *Issues) plan(prevBase string) workPlan {
	return workPlan{be: i.be, local: i.local, repo: i.repo, remote: remoteNameFor(i.remotes, i.repo), user: i.user, settings: i.settings, prevBase: prevBase}
}

// TECHDEBT: Writes the small state file on the UI thread; move it to a goroutine if the state grows.
func (i *Issues) saveWork() {
	if err := saveWork(i.h.UserDataDir(), i.repo, i.work); err != nil {
		i.h.ShowError("Cannot save the working issue", err)
	}
}

func (i *Issues) startWorkingAction() {
	if !i.requireReady() {
		return
	}
	if issue := i.selectedIssue(); issue != nil {
		i.startWorking(issue.Number, issue)
		return
	}
	i.h.Ask("Start Working on Issue", "Start Working", "", func(value string) error {
		number, err := parseIssueNumber(value)
		if err != nil {
			return err
		}
		i.startWorking(number, nil)
		return nil
	})
}

func (i *Issues) startWorking(number int, preload *github.Issue) {
	if !i.requireReady() {
		return
	}
	if active := i.work.Active; active != nil && active.Number == number {
		i.h.Notify(fmt.Sprintf("You are already working on #%d.", number))
		return
	}
	prev, _ := i.work.previous(number)
	plan := i.plan(prev.Base)
	var known *github.Issue
	if preload != nil {
		copied := *preload
		known = &copied
	}
	i.act(&i.working.busy, fmt.Sprintf("Starting work on #%d", number), fmt.Sprintf("Cannot start working on #%d", number), func(ctx context.Context) (func(), error) {
		issue := known
		if issue == nil {
			fetched, err := plan.be.GetIssue(ctx, plan.repo, number)
			if err != nil {
				return nil, err
			}
			issue = &fetched
		}
		out, err := plan.start(ctx, *issue)
		if err != nil {
			return nil, err
		}
		return func() { i.finishStart(out) }, nil
	})
}

func (i *Issues) finishStart(out startOutcome) {
	i.work.activate(out.item)
	i.saveWork()
	if d := i.details[out.item.Number]; d != nil && d.loaded {
		out.issue.LinkedPRs = d.issue.LinkedPRs
		d.issue = out.issue
	}
	i.markListsStale()
	notice := fmt.Sprintf("Working on #%d in branch %s", out.item.Number, out.item.Branch)
	if len(out.warnings) > 0 {
		notice += ". " + strings.Join(out.warnings, " ")
	}
	i.h.Notify(notice)
}

func (i *Issues) stopWorking() {
	active := i.work.Active
	if active == nil {
		i.h.Notify("You are not working on an issue.")
		return
	}
	item := *active
	local := i.local
	i.working.busy = "Stopping work"
	go func() {
		var err error
		if item.Base != "" {
			err = switchBack(i.ctx, local, item)
		}
		i.post(func() {
			i.working.busy = ""
			i.work.deactivate()
			i.saveWork()
			if err != nil {
				i.h.ShowError("Cannot switch back to "+item.Base, err)
			}
			i.h.Notify(fmt.Sprintf("Stopped working on #%d", item.Number))
		})
	}()
}

func switchBack(ctx context.Context, local Local, item workItem) error {
	status, err := local.Status(ctx)
	if err != nil {
		return err
	}
	if status.Branch != item.Branch {
		return nil
	}
	return local.Checkout(ctx, item.Base, false)
}

func (i *Issues) continueWorkingAction() {
	if !i.requireReady() {
		return
	}
	if active := i.work.Active; active != nil {
		i.h.Notify(fmt.Sprintf("You are already working on #%d.", active.Number))
		return
	}
	if len(i.work.Recent) == 0 {
		i.h.Notify("No earlier issue to continue. Start working on an issue first.")
		return
	}
	i.h.Ask("Continue Working on Issue", "Continue Working", strconv.Itoa(i.work.Recent[0].Number), func(value string) error {
		number, err := parseIssueNumber(value)
		if err != nil {
			return err
		}
		item, ok := i.work.previous(number)
		if !ok {
			return fmt.Errorf("You have not worked on #%d. Use Start Working instead.", number)
		}
		i.continueWorking(item)
		return nil
	})
}

func (i *Issues) continueWorking(item workItem) {
	if !i.requireReady() {
		return
	}
	plan := i.plan(item.Base)
	i.act(&i.working.busy, fmt.Sprintf("Continuing work on #%d", item.Number), fmt.Sprintf("Cannot continue working on #%d", item.Number), func(ctx context.Context) (func(), error) {
		if _, err := plan.ensureBranch(ctx, item.Branch, item.Number, false); err != nil {
			return nil, err
		}
		return func() {
			i.work.activate(item)
			i.saveWork()
			i.h.Notify(fmt.Sprintf("Working on #%d in branch %s", item.Number, item.Branch))
		}, nil
	})
}

func (i *Issues) BranchChanged() {
	if i.closed || !i.ready() || i.work.Active == nil {
		return
	}
	local, active := i.local, *i.work.Active
	go func() {
		status, err := local.Status(i.ctx)
		i.post(func() {
			current := i.work.Active
			if err != nil || current == nil || current.Number != active.Number || status.Branch == current.Branch {
				return
			}
			i.work.deactivate()
			i.saveWork()
		})
	}()
}

func (i *Issues) CommitTemplate() string {
	active := i.work.Active
	if active == nil || !i.ready() {
		return ""
	}
	return commitMessage(i.settings.WorkingIssueFormatSCM, templateVars{User: i.user, Owner: i.repo.Owner, Repo: i.repo.Name, Number: active.Number, Title: active.Title})
}

func (i *Issues) StatusItems(c *ui.Context) {
	active := i.work.Active
	if i.closed || !i.ready() || active == nil {
		return
	}
	t := c.Theme()
	number := active.Number
	label := fmt.Sprintf("Working on issue #%d", number)
	b := ui.ButtonBase(c.Key("issue-status")).Gap(statusGap).Padding(0, spacingSmall).Radius(spacingSmall).AlignItems(ui.Center).
		TextColor(t.TextMuted).Label(label).Tooltip(label + ": " + active.Title)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		ui.Icon(c, icons.GitBranch).Size(iconSmall, iconSmall)
		ui.Text(c, "#"+strconv.Itoa(number)).FontSize(fontBody).SingleLine()
	})
	b.ContextMenu(func(m *ui.Menu) {
		if m.Item("Open Issue").Chosen() {
			i.openIssue(number, nil)
		}
		if m.Item("Stop Working").Disabled(i.working.busy != "").Chosen() {
			i.stopWorking()
		}
	})
	b.OnClick(func() { i.openIssue(number, nil) })
}

func (i *Issues) ensureCompletionData() {
	if !i.mention.usersLoaded && !i.mention.usersLoading {
		i.mention.usersLoading = true
		be, repo := i.be, i.repo
		go func() {
			users, _ := collectUsers(i.ctx, func(ctx context.Context, after string) (github.UserPage, error) {
				return be.MentionableUsers(ctx, repo, after)
			})
			i.post(func() {
				i.mention.users, i.mention.usersLoading, i.mention.usersLoaded = users, false, true
			})
		}()
	}
	if !i.mention.issuesLoaded && !i.mention.issuesLoading {
		i.mention.issuesLoading = true
		be, query := i.be, i.expandQuery(recentIssuesQuery)
		go func() {
			page, _ := be.SearchIssues(i.ctx, query, 1)
			i.post(func() {
				i.mention.issues, i.mention.issuesLoading, i.mention.issuesLoaded = page.Issues, false, true
			})
		}()
	}
}

func (i *Issues) Completions(prefix string) []editor.Completion {
	if i.closed || !i.ready() || prefix == "" {
		return nil
	}
	i.ensureCompletionData()
	switch prefix[0] {
	case '#':
		return issueCompletions(append(i.allKnownIssues(), i.mention.issues...), prefix[1:])
	case '@':
		return userCompletions(i.mention.users, prefix[1:])
	}
	return nil
}
