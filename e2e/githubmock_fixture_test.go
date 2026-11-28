//go:build e2e

package e2e

import (
	"crypto/sha1"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	mockOwner            = "acme"
	mockDemoRepo         = "demo"
	mockStressRepo       = "stress"
	mockStressIssuesRepo = "stress-issues"
	mockStressIssues     = 1000
	mockStressNotes      = 500
	mockViewer           = "ada"
	mockToken            = "e2e-token"
	mockBranch           = "main"
	mockDemoPRs          = 40
	mockDemoIssues       = 34
	mockDetailPRIndex    = 3
	mockDetailPR         = mockDemoIssues + mockDetailPRIndex
	mockDetailIssue      = 1
	mockStressPRs        = 1000
	mockStressFiles      = 300
	mockStressBigPR      = 1
	mockStressAuthors    = 7
	mockCommentIDBase    = 5000
	mockReviewIDBase     = 7000
	mockCheckIDBase      = 9000
)

var mockEpoch = time.Date(2026, time.October, 1, 12, 0, 0, 0, time.UTC)

func mockAt(hoursAgo int) time.Time { return mockEpoch.Add(-time.Duration(hoursAgo) * time.Hour) }

func mockSHA(n int) string { return fmt.Sprintf("%x", sha1.Sum([]byte(strconv.Itoa(n)))) }

type mockLabel struct{ Name, Color, Description string }

type mockMilestone struct {
	Number       int
	Title        string
	Description  string
	Open, Closed int
	DueDays      int
}

type mockComment struct {
	ID     int64
	Author string
	Body   string
	Hours  int
}

type mockIssue struct {
	Number    int
	Title     string
	Body      string
	State     string
	Author    string
	Labels    []mockLabel
	Assignees []string
	Milestone int
	Comments  []mockComment
	Hours     int
}

type mockFile struct {
	Path, Prev, Status string
	Additions          int
	Deletions          int
	Patch              string
}

type mockReview struct {
	ID     int64
	Author string
	State  string
	Body   string
	Hours  int
}

type mockThread struct {
	ID       string
	Path     string
	Line     int
	Resolved bool
	Outdated bool
	Comments []mockComment
}

type mockCheck struct {
	Name       string
	Status     string
	Conclusion string
	Required   bool
}

type mockStatus struct{ Context, State string }

type mockCommit struct {
	Message string
	Author  string
	Hours   int
}

type mockPR struct {
	Number    int
	Title     string
	Body      string
	State     string
	Author    string
	Draft     bool
	Labels    []string
	Reviewers []string
	Files     []mockFile
	Reviews   []mockReview
	Comments  []mockComment
	Threads   []mockThread
	Checks    []mockCheck
	Statuses  []mockStatus
	Commits   []mockCommit
	Hours     int
}

type mockNotification struct {
	ID     string
	Number int
	Title  string
	Reason string
	Unread bool
	Hours  int
}

type mockRepo struct {
	Owner         string
	Name          string
	Labels        []mockLabel
	Milestones    []mockMilestone
	Users         []string
	Issues        []mockIssue
	PRs           []mockPR
	Notifications []mockNotification
}

func (r *mockRepo) key() string { return r.Owner + "/" + r.Name }

func (r *mockRepo) pr(number int) (*mockPR, bool) {
	i := number - 1 - len(r.Issues)
	if i < 0 || i >= len(r.PRs) {
		return nil, false
	}
	return &r.PRs[i], true
}

func (r *mockRepo) issue(number int) (*mockIssue, bool) {
	if number < 1 || number > len(r.Issues) {
		return nil, false
	}
	return &r.Issues[number-1], true
}

var mockLabelNames = []string{
	"bug", "enhancement", "documentation", "good first issue", "help wanted", "performance", "regression", "ui",
	"area/rendering-and-text-layout-pipeline", "area/language-server-protocol-integration", "needs-design-review",
	"blocked-on-upstream-dependency-release", "platform/macos", "platform/windows", "platform/linux", "accessibility",
	"security", "tech-debt", "flaky-test", "breaking-change",
}

var mockUsers = []string{mockViewer, "grace", "linus", "margaret", "dennis", "barbara", "ken"}

var mockTitles = []string{
	"Cursor jumps to the end of the line after undo",
	"Tab strip scrolls to the wrong tab when many tabs are open",
	"Add a command to reveal the active file in the explorer",
	"Search results ignore the exclude pattern for nested folders",
	"Crash when the window is resized below the minimum width",
	"Support workspace trust for task execution",
	"Syntax highlighting is slow for files with very long lines",
}

const mockLongTitle = "The extremely long issue title that keeps going to check how the list rows, the detail header and the tab strip cut text that does not fit into the space that is available for it"

func mockLabels() []mockLabel {
	colors := []string{"d73a4a", "a2eeef", "0075ca", "7057ff", "008672", "e4e669", "b60205", "fbca04"}
	labels := make([]mockLabel, len(mockLabelNames))
	for i, name := range mockLabelNames {
		labels[i] = mockLabel{Name: name, Color: colors[i%len(colors)], Description: "Used for " + name}
	}
	return labels
}

