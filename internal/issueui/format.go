package issueui

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/github"
)

const (
	justNowLimit  = 45 * time.Second
	hoursPerDay   = 24
	daysPerWeek   = 7
	daysPerYear   = 365
	dotComHost    = "github.com"
	issuesSegment = "issues"
)

var errBadIssueNumber = errors.New("Enter an issue number, such as 42 or #42.")

var preferredRemotes = []string{originRemote, upstreamRemote}

func trimmed(s string) string {
	return strings.TrimSpace(s)
}

func relativeTime(now, t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := now.Sub(t)
	switch {
	case d < justNowLimit:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d/time.Minute))
	case d < hoursPerDay*time.Hour:
		return fmt.Sprintf("%dh ago", int(d/time.Hour))
	}
	days := int(d / (hoursPerDay * time.Hour))
	switch {
	case days < daysPerWeek:
		return fmt.Sprintf("%dd ago", days)
	case days < daysPerYear:
		return fmt.Sprintf("%dw ago", days/daysPerWeek)
	}
	return fmt.Sprintf("%dy ago", days/daysPerYear)
}

func errorText(err error) string {
	var limit *github.RateLimitError
	var api *github.APIError
	switch {
	case errors.As(err, &limit):
		if limit.Reset.IsZero() {
			return "GitHub rate limit reached. Try again later."
		}
		return "GitHub rate limit reached. Try again after " + limit.Reset.Format("15:04") + "."
	case errors.As(err, &api):
		return fmt.Sprintf("GitHub answered %d: %s", api.Status, api.Message)
	}
	return strings.TrimPrefix(err.Error(), "github: ")
}

func isUnauthorized(err error) bool {
	var api *github.APIError
	return errors.As(err, &api) && api.Status == 401
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

func pickRemote(remotes []git.Remote) (github.Remote, map[string]github.Remote, bool) {
	byName := map[string]github.Remote{}
	var first github.Remote
	found := false
	for _, r := range remotes {
		parsed, ok := github.ParseRemote(r.FetchURL)
		if !ok {
			continue
		}
		byName[r.Name] = parsed
		if !found {
			first, found = parsed, true
		}
	}
	for _, name := range preferredRemotes {
		if r, ok := byName[name]; ok {
			return r, byName, true
		}
	}
	return first, byName, found
}

func remoteNameFor(remotes map[string]github.Remote, repo github.Remote) string {
	for _, name := range preferredRemotes {
		if r, ok := remotes[name]; ok && r == repo {
			return name
		}
	}
	for name, r := range remotes {
		if r == repo {
			return name
		}
	}
	return originRemote
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
