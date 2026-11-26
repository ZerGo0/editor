package prui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ZerGo0/editor/internal/github"
	"github.com/egoist/mygo/ui"
)

const (
	detailWidth  = 1000
	detailHeight = 1400
	prNumber     = 12
)

func viewTester(hs *harness, id string) *ui.Tester {
	return ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() { hs.h.views[id](c) })
		hs.p.Overlay(c)
	}, detailWidth, detailHeight)
}

func (hs *harness) openDetail(t *testing.T) (*prData, *ui.Tester) {
	t.Helper()
	hs.ready(t)
	hs.p.openPR(prNumber, hs.be.current)
	d := hs.p.details[prNumber]
	hs.h.pump(t, func() bool { return d.loaded })
	tt := viewTester(hs, detailID(hs.p.repo, prNumber))
	return d, tt
}

func (hs *harness) lastError() string {
	if len(hs.h.errs) == 0 {
		return ""
	}
	return hs.h.errs[len(hs.h.errs)-1]
}

func TestOpenPRUsesDocumentedViewID(t *testing.T) {
	hs := newHarness(t)
	hs.ready(t)
	hs.p.openPR(prNumber, hs.be.current)
	if id := hs.h.opened[0]; id != "pr/acme/widgets/12" {
		t.Errorf("view id %q", id)
	}
	if title := hs.h.titles["pr/acme/widgets/12"]; title != "#12 Fix the widget" {
		t.Errorf("title %q", title)
	}
}

func TestDetailShowsPullRequest(t *testing.T) {
	hs := newHarness(t)
	hs.be.reviews = []github.Review{{ID: "r1", Author: "bob", State: reviewApproved, Submitted: testNow.Add(-2 * time.Hour)}}
	_, tt := hs.openDetail(t)
	mustHaveText(t, tt, "Fix the widget", "#12", "Open", "alice wants to merge", "feature", "into", "main", "Updated 3h ago",
		"Description", "bug", "bob approved these changes", "Checks", "build", "1 of 1 checks passed", "Changed files", "src/widget.go",
		"+5", "-2", "Conversation", "carol", "commented", "Looks good to me", "Add a comment")
	if tt.HasText("Some details could not be loaded:") {
		t.Error("unexpected partial error banner")
	}
}

func TestDetailTitleUpdatesWhenOpenedByNumber(t *testing.T) {
	hs := newHarness(t)
	hs.ready(t)
	hs.p.openPR(prNumber, nil)
	if title := hs.h.titles["pr/acme/widgets/12"]; title != "#12" {
		t.Errorf("title %q", title)
	}
	hs.h.pump(t, func() bool { return hs.p.details[prNumber].loaded })
	if title := hs.h.titles["pr/acme/widgets/12"]; title != "#12 Fix the widget" {
		t.Errorf("title after load %q", title)
	}
}

func TestDetailLoadingState(t *testing.T) {
	hs := newHarness(t)
	hs.ready(t)
	hs.p.openPR(prNumber, nil)
	tt := viewTester(hs, detailID(hs.p.repo, prNumber))
	mustHaveText(t, tt, "Loading #12...")
}

func TestDetailErrorOffersRetry(t *testing.T) {
	hs := newHarness(t)
	hs.be.errs["Get"] = errors.New("github: timeout")
	hs.ready(t)
	hs.p.openPR(prNumber, nil)
	d := hs.p.details[prNumber]
	hs.h.pump(t, func() bool { return d.err != nil })
	tt := viewTester(hs, detailID(hs.p.repo, prNumber))
	mustHaveText(t, tt, "Cannot load #12", "timeout", "Retry")
	hs.be.clearErr("Get")
	mustClick(t, tt, "Retry")
	hs.h.pump(t, func() bool { return d.loaded })
	tt.Frame()
	mustHaveText(t, tt, "Fix the widget")
}

func TestDetailPartialFailureKeepsRest(t *testing.T) {
	hs := newHarness(t)
	hs.be.errs["Checks"] = errors.New("github: boom")
	d, tt := hs.openDetail(t)
	if len(d.partErrs) != 1 {
		t.Fatalf("part errors %v", d.partErrs)
	}
	mustHaveText(t, tt, "Some details could not be loaded:", "checks: boom", "src/widget.go", "No checks reported for this commit.")
}