func mockMilestones() []mockMilestone {
	return []mockMilestone{
		{1, "v1.0 Release Candidate with a milestone title that is far too long for a side bar row", "The first stable release", 12, 30, 20},
		{2, "v1.1", "Polish and performance", 5, 2, 60},
		{3, "Backlog", "", 40, 0, 0},
	}
}

func mockIssueList(labels []mockLabel, count int) []mockIssue {
	issues := make([]mockIssue, count)
	for i := range issues {
		n := i + 1
		title := fmt.Sprintf("%s (%d)", mockTitles[i%len(mockTitles)], n)
		if n%5 == 1 {
			title = mockLongTitle
		}
		issue := mockIssue{
			Number: n, Title: title, State: "open", Author: mockUsers[n%len(mockUsers)], Milestone: n % 4, Hours: n * 5,
			Body:      "Steps to reproduce:\n\n1. Open a large project\n2. Trigger the action\n\nExpected a quick response, got a pause.\n\n" + strings.Repeat("The quick brown fox jumps over the lazy dog. ", 12),
			Assignees: []string{mockUsers[n%len(mockUsers)]},
		}
		if n%2 == 1 {
			issue.Assignees = []string{mockViewer, mockUsers[(n+1)%len(mockUsers)], mockUsers[(n+2)%len(mockUsers)], mockUsers[(n+3)%len(mockUsers)]}
		}
		issue.Labels = labels[:n%7*3]
		if n == mockDetailIssue {
			issue.Labels = labels
		}
		if n%9 == 0 {
			issue.State = "closed"
		}
		for c := range n % 4 {
			issue.Comments = append(issue.Comments, mockComment{ID: int64(mockCommentIDBase + n*10 + c), Author: mockUsers[(n+c)%len(mockUsers)], Body: fmt.Sprintf("Comment %d: I can reproduce this on the latest build. %s", c+1, strings.Repeat("More detail follows. ", 8)), Hours: n*5 - c})
		}
		issues[i] = issue
	}
	return issues
}

func mockPatch(i int) string {
	return fmt.Sprintf("@@ -1,5 +1,6 @@\n package part%d\n \n-func old%d() int { return %d }\n+func renamed%d() int { return %d }\n+func added%d() int { return %d }\n // end\n", i, i, i, i, i+1, i, i)
}

func mockFiles(count int) []mockFile {
	files := make([]mockFile, count)
	for i := range files {
		files[i] = mockFile{Path: fmt.Sprintf("internal/module%02d/package%03d/file%03d.go", i%12, i%40, i), Status: "modified", Additions: i%9 + 2, Deletions: i % 4, Patch: mockPatch(i)}
	}
	return files
}

func mockDetailFiles() []mockFile {
	files := mockFiles(8)
	files = append(files,
		mockFile{Path: "internal/render/a-very-long-directory-name-for-the-text-layout-engine/subpackage-with-a-long-name/new_feature_with_a_long_file_name.go", Status: "added", Additions: 40, Patch: "@@ -0,0 +1,3 @@\n+package layout\n+\n+func Layout() {}\n"},
		mockFile{Path: "docs/obsolete.md", Status: "removed", Deletions: 12, Patch: "@@ -1,2 +0,0 @@\n-# Obsolete\n-text\n"},
		mockFile{Path: "internal/app/renamed.go", Prev: "internal/app/original.go", Status: "renamed", Additions: 1, Deletions: 1, Patch: "@@ -1,1 +1,1 @@\n-package old\n+package app\n"},
	)
	return files
}

