package issueui

import (
	"strings"
	"testing"
)

func TestSanitizeTitle(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Crash on start!", "crash-on-start"},
		{"  Fix: the  [widget]  ", "fix-the-widget"},
		{"Ünïcode títle", "ünïcode-títle"},
		{"///", ""},
		{"a very long title that goes on and on and on and on and on forever", "a-very-long-title-that-goes-on-and-on-and-on-and-o"},
		{"trailing cut at limit    " + strings.Repeat("x", 40), "trailing-cut-at-limit-" + strings.Repeat("x", maxTitleSlugLength-len("trailing-cut-at-limit-"))},
	}
	for _, tt := range tests {
		if got := sanitizeTitle(tt.in); got != tt.want {
			t.Errorf("sanitizeTitle(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestSanitizeBranch(t *testing.T) {
	tests := []struct{ in, want string }{
		{"me/issue5", "me/issue5"},
		{"me/fix bug", "me/fix-bug"},
		{"a..b", "a.b"},
		{"/leading//double/", "leading/double"},
		{"-dash/.hidden/x.lock", "dash/hidden/x"},
		{"a~b^c:d?e*f[g\\h", "a-b-c-d-e-f-g-h"},
		{"x@{y}", "x-y}"},
		{"trail.", "trail"},
		{"@", ""},
		{"", ""},
	}
	for _, tt := range tests {
		if got := sanitizeBranch(tt.in); got != tt.want {
			t.Errorf("sanitizeBranch(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestBranchName(t *testing.T) {
	v := templateVars{User: "me", Owner: "acme", Repo: "widgets", Number: 42, Title: "Crash on Start!"}
	tests := []struct{ tpl, want string }{
		{defaultBranchTitle, "me/issue42"},
		{"${user}/${issueNumber}-${issueTitle}", "me/42-crash-on-start"},
		{"${user}/${sanitizedIssueTitle}", "me/crash-on-start"},
		{"feature/${owner}-${repository}-${issueNumber}", "feature/acme-widgets-42"},
		{"${unknown}/x", "${unknown}/x"},
		{"${issueTitle}", "crash-on-start"},
	}
	for _, tt := range tests {
		if got := branchName(tt.tpl, v); got != tt.want {
			t.Errorf("branchName(%q) = %q, want %q", tt.tpl, got, tt.want)
		}
	}
	empty := templateVars{Number: 7, Title: "!!!"}
	if got := branchName("${issueTitle}", empty); got != "issue-7" {
		t.Errorf("fallback = %q", got)
	}
}

func TestCommitMessage(t *testing.T) {
	v := templateVars{User: "me", Number: 42, Title: "Crash on Start!"}
	if got := commitMessage(defaultCommitTemplate, v); got != "Crash on Start!\nFixes #42" {
		t.Errorf("default = %q", got)
	}
	if got := commitMessage("${issueNumberLabel}: ${sanitizedIssueTitle} by ${user}", v); got != "#42: crash-on-start by me" {
		t.Errorf("custom = %q", got)
	}
}

func TestSettingsNormalized(t *testing.T) {
	got := Settings{Queries: []Query{{Label: "", Query: "x"}, {Label: "L", Query: "q", GroupBy: []string{"milestone"}}}}.normalized()
	if got.IssueBranchTitle != defaultBranchTitle || got.WorkingIssueFormatSCM != defaultCommitTemplate {
		t.Errorf("templates %+v", got)
	}
	if len(got.Queries) != 1 || got.Queries[0].Label != "L" || !got.Queries[0].groupsByMilestone() {
		t.Errorf("queries %+v", got.Queries)
	}
	if def := (Settings{}).normalized(); len(def.Queries) != len(DefaultQueries()) {
		t.Errorf("default queries %+v", def.Queries)
	}
	d := DefaultSettings()
	if !d.AssignWhenWorking || d.LinkBranchOnGitHub || len(d.Queries) != 3 || d.Queries[0].Label != "My Issues" {
		t.Errorf("defaults %+v", d)
	}
}
