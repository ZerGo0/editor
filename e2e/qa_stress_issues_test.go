//go:build e2e

package e2e

import (
	"fmt"
)

const (
	issuesSearchLabel  = "Search issues"
	issuesList         = "Issues"
	notificationsList  = "Notifications"
	stressIssuesScroll = 2000
	stressNotesScroll  = 2000
	stressIssueQuery   = "e"
)

func loadAllIssues(q *QAApp) error {
	return loadAllRows(q, issuesSearchLabel, stressIssueQuery, issuesList, fmt.Sprintf("#%d by", mockStressIssues))
}

func loadNotifications(q *QAApp) error {
	last := fmt.Sprintf("#%d,", mockStressNotes)
	if _, err := q.scrollToRow(notificationsList, rowContaining(last), q.waitFor()); err != nil {
		return fmt.Errorf("the last of %d notifications did not load: %w", mockStressNotes, err)
	}
	return nil
}

func stressIssueActions() []latencyAction {
	issues := fmt.Sprintf("scroll the issue list (%d issues)", mockStressIssues)
	notes := fmt.Sprintf("scroll the notification list (%d notifications)", mockStressNotes)
	return []latencyAction{
		scrollGitHubListAction(issuesPane, issuesList, stressIssuesScroll, loadAllIssues).onGitHubRepo(githubStressIssuesRepo, issues, stressQuietLong, stressSamples),
		scrollGitHubListAction(notificationsPane, notificationsList, stressNotesScroll, loadNotifications).onGitHubRepo(githubStressIssuesRepo, notes, stressQuietLong, stressSamples),
	}
}
