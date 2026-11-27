package issueui

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/ZerGo0/editor/internal/github"
)

const issuesSegment = "issues"

var errBadIssueNumber = errors.New("Enter an issue number, such as 42 or #42.")

func trimmed(s string) string {
	return strings.TrimSpace(s)
}

func parseIssueNumber(value string) (int, error) {
	value = trimmed(value)
	if u, err := url.Parse(value); err == nil && u.Host != "" {
		segments := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(segments) < 2 || segments[len(segments)-2] != issuesSegment {
			return 0, errBadIssueNumber
		}
		value = segments[len(segments)-1]
	}
	number, err := strconv.Atoi(strings.TrimPrefix(value, "#"))
	if err != nil || number <= 0 {
		return 0, errBadIssueNumber
	}
	return number, nil
}

func issueURL(repo github.Remote, number int) string {
	return fmt.Sprintf("https://%s/%s/%s/issues/%d", repo.Host, repo.Owner, repo.Name, number)
}

func userURL(repo github.Remote, login string) string {
	return fmt.Sprintf("https://%s/%s", repo.Host, url.PathEscape(login))
}

func detailID(repo github.Remote, number int) string {
	return fmt.Sprintf("issue/%s/%s/%d", repo.Owner, repo.Name, number)
}

func detailTitle(number int, title string) string {
	if title == "" {
		return fmt.Sprintf("Issue #%d", number)
	}
	return fmt.Sprintf("#%d %s", number, title)
}

func isOpen(issue github.Issue) bool {
	return issue.State == github.StateOpen
}

func stateLabel(issue github.Issue) string {
	if isOpen(issue) {
		return "Open"
	}
	return "Closed"
}

func milestoneTitle(issue github.Issue) string {
	if issue.Milestone == nil {
		return ""
	}
	return issue.Milestone.Title
}
