package issueui

import (
	"testing"
	"time"

	"github.com/ZerGo0/editor/internal/github"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown/parser"
)

const (
	detailWidth  = 1000
	detailHeight = 1400
	issueNumber  = 5
	issueViewID  = "issue/acme/widgets/5"
)

func viewTester(hs *harness, id string) *ui.Tester {
	return ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() { hs.h.views[id](c) })
	}, detailWidth, detailHeight)
}

func (hs *harness) seedIssue() {
	issue := testIssue(issueNumber, "Crash on start")
	issue.Milestone = &github.Milestone{Number: 1, Title: "v1"}
	issue.Assignees = []string{"bob"}
	issue.Labels = []github.Label{{Name: "bug", Color: "d73a4a"}}
	issue.Comments = 1
	issue.LinkedPRs = []github.LinkedPR{{Number: 9, Title: "Fix the crash", URL: "https://github.com/acme/widgets/pull/9", State: "open"}}
	hs.be.issues[issueNumber] = issue
	hs.be.comments[issueNumber] = []github.Comment{{ID: "c1", Author: "carol", Body: "I can reproduce this", Created: testNow.Add(-2 * time.Hour)}}
}

func (hs *harness) openDetail(t *testing.T) (*issueData, *ui.Tester) {
	t.Helper()
	hs.seedIssue()
	hs.ready(t)
	hs.i.openIssue(issueNumber, nil)
	d := hs.i.details[issueNumber]
	hs.h.pump(t, func() bool { return d.loaded })
	return d, viewTester(hs, issueViewID)
}

func TestOpenIssueUsesDocumentedViewID(t *testing.T) {
	hs := newHarness(t)
	hs.seedIssue()
	hs.ready(t)
	hs.i.openIssue(issueNumber, nil)
	if id := hs.h.opened[0]; id != issueViewID {
		t.Errorf("view id %q", id)
	}
	if title := hs.h.titles[issueViewID]; title != "Issue #5" {
		t.Errorf("title %q", title)
	}
	hs.h.pump(t, func() bool { return hs.i.details[issueNumber].loaded })
	if title := hs.h.titles[issueViewID]; title != "#5 Crash on start" {
		t.Errorf("title after load %q", title)
	}
}

func TestDetailShowsIssue(t *testing.T) {
	hs := newHarness(t)
	d, tt := hs.openDetail(t)
	_ = d
	mustHaveText(t, tt, "Crash on start", "#5", "Open", "alice opened this 2d ago", "Milestone", "v1", "Assignees", "bob", "Labels", "bug",
		"Description", "Body of Crash on start", "Linked pull requests", "#9 Fix the crash", "Conversation", "carol", "commented 2h ago", "I can reproduce this",
		"Add a comment", "Edit", "Close Issue", "Open on GitHub", "Start Working")
	if tt.HasText("Some details could not be loaded:") {
		t.Error("unexpected partial error banner")
	}
}

func TestDetailLoadingAndErrorStates(t *testing.T) {
	hs := newHarness(t)
	hs.seedIssue()
	hs.ready(t)
	gate := make(chan struct{})
	hs.be.with(func() { hs.be.gate["GetIssue"] = gate })
	hs.i.openIssue(issueNumber, nil)
	tt := viewTester(hs, issueViewID)
	mustHaveText(t, tt, "Loading #5...")
	if _, ok := tt.AccessByRole(ui.RoleProgress, "Loading #5..."); !ok {
		t.Error("the spinner has no accessible name")
	}
	hs.be.with(func() { delete(hs.be.gate, "GetIssue") })
	close(gate)
	hs.h.pump(t, func() bool { return hs.i.details[issueNumber].loaded })

	hs2 := newHarness(t)
	hs2.seedIssue()
	hs2.be.errs["GetIssue"] = errBoom
	hs2.ready(t)
	hs2.i.openIssue(issueNumber, nil)
	d := hs2.i.details[issueNumber]
	hs2.h.pump(t, func() bool { return d.err != nil })
	tt2 := viewTester(hs2, issueViewID)
	mustHaveText(t, tt2, "Cannot load #5", "boom", "Retry")
	delete(hs2.be.errs, "GetIssue")
	mustClick(t, tt2, "Retry")
	hs2.h.pump(t, func() bool { return d.loaded })
	tt2.Frame()
	mustHaveText(t, tt2, "Crash on start")
}