func mockDetailPRData(n int) mockPR {
	thread := func(id string, line int, resolved bool, comments ...mockComment) mockThread {
		return mockThread{ID: id, Path: "internal/module00/package000/file000.go", Line: line, Resolved: resolved, Comments: comments}
	}
	return mockPR{
		Number: n, Title: "Rework the text layout pipeline to cache shaped runs between frames and avoid reflow", State: "open", Author: "grace",
		Labels: mockLabelNames[:6], Reviewers: []string{mockViewer, "linus"}, Hours: 3, Files: mockDetailFiles(),
		Body: "## Summary\n\nCache shaped runs between frames.\n\n- Fewer allocations\n- Faster scrolling\n\n" + strings.Repeat("This paragraph explains the change in detail so the description wraps over several lines. ", 6),
		Reviews: []mockReview{
			{ID: mockReviewIDBase + 1, Author: "linus", State: "CHANGES_REQUESTED", Body: "Please add a benchmark before this lands.", Hours: 20},
			{ID: mockReviewIDBase + 2, Author: "margaret", State: "APPROVED", Body: "Looks good to me.", Hours: 10},
			{ID: mockReviewIDBase + 3, Author: "dennis", State: "COMMENTED", Hours: 8},
		},
		Comments: []mockComment{
			{ID: mockCommentIDBase + 1, Author: "linus", Body: "Can you split the rename into its own change?", Hours: 30},
			{ID: mockCommentIDBase + 2, Author: "grace", Body: "Done, see the latest commit. " + strings.Repeat("The rename is now separate. ", 10), Hours: 12},
		},
		Threads: []mockThread{
			thread("PRT_1", 3, false,
				mockComment{ID: mockCommentIDBase + 11, Author: "linus", Body: "This allocates on every frame. " + strings.Repeat("Can we reuse the buffer? ", 8), Hours: 20},
				mockComment{ID: mockCommentIDBase + 12, Author: "grace", Body: "Good catch, fixed.", Hours: 18}),
			thread("PRT_2", 4, true, mockComment{ID: mockCommentIDBase + 13, Author: "margaret", Body: "Nit: rename this.", Hours: 15}),
		},
		Checks: []mockCheck{
			{"build (macos-latest, go 1.26)", "completed", "success", true},
			{"lint", "completed", "failure", true},
			{"test with a very long job name that does not fit in the checks list of the detail view", "completed", "success", false},
			{"deploy preview", "in_progress", "", false},
		},
		Statuses: []mockStatus{{"ci/legacy-integration", "pending"}, {"coverage/report", "success"}},
		Commits: []mockCommit{
			{"refactor: cache shaped runs", "grace", 40},
			{"test: add layout benchmark", "grace", 30},
			{"fix: reuse the layout buffer " + strings.Repeat("with a long commit subject ", 4), "grace", 20},
		},
	}
}

func mockDemoRepoData() *mockRepo {
	labels := mockLabels()
	issues := mockIssueList(labels, mockDemoIssues)
	prs := make([]mockPR, mockDemoPRs)
	for i := range prs {
		n := i + 1
		prs[i] = mockPR{Number: mockDemoIssues + n, Title: mockTitles[i%len(mockTitles)] + fmt.Sprintf(" #%d", mockDemoIssues+n), State: "open", Author: mockUsers[(n+1)%len(mockUsers)], Hours: n * 3,
			Body: "Fixes the problem described in the issue.", Draft: n%11 == 0, Labels: mockLabelNames[:n%4], Files: mockFiles(n % 5)}
		if n == 12 || n == 7 {
			prs[i].Reviewers = []string{mockViewer}
		}
	}
	prs[mockDetailPRIndex-1] = mockDetailPRData(mockDetailPR)
	prs[mockDetailPRIndex+1].Title = mockLongTitle
	note := func(id string, index int, reason string, unread bool, hours int) mockNotification {
		return mockNotification{ID: id, Number: prs[index-1].Number, Title: prs[index-1].Title, Reason: reason, Unread: unread, Hours: hours}
	}
	notes := []mockNotification{
		note("n1", mockDetailPRIndex, "review_requested", true, 2),
		note("n2", 7, "mention", true, 6),
		note("n3", 12, "author", false, 9),
		note("n4", 21, "ci_activity", true, 12),
		note("n5", 30, "state_change", false, 30),
		note("n6", 5, "team_mention", false, 50),
	}
	return &mockRepo{Owner: mockOwner, Name: mockDemoRepo, Labels: labels, Milestones: mockMilestones(), Users: mockUsers, Issues: issues, PRs: prs, Notifications: notes}
}

func mockStressRepoData() *mockRepo {
	prs := make([]mockPR, mockStressPRs)
	for i := range prs {
		n := i + 1
		prs[i] = mockPR{Number: n, Title: fmt.Sprintf("%s (%d)", mockTitles[i%len(mockTitles)], n), State: "open", Author: mockUsers[n%mockStressAuthors], Hours: n, Body: "Stress pull request.", Labels: mockLabelNames[:n%3]}
	}
	big := &prs[mockStressBigPR-1]
	*big = mockDetailPRData(mockStressBigPR)
	big.Files = mockFiles(mockStressFiles)
	return &mockRepo{Owner: mockOwner, Name: mockStressRepo, Labels: mockLabels(), Users: mockUsers, PRs: prs}
}

func mockStressIssuesRepoData() *mockRepo {
	notes := make([]mockNotification, mockStressNotes)
	for i := range notes {
		n := i + 1
		notes[i] = mockNotification{ID: fmt.Sprintf("sn%d", n), Number: n, Title: fmt.Sprintf("%s (%d)", mockTitles[i%len(mockTitles)], n), Reason: []string{"mention", "review_requested", "author", "ci_activity"}[i%4], Unread: n%3 != 0, Hours: n}
	}
	issues := mockIssueList(mockLabels(), mockStressIssues)
	for i := range issues {
		issues[i].Author, issues[i].Assignees = mockViewer, []string{mockViewer}
	}
	return &mockRepo{Owner: mockOwner, Name: mockStressIssuesRepo, Labels: mockLabels(), Milestones: mockMilestones(), Users: mockUsers, Issues: issues, Notifications: notes}
}

func mockRepos() []*mockRepo {
	return []*mockRepo{mockDemoRepoData(), mockStressRepoData(), mockStressIssuesRepoData()}
}
