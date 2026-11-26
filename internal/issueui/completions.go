package issueui

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"github.com/ZerGo0/editor/internal/github"
	"github.com/egoist/mygo/ui/editor"
)

func truncate(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit]) + "..."
}

func issueCompletions(issues []github.Issue, text string) []editor.Completion {
	needle := strings.ToLower(text)
	type match struct {
		issue github.Issue
		rank  int
	}
	var matches []match
	seen := map[int]bool{}
	for _, issue := range issues {
		if seen[issue.Number] {
			continue
		}
		seen[issue.Number] = true
		number := strconv.Itoa(issue.Number)
		switch {
		case needle == "" || strings.HasPrefix(number, needle):
			matches = append(matches, match{issue, 0})
		case strings.Contains(strings.ToLower(issue.Title), needle):
			matches = append(matches, match{issue, 1})
		}
	}
	slices.SortStableFunc(matches, func(a, b match) int {
		return cmp.Or(cmp.Compare(a.rank, b.rank), b.issue.Updated.Compare(a.issue.Updated))
	})
	if len(matches) > maxCompletions {
		matches = matches[:maxCompletions]
	}
	out := make([]editor.Completion, 0, len(matches))
	for _, m := range matches {
		ref := "#" + strconv.Itoa(m.issue.Number)
		out = append(out, editor.Completion{
			Label:         ref + " " + m.issue.Title,
			Detail:        stateLabel(m.issue),
			Kind:          editor.CompletionText,
			Insert:        ref,
			FilterText:    ref + " " + m.issue.Title,
			Documentation: truncate(m.issue.Body, completionDocLength),
		})
	}
	return out
}

func userCompletions(users []github.User, text string) []editor.Completion {
	needle := strings.ToLower(text)
	type match struct {
		user github.User
		rank int
	}
	var matches []match
	for _, u := range users {
		login := strings.ToLower(u.Login)
		switch {
		case needle == "" || strings.HasPrefix(login, needle):
			matches = append(matches, match{u, 0})
		case strings.Contains(login, needle) || strings.Contains(strings.ToLower(u.Name), needle):
			matches = append(matches, match{u, 1})
		}
	}
	slices.SortStableFunc(matches, func(a, b match) int { return cmp.Compare(a.rank, b.rank) })
	if len(matches) > maxCompletions {
		matches = matches[:maxCompletions]
	}
	out := make([]editor.Completion, 0, len(matches))
	for _, m := range matches {
		out = append(out, editor.Completion{
			Label:      "@" + m.user.Login,
			Detail:     m.user.Name,
			Kind:       editor.CompletionVariable,
			Insert:     "@" + m.user.Login,
			FilterText: "@" + m.user.Login + " " + m.user.Name,
		})
	}
	return out
}