func TestDetailPartialFailureKeepsIssueAndOffersRetry(t *testing.T) {
	hs := newHarness(t)
	hs.be.errs["IssueComments"] = errBoom
	_, tt := hs.openDetail(t)
	mustHaveText(t, tt, "Some details could not be loaded:", "comments: boom", "Crash on start")
}

func TestDetailPreloadShowsListDataImmediately(t *testing.T) {
	hs := newHarness(t)
	hs.seedIssue()
	hs.ready(t)
	preload := testIssue(issueNumber, "From the list")
	hs.i.openIssue(issueNumber, &preload)
	if title := hs.h.titles[issueViewID]; title != "#5 From the list" {
		t.Errorf("title %q", title)
	}
	mustHaveText(t, viewTester(hs, issueViewID), "From the list")
}

func TestCloseAndReopen(t *testing.T) {
	hs := newHarness(t)
	d, tt := hs.openDetail(t)
	mustClick(t, tt, "Close Issue")
	hs.h.pump(t, func() bool { return !isOpen(d.issue) && d.busy == "" })
	tt.Frame()
	if len(hs.be.edits) != 1 || hs.be.edits[0].State == nil || *hs.be.edits[0].State != github.StateClosed {
		t.Fatalf("edits %+v", hs.be.edits)
	}
	mustHaveText(t, tt, "Closed", "Reopen Issue")
	if len(d.issue.LinkedPRs) != 1 {
		t.Error("linked pull requests were lost after the edit")
	}
	mustClick(t, tt, "Reopen Issue")
	hs.h.pump(t, func() bool { return isOpen(d.issue) && d.busy == "" })
	if got := hs.h.lastNotice(); got != "Reopened #5" {
		t.Errorf("notice %q", got)
	}
}

func TestCloseFailureShowsError(t *testing.T) {
	hs := newHarness(t)
	d, tt := hs.openDetail(t)
	hs.be.with(func() { hs.be.errs["EditIssue"] = errBoom })
	mustClick(t, tt, "Close Issue")
	hs.h.pump(t, func() bool { return d.busy == "" && hs.h.lastError() != "" })
	if !isOpen(d.issue) {
		t.Error("the issue changed although the edit failed")
	}
}

func TestEditTitleAndBody(t *testing.T) {
	hs := newHarness(t)
	d, tt := hs.openDetail(t)
	mustClick(t, tt, "Edit")
	tt.Frame()
	mustHaveText(t, tt, "Save", "Cancel")
	if d.editTitle != "Crash on start" || d.editBody != "Body of Crash on start" {
		t.Fatalf("edit fields %q %q", d.editTitle, d.editBody)
	}
	d.editTitle, d.editBody = "  New title ", "New body"
	tt.Frame()
	mustClick(t, tt, "Save")
	hs.h.pump(t, func() bool { return !d.editing })
	if len(hs.be.edits) != 1 || *hs.be.edits[0].Title != "New title" || *hs.be.edits[0].Body != "New body" {
		t.Fatalf("edits %+v", hs.be.edits)
	}
	if hs.h.titles[issueViewID] != "#5 New title" {
		t.Errorf("tab title %q", hs.h.titles[issueViewID])
	}
	tt.Frame()
	mustHaveText(t, tt, "New title", "New body")
}

func TestEditValidationAndCancel(t *testing.T) {
	hs := newHarness(t)
	d, tt := hs.openDetail(t)
	mustClick(t, tt, "Edit")
	d.editTitle = " "
	tt.Frame()
	mustClick(t, tt, "Save")
	tt.Frame()
	mustHaveText(t, tt, "Enter a title.")
	if len(hs.be.edits) != 0 {
		t.Error("an invalid edit was sent")
	}
	d.editTitle = d.issue.Title
	d.editBody = d.issue.Body
	tt.Frame()
	mustClick(t, tt, "Save")
	if d.editing || len(hs.be.edits) != 0 {
		t.Errorf("an unchanged edit must close without a request: editing %v edits %v", d.editing, hs.be.edits)
	}
	mustClick(t, tt, "Edit")
	mustClick(t, tt, "Cancel")
	if d.editing {
		t.Error("cancel kept the edit form")
	}
}