func TestDetailEmptyStates(t *testing.T) {
	hs := newHarness(t)
	hs.be.pr.Body, hs.be.pr.Labels, hs.be.pr.Reviewers = "", nil, nil
	hs.be.checks, hs.be.comments, hs.be.files = nil, nil, nil
	_, tt := hs.openDetail(t)
	mustHaveText(t, tt, "No description provided.", "None requested", "No checks reported for this commit.", "No changed files.", "No activity yet.")
}

func TestDetailAddComment(t *testing.T) {
	hs := newHarness(t)
	d, tt := hs.openDetail(t)
	if a, ok := tt.AccessByRole(ui.RoleButton, "Add Comment"); !ok || !a.Disabled {
		t.Fatalf("Add Comment must start disabled: %+v %v", a, ok)
	}
	d.draft = "  Thanks!  "
	tt.Frame()
	mustClick(t, tt, "Add Comment")
	hs.h.pump(t, func() bool { return d.busy == "" && len(d.timeline) == 2 })
	tt.Frame()
	if hs.be.body != "Thanks!" {
		t.Errorf("posted %q", hs.be.body)
	}
	if d.draft != "" {
		t.Error("draft kept after posting")
	}
	mustHaveText(t, tt, "Thanks!")
}

func TestDetailCommentFailureShowsError(t *testing.T) {
	hs := newHarness(t)
	hs.be.errs["AddComment"] = errors.New("github: nope")
	d, tt := hs.openDetail(t)
	d.draft = "hello"
	tt.Frame()
	mustClick(t, tt, "Add Comment")
	hs.h.pump(t, func() bool { return d.busy == "" && hs.lastError() != "" })
	if !strings.Contains(hs.lastError(), "Cannot post comment") || d.draft != "hello" {
		t.Errorf("error %q draft %q", hs.lastError(), d.draft)
	}
}

func TestDetailCheckout(t *testing.T) {
	hs := newHarness(t)
	other := testPR(15, "Another one")
	hs.be.pr = other
	hs.ready(t)
	hs.p.openPR(15, &other)
	d := hs.p.details[15]
	hs.h.pump(t, func() bool { return d.loaded })
	tt := viewTester(hs, detailID(hs.p.repo, 15))
	mustClick(t, tt, "Checkout")
	hs.h.pump(t, func() bool { return len(hs.h.notices) > 0 })
	hs.mu.Lock()
	defer hs.mu.Unlock()
	if len(hs.co) != 1 || hs.co[0] != 15 {
		t.Errorf("checkout calls %v", hs.co)
	}
}

func TestCheckoutDisabledForCurrentBranch(t *testing.T) {
	hs := newHarness(t)
	_, tt := hs.openDetail(t)
	if a, ok := tt.AccessByRole(ui.RoleButton, "Checked Out"); !ok || !a.Disabled {
		t.Errorf("checked out button: %+v %v", a, ok)
	}
}

func TestDetailOpenOnGitHub(t *testing.T) {
	hs := newHarness(t)
	_, tt := hs.openDetail(t)
	mustClick(t, tt, "Open on GitHub")
	if urls := tt.OpenedURLs(); len(urls) != 1 || urls[0] != "https://github.com/acme/widgets/pull/12" {
		t.Errorf("opened %v", urls)
	}
}

func TestDetailOpensOnlyWebURLs(t *testing.T) {
	hs := newHarness(t)
	hs.be.pr.URL = "javascript:alert(1)"
	_, tt := hs.openDetail(t)
	mustClick(t, tt, "Open on GitHub")
	if urls := tt.OpenedURLs(); len(urls) != 0 {
		t.Errorf("opened %v", urls)
	}
}

func TestDetailCheckLinkOpens(t *testing.T) {
	hs := newHarness(t)
	_, tt := hs.openDetail(t)
	mustClick(t, tt, "Details for build")
	if urls := tt.OpenedURLs(); len(urls) != 1 || urls[0] != "https://ci.example/build" {
		t.Errorf("opened %v", urls)
	}
}