func TestPostComment(t *testing.T) {
	hs := newHarness(t)
	d, tt := hs.openDetail(t)
	if a, ok := tt.AccessByRole(ui.RoleButton, "Add Comment"); !ok || !a.Disabled {
		t.Errorf("empty comment must disable the button: %+v %v", a, ok)
	}
	d.draft = "  Thanks!  "
	tt.Frame()
	mustClick(t, tt, "Add Comment")
	hs.h.pump(t, func() bool { return len(d.comments) == 2 })
	if hs.be.commented != "Thanks!" || d.draft != "" || d.issue.Comments != 2 {
		t.Errorf("commented %q draft %q count %d", hs.be.commented, d.draft, d.issue.Comments)
	}
	tt.Frame()
	mustHaveText(t, tt, "Thanks!")
}

func TestPostCommentFailureKeepsDraft(t *testing.T) {
	hs := newHarness(t)
	d, tt := hs.openDetail(t)
	hs.be.with(func() { hs.be.errs["AddIssueComment"] = errBoom })
	d.draft = "Hello"
	tt.Frame()
	mustClick(t, tt, "Add Comment")
	hs.h.pump(t, func() bool { return d.busy == "" && hs.h.lastError() != "" })
	if d.draft != "Hello" || len(d.comments) != 1 {
		t.Errorf("draft %q comments %d", d.draft, len(d.comments))
	}
}

func TestOpenOnGitHubAndLinkedPullRequest(t *testing.T) {
	hs := newHarness(t)
	_, tt := hs.openDetail(t)
	mustClick(t, tt, "Open on GitHub")
	mustClick(t, tt, "Open pull request #9")
	urls := tt.OpenedURLs()
	if len(urls) != 2 || urls[0] != "https://github.com/acme/widgets/issues/5" || urls[1] != "https://github.com/acme/widgets/pull/9" {
		t.Errorf("opened %v", urls)
	}
}

func TestDetailAccessibility(t *testing.T) {
	hs := newHarness(t)
	_, tt := hs.openDetail(t)
	for _, a := range tt.AccessAll() {
		if a.Role == ui.RoleButton && a.Name == "" {
			t.Errorf("a button at %v has no name", a.Bounds)
		}
	}
	for _, name := range []string{"Comment", "Refresh"} {
		if _, ok := tt.Access(name); !ok {
			t.Errorf("no accessible element %q", name)
		}
	}
}

func TestResolveReferences(t *testing.T) {
	hs := newHarness(t)
	hs.ready(t)
	tests := []struct {
		in   parser.Inline
		want string
		ok   bool
	}{
		{parser.Inline{Kind: parser.Mention, Text: "@octocat"}, "https://github.com/octocat", true},
		{parser.Inline{Kind: parser.Mention, Text: "@acme/docs"}, "", false},
		{parser.Inline{Kind: parser.Reference, Text: "#12"}, "https://github.com/acme/widgets/issues/12", true},
		{parser.Inline{Kind: parser.Reference, Text: "other/thing#3"}, "https://github.com/other/thing/issues/3", true},
		{parser.Inline{Kind: parser.Text, Text: "plain"}, "", false},
	}
	for _, tt := range tests {
		got, ok := hs.i.resolveReference(tt.in)
		if got != tt.want || ok != tt.ok {
			t.Errorf("resolve(%q) = %q, %v; want %q, %v", tt.in.Text, got, ok, tt.want, tt.ok)
		}
	}
}

func TestOpenLinkOnlyOpensWebAddresses(t *testing.T) {
	hs := newHarness(t)
	hs.ready(t)
	for _, u := range []string{"javascript:alert(1)", "file:///etc/passwd", "relative/path", "https://"} {
		hs.i.openLink(u)
	}
	if len(hs.urls) != 0 {
		t.Errorf("opened %v", hs.urls)
	}
	hs.i.openLink("https://example.com/x")
	if len(hs.urls) != 1 {
		t.Errorf("opened %v", hs.urls)
	}
}

func TestStaleDetailResultIsIgnored(t *testing.T) {
	hs := newHarness(t)
	hs.seedIssue()
	hs.ready(t)
	d := hs.i.dataFor(issueNumber)
	gate := make(chan struct{})
	hs.be.with(func() { hs.be.gate["GetIssue"] = gate })
	hs.i.loadDetail(d)
	hs.be.with(func() {
		delete(hs.be.gate, "GetIssue")
		newer := hs.be.issues[issueNumber]
		newer.Title = "Newer title"
		hs.be.issues[issueNumber] = newer
	})
	hs.i.loadDetail(d)
	hs.h.pump(t, func() bool { return d.loaded })
	close(gate)
	hs.h.drain()
	if d.issue.Title != "Newer title" {
		t.Errorf("title %q", d.issue.Title)
	}
}