func TestMergeDialogConfirmsAndUsesMethod(t *testing.T) {
	hs := newHarness(t)
	d, tt := hs.openDetail(t)
	mustClick(t, tt, "Merge")
	mustHaveText(t, tt, "Merge #12 into main", "Commit title (optional)", "Delete branch after merge")
	hs.p.dialog.choice = 1
	hs.p.dialog.checks[0].on = true
	hs.setOK(false)
	hs.p.submitDialog()
	hs.h.pump(t, func() bool { return !hs.p.dialog.busy })
	if hs.be.called("Merge") != 0 || !hs.p.dialog.open {
		t.Fatal("merged without confirmation")
	}
	hs.setOK(true)
	hs.p.dialog.body = "Because"
	hs.p.submitDialog()
	hs.h.pump(t, func() bool { return hs.be.called("DeleteBranch") == 1 && !hs.p.dialog.open })
	if hs.be.merged != "12:squash" || hs.be.arg != "acme/widgets:feature" {
		t.Errorf("merged %q arg %q", hs.be.merged, hs.be.arg)
	}
	hs.h.pump(t, func() bool { return !d.loading })
}

func TestMergeDialogSendsCommitText(t *testing.T) {
	hs := newHarness(t)
	_, tt := hs.openDetail(t)
	mustClick(t, tt, "Merge")
	hs.p.dialog.body = "Message body"
	hs.p.submitDialog()
	hs.h.pump(t, func() bool { return hs.be.called("Merge") == 1 })
	hs.be.mu.Lock()
	merge := hs.be.merged
	hs.be.mu.Unlock()
	if merge != "12:merge" {
		t.Errorf("merged %q", merge)
	}
}

func TestMergeBlockedStates(t *testing.T) {
	conflict := false
	cases := []struct {
		name   string
		mutate func(pr *github.PullRequest)
		reason string
	}{
		{"draft", func(pr *github.PullRequest) { pr.Draft = true }, "Mark the pull request ready for review first."},
		{"conflict", func(pr *github.PullRequest) { pr.Mergeable = &conflict }, "This pull request has merge conflicts."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hs := newHarness(t)
			tc.mutate(&hs.be.pr)
			d, tt := hs.openDetail(t)
			if got := d.mergeBlocked(); got != tc.reason {
				t.Errorf("reason %q", got)
			}
			if a, ok := tt.AccessByRole(ui.RoleButton, "Merge"); !ok || !a.Disabled {
				t.Errorf("merge button %+v %v", a, ok)
			}
		})
	}
}

func TestMergedPRHasNoActions(t *testing.T) {
	hs := newHarness(t)
	hs.be.pr.State, hs.be.pr.Merged = github.StateClosed, true
	_, tt := hs.openDetail(t)
	mustHaveText(t, tt, "Merged")
	for _, name := range []string{"Merge", "Approve", "Request Changes"} {
		if _, ok := tt.AccessByRole(ui.RoleButton, name); ok {
			t.Errorf("%q offered on a merged pull request", name)
		}
	}
}

func TestMarkReadyForReview(t *testing.T) {
	hs := newHarness(t)
	hs.be.pr.Draft = true
	d, tt := hs.openDetail(t)
	mustHaveText(t, tt, "Draft")
	mustClick(t, tt, "Mark Ready for Review")
	hs.h.pump(t, func() bool { return hs.be.called("MarkReady") == 1 && d.busy == "" })
	hs.h.pump(t, func() bool { return !d.loading })
}

func TestReviewDialogSubmitsApproval(t *testing.T) {
	hs := newHarness(t)
	d, tt := hs.openDetail(t)
	mustClick(t, tt, "Approve")
	mustHaveText(t, tt, "Submit Review for #12", "Choice")
	mustClick(t, tt, "Submit Review")
	hs.h.pump(t, func() bool { return hs.be.called("SubmitReview") == 1 && !hs.p.dialog.open })
	if hs.be.submitted != "review-1:APPROVE:" {
		t.Errorf("submitted %q", hs.be.submitted)
	}
	hs.h.pump(t, func() bool { return !d.loading })
}

func TestReviewDialogRequiresBodyForChangesRequest(t *testing.T) {
	hs := newHarness(t)
	_, tt := hs.openDetail(t)
	mustClick(t, tt, "Request Changes")
	mustClick(t, tt, "Submit Review")
	mustHaveText(t, tt, "Write a summary for this review.")
	if hs.be.called("SubmitReview") != 0 {
		t.Error("submitted without a body")
	}
	hs.p.dialog.body = "Please fix the tests"
	tt.Frame()
	tt.Advance(time.Second)
	mustClick(t, tt, "Submit Review")
	hs.h.pump(t, func() bool { return hs.be.called("SubmitReview") == 1 })
	if hs.be.submitted != "review-1:REQUEST_CHANGES:Please fix the tests" {
		t.Errorf("submitted %q", hs.be.submitted)
	}
}

func TestPendingReviewIsSubmittedNotRestarted(t *testing.T) {
	hs := newHarness(t)
	hs.be.reviews = []github.Review{{ID: "pending-9", Author: "me", State: reviewPending}}
	d, tt := hs.openDetail(t)
	if d.pendingReview != "pending-9" {
		t.Fatalf("pending %q", d.pendingReview)
	}
	mustClick(t, tt, "Submit Review")
	mustClick(t, tt, "Submit Review")
	hs.h.pump(t, func() bool { return hs.be.called("SubmitReview") == 1 })
	if hs.be.called("StartReview") != 0 || hs.be.submitted != "pending-9:COMMENT:" {
		t.Errorf("start %d submitted %q", hs.be.called("StartReview"), hs.be.submitted)
	}
}

func TestReviewSubmitFailureStaysInDialog(t *testing.T) {
	hs := newHarness(t)
	hs.be.errs["SubmitReview"] = &github.APIError{Status: 422, Message: "Can not approve your own pull request"}
	_, tt := hs.openDetail(t)
	mustClick(t, tt, "Approve")
	mustClick(t, tt, "Submit Review")
	hs.h.pump(t, func() bool { return hs.p.dialog.err != "" && !hs.p.dialog.busy })
	tt.Frame()
	mustHaveText(t, tt, "Can not approve your own pull request")
	if !hs.p.dialog.open {
		t.Error("dialog closed after a failure")
	}
}

func TestDialogCancel(t *testing.T) {
	hs := newHarness(t)
	_, tt := hs.openDetail(t)
	mustClick(t, tt, "Approve")
	mustClick(t, tt, "Cancel")
	if hs.p.dialog.open {
		t.Error("dialog still open")
	}
}

func TestStaleDetailResultIgnored(t *testing.T) {
	hs := newHarness(t)
	hs.ready(t)
	hs.p.openPR(prNumber, nil)
	d := hs.p.details[prNumber]
	hs.p.loadDetail(d)
	gen := d.gen
	hs.h.pump(t, func() bool { return d.loaded })
	hs.h.drain()
	if d.gen != gen || d.loading {
		t.Errorf("gen %d/%d loading %v", d.gen, gen, d.loading)
	}
}

func TestDetailAutoRefreshWhenVisibleAndDue(t *testing.T) {
	hs := newHarness(t)
	d, tt := hs.openDetail(t)
	now := testNow
	hs.p.env.Now = func() time.Time { return now }
	d.loadedAt = now
	tt.Frame()
	base := hs.be.called("Get")
	now = now.Add(autoRefreshInterval + time.Second)
	tt.Frame()
	hs.h.pump(t, func() bool { return hs.be.called("Get") > base })
}

func TestFetchDetailCollectsPartErrors(t *testing.T) {
	hs := newHarness(t)
	hs.be.errs["Files"] = errors.New("a")
	hs.be.errs["Threads"] = errors.New("b")
	res := fetchDetail(t.Context(), hs.be, github.Remote{Owner: "acme", Name: "widgets"}, 12)
	if res.err != nil || len(res.partErrs) != 2 || res.partErrs[0] != "files: a" || res.partErrs[1] != "review threads: b" {
		t.Errorf("%+v", res)
	}
	hs.be.errs["Get"] = errors.New("fatal")
	if res := fetchDetail(t.Context(), hs.be, github.Remote{}, 12); res.err == nil {
		t.Error("Get failure must be fatal")
	}
}
