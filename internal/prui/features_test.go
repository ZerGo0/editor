package prui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ZerGo0/editor/internal/ghshared/ghtest"
	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/github"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

func chooseMore(t *testing.T, tt *ui.Tester, item string) {
	t.Helper()
	ghtest.MustClick(t, tt, "More actions")
	if err := tt.ChooseMenuItem(item); err != nil {
		t.Fatalf("menu %q: %v (menu %q)", item, err, tt.Menu())
	}
}

func TestDetailShowsOverviewAndTimelineEvents(t *testing.T) {
	hs := newHarness(t)
	hs.be.ov.Assignees = []string{"a1"}
	hs.be.ov.Milestone = &MilestoneRef{Number: 3, Title: "v1"}
	hs.be.ov.Linked = []LinkedIssue{{Number: 8, Title: "Crash on start", URL: "https://github.com/acme/widgets/issues/8"}}
	hs.be.commits = []github.Commit{{SHA: "abcdef1234", Message: "Fix typo\n\nlong body", Author: "alice", Date: testNow.Add(-time.Hour), URL: "https://github.com/acme/widgets/commit/abcdef1234"}}
	hs.be.timeline = []TimelineEvent{
		{Kind: "LabeledEvent", Actor: "zed", Detail: "bug", Time: testNow.Add(-5 * time.Hour)},
		{Kind: "HeadRefForcePushedEvent", Actor: "alice", Detail: "1111111 to 2222222", Time: testNow.Add(-4 * time.Hour)},
		{Kind: "IssueComment", ID: "IC1", Actor: "carol", Body: "Nice", Time: testNow.Add(-2 * time.Hour), Reactions: []Reaction{{Content: "THUMBS_UP", Count: 2, Viewer: true}}},
	}
	_, tt := hs.openDetail(t)
	ghtest.MustHaveText(t, tt, "a1", "v1", "#8 Crash on start", "Commits", "abcdef1", "Fix typo", "Ready to merge",
		"zed", "added the bug label", "force-pushed the branch (1111111 to 2222222)", "carol", "Nice", "+1 2", "React")
	tt.Advance(time.Second)
	ghtest.MustClick(t, tt, "Open issue #8")
	ghtest.MustClick(t, tt, "Open commit abcdef1")
	if urls := tt.OpenedURLs(); len(urls) != 2 || urls[0] != "https://github.com/acme/widgets/issues/8" {
		t.Errorf("urls %v", urls)
	}
}

func TestDetailEditTitle(t *testing.T) {
	hs := newHarness(t)
	d, tt := hs.openDetail(t)
	ghtest.MustClick(t, tt, "Edit Title")
	ghtest.MustHaveText(t, tt, "Save Title")
	d.titleDraft = "  A better title "
	tt.Frame()
	ghtest.MustClick(t, tt, "Save Title")
	hs.h.Pump(t, func() bool { return hs.be.Count("EditIssue") == 1 && !d.loading && d.busy == "" })
	if hs.be.edit.Title == nil || *hs.be.edit.Title != "A better title" {
		t.Errorf("%+v", hs.be.edit)
	}
	d.editingTitle, d.titleDraft = true, d.pr.Title
	hs.p.saveTitle(d, d.pr.Title)
	if d.editingTitle || hs.be.Count("EditIssue") != 1 {
		t.Error("an unchanged title was sent")
	}
}

func TestDetailEditDescription(t *testing.T) {
	hs := newHarness(t)
	d, tt := hs.openDetail(t)
	ghtest.MustClick(t, tt, "Edit Description")
	if hs.p.dialog.body != "Fixes the **widget**." {
		t.Fatalf("body %q", hs.p.dialog.body)
	}
	hs.p.dialog.body = "New text"
	hs.p.submitDialog()
	hs.pumpCalled(t, "EditIssue", 1, hs.dialogClosed)
	if hs.be.edit.Body == nil || *hs.be.edit.Body != "New text" {
		t.Errorf("%+v", hs.be.edit)
	}
	hs.h.Pump(t, func() bool { return !d.loading })
}

func TestMoreMenuStateChanges(t *testing.T) {
	cases := []struct {
		item   string
		setup  func(hs *harness)
		called string
	}{
		{"Close Pull Request", nil, "EditIssue"},
		{"Convert to Draft", nil, "ConvertToDraft"},
		{"Update Branch", func(hs *harness) { hs.be.ov.MergeState = mergeStateBehind }, "UpdateBranch"},
		{"Disable Auto-Merge", func(hs *harness) {
			hs.be.ov.AutoMerge, hs.be.ov.CanDisableAutoMerge = &AutoMerge{Method: "squash", By: "me"}, true
		}, "SetAutoMerge"},
	}
	for _, tc := range cases {
		t.Run(tc.item, func(t *testing.T) {
			hs := newHarness(t)
			if tc.setup != nil {
				tc.setup(hs)
			}
			d, tt := hs.openDetail(t)
			chooseMore(t, tt, tc.item)
			hs.pumpCalled(t, tc.called, 1, d.idle)
			hs.h.Pump(t, func() bool { return !d.loading })
		})
	}
}

func TestMoreMenuCopiesLinks(t *testing.T) {
	hs := newHarness(t)
	_, tt := hs.openDetail(t)
	chooseMore(t, tt, "Copy Pull Request Link")
	if tt.Clipboard() != "https://github.com/acme/widgets/pull/12" {
		t.Errorf("clipboard %q", tt.Clipboard())
	}
	tt.Advance(time.Second)
	chooseMore(t, tt, "Copy as Markdown Link")
	if tt.Clipboard() != "[#12 Fix the widget](https://github.com/acme/widgets/pull/12)" {
		t.Errorf("clipboard %q", tt.Clipboard())
	}
	tt.Advance(time.Second)
	ghtest.MustClick(t, tt, "Copy Link")
	if tt.Clipboard() != "https://github.com/acme/widgets/pull/12" {
		t.Errorf("clipboard %q", tt.Clipboard())
	}
}

func TestMoreMenuReopenAndMergedActions(t *testing.T) {
	hs := newHarness(t)
	hs.be.pr.State = github.StateClosed
	d, _ := hs.openDetail(t)
	hs.p.setState(d, github.StateOpen)
	hs.pumpCalled(t, "EditIssue", 1, d.idle)
	if hs.be.edit.State == nil || *hs.be.edit.State != "open" {
		t.Errorf("%+v", hs.be.edit)
	}
	hs.h.Pump(t, func() bool { return !d.loading })
	d.pr.Merged = true
	hs.p.revert(d)
	hs.pumpCalled(t, "Revert", 1, d.idle)
	if hs.h.Opened[len(hs.h.Opened)-1] != "pr/acme/widgets/90" {
		t.Errorf("opened %v", hs.h.Opened)
	}
	hs.p.deleteBranch(d)
	hs.pumpCalled(t, "DeleteBranch", 1, d.idle)
	hs.setOK(false)
	hs.p.deleteBranch(d)
	hs.h.Drain()
	if hs.be.Count("DeleteBranch") != 1 {
		t.Error("deleted without a confirmation")
	}
}

func TestAutoMergeDialog(t *testing.T) {
	hs := newHarness(t)
	hs.be.ov.CanEnableAutoMerge = true
	d, tt := hs.openDetail(t)
	chooseMore(t, tt, "Enable Auto-Merge...")
	ghtest.MustHaveText(t, tt, "Enable Auto-Merge for #12", "Squash and merge")
	hs.p.dialog.choice = 1
	hs.p.submitDialog()
	hs.pumpCalled(t, "SetAutoMerge", 1, hs.dialogClosed)
	if hs.be.arg != "squash:true" {
		t.Errorf("%q", hs.be.arg)
	}
	hs.h.Pump(t, func() bool { return !d.loading })
}

func TestMergeQueueButtons(t *testing.T) {
	hs := newHarness(t)
	hs.be.ov.Queue = MergeQueue{Enabled: true}
	d, tt := hs.openDetail(t)
	if _, ok := tt.AccessByRole(ui.RoleButton, "Merge"); ok {
		t.Error("plain merge offered with a merge queue")
	}
	ghtest.MustClick(t, tt, "Add to Merge Queue")
	hs.pumpCalled(t, "Enqueue", 1, d.idle)
	hs.h.Pump(t, func() bool { return !d.loading })
	hs.be.ov.Queue = MergeQueue{Enabled: true, InQueue: true, EntryID: "E1", Position: 1, State: "QUEUED"}
	hs.p.loadDetail(d)
	hs.h.Pump(t, func() bool { return !d.loading })
	tt.Frame()
	ghtest.MustHaveText(t, tt, "In the merge queue (position 1, queued)")
	ghtest.MustClick(t, tt, "Remove from Merge Queue")
	hs.h.Pump(t, func() bool { return hs.be.Count("Dequeue") == 1 })
}

func TestPickersApplyChanges(t *testing.T) {
	hs := newHarness(t)
	hs.be.labels = []github.Label{{Name: "bug"}, {Name: "docs"}}
	hs.be.users = []github.User{{Login: "bob"}, {Login: "carol"}}
	hs.be.milestones = []github.Milestone{{Number: 3, Title: "v1"}, {Number: 4, Title: "v2"}}
	d, tt := hs.openDetail(t)
	ghtest.MustClick(t, tt, "Edit Labels")
	hs.h.Pump(t, func() bool { return !hs.p.picker.loading })
	tt.Frame()
	ghtest.MustHaveText(t, tt, "Labels", "bug", "docs", "Filter")
	hs.p.picker.selected["docs"] = true
	hs.p.submitPicker()
	hs.pumpCalled(t, "EditIssue", 1, hs.pickerClosed)
	if hs.be.edit.Labels == nil || strings.Join(*hs.be.edit.Labels, ",") != "bug,docs" {
		t.Errorf("%+v", hs.be.edit.Labels)
	}
	hs.h.Pump(t, func() bool { return !d.loading })

	hs.p.openAssigneePicker(d)
	hs.h.Pump(t, func() bool { return !hs.p.picker.loading })
	hs.p.picker.selected["bob"] = true
	hs.p.submitPicker()
	hs.pumpCalled(t, "EditIssue", 2, hs.pickerClosed)
	if hs.be.edit.Assignees == nil || (*hs.be.edit.Assignees)[0] != "bob" {
		t.Errorf("%+v", hs.be.edit.Assignees)
	}
	hs.h.Pump(t, func() bool { return !d.loading })

	hs.p.openReviewerPicker(d)
	hs.h.Pump(t, func() bool { return !hs.p.picker.loading })
	hs.p.submitPicker()
	hs.h.Pump(t, func() bool { return !hs.p.picker.busy })
	if hs.p.picker.err == "" || !hs.p.picker.open {
		t.Error("an empty reviewer list was accepted")
	}
	hs.p.picker.selected["carol"] = true
	hs.p.submitPicker()
	hs.pumpCalled(t, "RequestReviewers", 1, hs.pickerClosed)
	if hs.be.arg != "[carol]" {
		t.Errorf("%q", hs.be.arg)
	}
	hs.h.Pump(t, func() bool { return !d.loading })

	hs.p.openMilestonePicker(d)
	hs.h.Pump(t, func() bool { return !hs.p.picker.loading })
	hs.p.picker.selected["v2"] = true
	hs.p.submitPicker()
	hs.pumpCalled(t, "EditIssue", 3, hs.pickerClosed)
	if hs.be.edit.Milestone == nil || *hs.be.edit.Milestone != 4 {
		t.Errorf("%+v", hs.be.edit.Milestone)
	}
}

func TestPickerFilterAndSingleChoice(t *testing.T) {
	pk := picker{items: []string{"Alpha", "beta", "ALPINE"}, filter: " al "}
	if got := pk.visible(); len(got) != 2 {
		t.Errorf("%v", got)
	}
	pk.selected = map[string]bool{"beta": true, "x": false}
	if got := pk.chosen(); len(got) != 1 || got[0] != "beta" {
		t.Errorf("%v", got)
	}
}

func TestPickerLoadFailureShowsError(t *testing.T) {
	hs := newHarness(t)
	hs.be.errs["Labels"] = &github.APIError{Status: 500, Message: "boom"}
	d, tt := hs.openDetail(t)
	hs.p.openLabelPicker(d)
	hs.h.Pump(t, func() bool { return !hs.p.picker.loading })
	tt.Frame()
	ghtest.MustHaveText(t, tt, "GitHub answered 500: boom")
}

func TestReactionsToggle(t *testing.T) {
	hs := newHarness(t)
	hs.be.timeline = []TimelineEvent{{Kind: "IssueComment", ID: "IC1", Actor: "carol", Body: "Nice", Time: testNow, Reactions: []Reaction{{Content: "THUMBS_UP", Count: 2, Viewer: true}, {Content: "HEART", Count: 1}}}}
	d, tt := hs.openDetail(t)
	ghtest.MustClick(t, tt, "+1 reaction")
	hs.pumpCalled(t, "React", 1, d.idle)
	if hs.be.arg != "IC1:THUMBS_UP:false" {
		t.Errorf("%q", hs.be.arg)
	}
	hs.h.Pump(t, func() bool { return !d.loading })
	tt.Advance(time.Second)
	ghtest.MustClick(t, tt, "Heart reaction")
	hs.pumpCalled(t, "React", 2, d.idle)
	if hs.be.arg != "IC1:HEART:true" {
		t.Errorf("%q", hs.be.arg)
	}
	if reactionLabel("CONFUSED") != "Confused" || reactionLabel("OTHER") != "other" {
		t.Error("reaction labels")
	}
}

func TestEditAndDeleteOwnComment(t *testing.T) {
	hs := newHarness(t)
	hs.be.timeline = []TimelineEvent{
		{Kind: "IssueComment", ID: "IC1", Actor: "me", Body: "mine", Time: testNow, CanEdit: true, CanDelete: true},
		{Kind: "IssueComment", ID: "IC2", Actor: "other", Body: "theirs", Time: testNow},
	}
	d, tt := hs.openDetail(t)
	if _, ok := tt.AccessByRole(ui.RoleButton, "Edit comment by other"); ok {
		t.Error("edit offered on another person's comment")
	}
	ghtest.MustClick(t, tt, "Edit comment by me")
	if hs.p.dialog.body != "mine" {
		t.Fatalf("body %q", hs.p.dialog.body)
	}
	hs.p.dialog.body = ""
	hs.p.submitDialog()
	if hs.p.dialog.err == "" {
		t.Error("an empty comment was accepted")
	}
	hs.p.dialog.body = "edited"
	hs.p.submitDialog()
	hs.pumpCalled(t, "EditComment", 1, hs.dialogClosed)
	if hs.be.arg != "issue:IC1:edited" || d.timeline[0].Body != "edited" {
		t.Errorf("%q %q", hs.be.arg, d.timeline[0].Body)
	}
	tt.Advance(time.Second)
	tt.Frame()
	ghtest.MustClick(t, tt, "Delete comment by me")
	hs.pumpCalled(t, "DeleteComment", 1, d.idle)
	if len(d.timeline) != 1 || d.timeline[0].ID != "IC2" {
		t.Errorf("%+v", d.timeline)
	}
}

func TestViewedFilesToggle(t *testing.T) {
	hs := newHarness(t)
	hs.be.files = append(hs.be.files, github.PRFile{Path: "b.go", Status: statusAdded})
	hs.be.viewed = map[string]bool{"b.go": true}
	d, tt := hs.openDetail(t)
	ghtest.MustHaveText(t, tt, "Hide viewed", "1 viewed")
	ghtest.MustClick(t, tt, "Viewed src/widget.go")
	hs.pumpCalled(t, "SetViewed", 1, d.idle)
	if hs.be.arg != "src/widget.go:true" || !d.viewed["src/widget.go"] {
		t.Errorf("%q %v", hs.be.arg, d.viewed)
	}
	d.hideViewed = true
	tt.Frame()
	ghtest.MustHaveText(t, tt, "Every file is marked as viewed.")
}

func TestViewedFailureRollsBack(t *testing.T) {
	hs := newHarness(t)
	hs.be.errs["SetViewed"] = &github.APIError{Status: 500, Message: "x"}
	d, _ := hs.openDetail(t)
	hs.p.setViewed(d, "src/widget.go", true)
	hs.h.Pump(t, func() bool { return d.busy == "" && hs.h.LastError() != "" })
	if d.viewed["src/widget.go"] {
		t.Error("the failed change was kept")
	}
}

func TestChecksShowRequiredLogsAndActions(t *testing.T) {
	hs := newHarness(t)
	hs.be.checks = []github.Check{
		{Name: "build", Status: statusCompleted, Conclusion: "failure", URL: "https://ci/build"},
		{Name: "lint", Status: "in_progress"},
		{Name: "deploy", Status: statusCompleted, Conclusion: "action_required"},
	}
	hs.be.ov.Checks = []CheckRun{{Name: "build", Required: true, JobID: 55, RunID: 66}, {Name: "lint", RunID: 67}, {Name: "deploy", RunID: 68}}
	hs.be.logText = "\x1b[31mred line\x1b[0m\nsecond line"
	d, tt := hs.openDetail(t)
	ghtest.MustHaveText(t, tt, "Required", "0 of 3 checks passed")
	ghtest.MustClick(t, tt, "View logs of build")
	logID := "pr/acme/widgets/12/log/55"
	if hs.h.Titles[logID] != "Logs: build" {
		t.Fatalf("opened %v", hs.h.Opened)
	}
	logTab := hs.p.logs[logID]
	hs.h.Pump(t, func() bool { return logTab.state != nil })
	if got := logTab.state.Buffer.String(); got != "red line\nsecond line" {
		t.Errorf("logs %q", got)
	}
	lt := viewTester(hs, logID)
	ghtest.MustHaveText(t, lt, "Logs of build")
	for _, c := range []struct{ label, arg string }{
		{"Re-run build", "66:rerun-failed-jobs"}, {"Cancel lint", "67:cancel"}, {"Approve run of deploy", "68:approve"},
	} {
		tt.Advance(time.Second)
		tt.Frame()
		ghtest.MustClick(t, tt, c.label)
		hs.h.Pump(t, func() bool { return hs.be.arg == c.arg && d.busy == "" })
		hs.h.Pump(t, func() bool { return !d.loading })
	}
}

func TestLogLoadFailureOffersRetry(t *testing.T) {
	hs := newHarness(t)
	hs.be.errs["JobLogs"] = &github.APIError{Status: 404, Message: "Not Found"}
	d, _ := hs.openDetail(t)
	hs.p.openLogs(d, checkRow{Check: github.Check{Name: "build"}, JobID: 5})
	tab := hs.p.logs["pr/acme/widgets/12/log/5"]
	hs.h.Pump(t, func() bool { return tab.err != nil })
	lt := viewTester(hs, "pr/acme/widgets/12/log/5")
	ghtest.MustHaveText(t, lt, "Cannot load the logs of build", "Retry")
	hs.be.clearErr("JobLogs")
	hs.be.logText = "ok"
	ghtest.MustClick(t, lt, "Retry")
	hs.h.Pump(t, func() bool { return tab.state != nil })
}

func TestChangesSinceLastReview(t *testing.T) {
	hs := newHarness(t)
	hs.be.reviews = []github.Review{{ID: "r1", Author: "me", State: reviewApproved, CommitID: "oldsha", Submitted: testNow.Add(-time.Hour)}}
	hs.be.compare = Comparison{Files: []github.PRFile{{Path: "src/new.go", Status: statusAdded, Additions: 3}}}
	hs.be.contents["headsha:src/new.go"] = "package n\n"
	d, tt := hs.openDetail(t)
	if d.sinceSHA != "oldsha" || len(d.since) != 1 {
		t.Fatalf("%q %v", d.sinceSHA, d.since)
	}
	ghtest.MustHaveText(t, tt, "Changes since your last review", "Compared with commit oldsha, the one you last reviewed.", "src/new.go")
	ghtest.MustClick(t, tt, "src/new.go")
	id := "pr/acme/widgets/12/since"
	if hs.h.Opened[len(hs.h.Opened)-1] != id {
		t.Fatalf("opened %v", hs.h.Opened)
	}
	tab, _ := hs.awaitDiff(t, id, "src/new.go")
	if err := hs.diffError(id, "src/new.go"); err != nil || tab.state.New.String() != "package n\n" || hs.h.Titles[id] != "#12 Changes Since Review" {
		t.Errorf("%v %q", err, hs.h.Titles[id])
	}
	if hs.be.Count("Compare") != 1 {
		t.Error("compare not requested once")
	}
}

func TestDiscardPendingReview(t *testing.T) {
	hs := newHarness(t)
	hs.be.reviews = []github.Review{{ID: "pend", Author: "me", State: reviewPending}}
	d, tt := hs.openDetail(t)
	hs.setOK(false)
	ghtest.MustClick(t, tt, "Discard Review")
	hs.h.Drain()
	if hs.be.Count("DeleteReview") != 0 {
		t.Fatal("discarded without confirmation")
	}
	hs.setOK(true)
	tt.Advance(time.Second)
	ghtest.MustClick(t, tt, "Discard Review")
	hs.pumpCalled(t, "DeleteReview", 1, d.idle)
}

func TestQuickPRFlowWithoutOverview(t *testing.T) {
	hs := newHarness(t)
	hs.be.ovErr = &github.GraphQLError{Errors: []github.GraphQLMessage{{Message: "nope"}}}
	d, tt := hs.openDetail(t)
	if d.hasOv {
		t.Fatal("overview must be absent")
	}
	ghtest.MustHaveText(t, tt, "Some details could not be loaded:", "overview:")
	if _, ok := tt.AccessByRole(ui.RoleButton, "Merge"); !ok {
		t.Error("merge must stay available without an overview")
	}
}

func (hs *harness) openDiffAt(t *testing.T, root string) (*prData, *diffTab, *ui.Tester) {
	t.Helper()
	hs.h.Root = root
	return hs.openDiffTab(t)
}

func TestDiffLinkMenu(t *testing.T) {
	hs := newHarness(t)
	_, tab, tt := hs.openDiffTab(t)
	tab.state.SetSelectedLines(editor.DiffLineRange{Side: editor.DiffNew, Start: 1, End: 2}, true)
	tt.Frame()
	items := map[string]string{
		"Copy GitHub Permalink":  "https://github.com/acme/widgets/blob/headsha/src/widget.go#L2-L3",
		"Copy Link to Branch":    "https://github.com/acme/widgets/blob/feature/src/widget.go#L2-L3",
		"Copy as Markdown":       "[src/widget.go#L2-L3](https://github.com/acme/widgets/blob/headsha/src/widget.go#L2-L3)",
		"Copy Pull Request Link": "https://github.com/acme/widgets/pull/12",
	}
	for item, want := range items {
		ghtest.MustClick(t, tt, "Copy or open a link")
		if err := tt.ChooseMenuItem(item); err != nil {
			t.Fatalf("%s: %v (menu %q)", item, err, tt.Menu())
		}
		if tt.Clipboard() != want {
			t.Errorf("%s: clipboard %q, want %q", item, tt.Clipboard(), want)
		}
		tt.Advance(time.Second)
	}
	ghtest.MustClick(t, tt, "Copy or open a link")
	if err := tt.ChooseMenuItem("Open in github.dev"); err != nil {
		t.Fatal(err)
	}
	if urls := tt.OpenedURLs(); len(urls) != 1 || urls[0] != "https://github.dev/acme/widgets/blob/headsha/src/widget.go#L2-L3" {
		t.Errorf("urls %v", urls)
	}
	tab.state.SetSelectedLines(editor.DiffLineRange{Side: editor.DiffOld, Start: 0, End: 0}, true)
	tt.Frame()
	tt.Advance(time.Second)
	ghtest.MustClick(t, tt, "Copy or open a link")
	if err := tt.ChooseMenuItem("Copy GitHub Permalink"); err != nil {
		t.Fatal(err)
	}
	if tt.Clipboard() != "https://github.com/acme/widgets/blob/basesha/src/widget.go#L1" {
		t.Errorf("old side permalink %q", tt.Clipboard())
	}
}

func TestDiffCompareWithLocal(t *testing.T) {
	hs := newHarness(t)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src/widget.go"), []byte("package local\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, tt := hs.openDiffAt(t, root)
	ghtest.MustClick(t, tt, "Compare with Local")
	id := "pr/acme/widgets/12/local/src/widget.go"
	tab, _ := hs.awaitDiff(t, id, diffPath)
	if err := hs.diffError(id, diffPath); err != nil || tab.state.New.String() != "package local\n" || tab.state.Old.String() != headText || hs.h.Titles[id] != "widget.go (#12, local)" {
		t.Errorf("%v new %q old %q title %q", hs.diffError(id, diffPath), tab.state.New.String(), tab.state.Old.String(), hs.h.Titles[id])
	}
	lt := viewTester(hs, id)
	if _, ok := lt.AccessByRole(ui.RoleButton, "Compare with Local"); ok {
		t.Error("local compare offered inside a local compare")
	}
	tab.state.OnGutterButton(editor.DiffLineRange{Side: editor.DiffNew})
	if hs.p.dialog.open {
		t.Error("commenting offered in a local compare")
	}
}

func TestDiffStepFilesAndCloseEditors(t *testing.T) {
	hs := newHarness(t)
	hs.be.files = []github.PRFile{{Path: "src/widget.go", Status: statusModified}, {Path: "b.txt", Status: statusAdded}}
	hs.be.contents["headsha:b.txt"] = "b\n"
	_, _, tt := hs.openDiffTab(t)
	if a, ok := tt.AccessByRole(ui.RoleButton, "Previous File"); !ok || !a.Disabled {
		t.Errorf("previous file on the first file: %+v", a)
	}
	if a, ok := tt.AccessByRole(ui.RoleButton, "Next File"); !ok || a.Disabled {
		t.Errorf("next file on the first of two files: %+v", a)
	}
	ghtest.MustClick(t, tt, "Next File")
	tt.Frame()
	hs.p.Run(ActionCloseEditors)
	if len(hs.h.Closed) != 1 || len(hs.p.diffs) != 0 {
		t.Errorf("closed %v diffs %d", hs.h.Closed, len(hs.p.diffs))
	}
}

func TestDiffHideCommentsToggle(t *testing.T) {
	hs := newHarness(t)
	_, tab, tt := hs.openDiffTab(t)
	ghtest.MustClick(t, tt, "Hide Comments")
	tt.Frame()
	if hidden := hs.p.diffs[diffViewID].hideComments; !hidden || len(tab.state.Annotations) != 0 || tt.HasText("Please rename this function") {
		t.Errorf("hidden %v annotations %d", hidden, len(tab.state.Annotations))
	}
}

func TestDiffFileComment(t *testing.T) {
	hs := newHarness(t)
	d, tab, tt := hs.openDiffTab(t)
	ghtest.MustClick(t, tt, "Comment on File")
	ghtest.MustHaveText(t, tt, "Comment on src/widget.go")
	hs.p.dialog.body = "About the whole file"
	hs.p.submitDialog()
	hs.pumpCalled(t, "AddFileThread", 1, hs.dialogClosed)
	if hs.be.arg != "review-1:src/widget.go:About the whole file" || d.pendingReview != "review-1" {
		t.Errorf("%q %q", hs.be.arg, d.pendingReview)
	}
	hs.h.Pump(t, func() bool { return !d.loading })
	_ = tab
}

func TestDiffSuggestionInsertion(t *testing.T) {
	hs := newHarness(t)
	_, tab, _ := hs.openDiffTab(t)
	tab.state.OnGutterButton(editor.DiffLineRange{Side: editor.DiffNew, Start: 2, End: 2})
	if hs.p.dialog.suggest != "func A() {}" {
		t.Fatalf("suggest %q", hs.p.dialog.suggest)
	}
	tt := viewTester(hs, diffViewID)
	ghtest.MustClick(t, tt, "Insert Suggestion")
	if hs.p.dialog.body != "```suggestion\nfunc A() {}\n```" {
		t.Errorf("body %q", hs.p.dialog.body)
	}
	tab.state.OnGutterButton(editor.DiffLineRange{Side: editor.DiffOld, Start: 2, End: 2})
	if hs.p.dialog.suggest != "" {
		t.Error("a suggestion offered for the old side")
	}
}

func TestDiffThreadCommentOwnerActions(t *testing.T) {
	hs := newHarness(t)
	hs.withDiffFixtures()
	hs.be.threads[0].Comments = append(hs.be.threads[0].Comments, github.Comment{ID: "c-mine", Author: "me", Body: "mine", Created: testNow})
	_, _ = hs.openDetail(t)
	hs.p.viewer = "me"
	d := hs.p.details[prNumber]
	hs.p.openDiff(d, d.files[0], diffSpec{mode: modePR})
	_, tt := hs.awaitDiff(t, diffViewID, diffPath)
	if _, ok := tt.AccessByRole(ui.RoleButton, "Edit comment by carol"); ok {
		t.Error("edit offered on another person's comment")
	}
	ghtest.MustClick(t, tt, "Edit comment by me")
	hs.p.dialog.body = "changed"
	hs.p.submitDialog()
	hs.pumpCalled(t, "EditComment", 1, hs.dialogClosed)
	if hs.be.arg != "review:c-mine:changed" || d.threads[0].Comments[1].Body != "changed" {
		t.Errorf("%q", hs.be.arg)
	}
	tt.Advance(time.Second)
	tt.Frame()
	ghtest.MustClick(t, tt, "Delete comment by me")
	hs.pumpCalled(t, "DeleteComment", 1, d.idle)
	if len(d.threads[0].Comments) != 1 {
		t.Errorf("comments %d", len(d.threads[0].Comments))
	}
	hs.p.deleteThreadComment(d, d.threads[0].Comments[0])
	hs.pumpCalled(t, "DeleteComment", 2, d.idle)
	for _, th := range d.threads {
		if th.ID == "t-open" {
			t.Error("an empty thread was kept")
		}
	}
}

func TestReplyCanResolveThread(t *testing.T) {
	hs := newHarness(t)
	d, _, tt := hs.openDiffTab(t)
	ghtest.MustClick(t, tt, "Reply")
	ghtest.MustHaveText(t, tt, resolveAfterLabel)
	hs.p.dialog.body = "Fixed"
	hs.p.dialog.checks[0].on = true
	hs.p.submitDialog()
	hs.h.Pump(t, func() bool { return hs.be.Count("Reply") == 1 && hs.be.Count("Resolve") == 1 && !hs.p.dialog.open })
	if !d.threads[0].Resolved || len(d.threads[0].Comments) != 2 {
		t.Errorf("%+v", d.threads[0])
	}
	hs.p.replyTo(d, d.threads[1])
	if len(hs.p.dialog.checks) != 0 {
		t.Error("resolve offered on a resolved thread")
	}
}

func TestApplySuggestion(t *testing.T) {
	hs := newHarness(t)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "src/widget.go")
	if err := os.WriteFile(target, []byte("package w\n\nfunc A() {}\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	hs.h.Root = root
	hs.withDiffFixtures()
	hs.be.threads[0].Comments[0].Body = "Use this:\n```suggestion\nfunc Renamed() {}\n```"
	d, _ := hs.openDetail(t)
	th := d.threads[0]
	hs.p.applySuggestion(d, th, "func Renamed() {}")
	if len(hs.h.Notices) != 1 || !strings.Contains(hs.h.Notices[0], "Check out this pull request") {
		t.Fatalf("notices %v", hs.h.Notices)
	}
	hs.p.branch.OID = "headsha"
	hs.p.applySuggestion(d, th, "func Renamed() {}")
	hs.h.Pump(t, func() bool { return d.busy == "" && len(hs.h.Notices) == 2 })
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "package w\n\nfunc Renamed() {}\n" {
		t.Errorf("%q %v", data, err)
	}
	if info, _ := os.Stat(target); info.Mode().Perm() != 0o640 {
		t.Errorf("mode %v", info.Mode())
	}
	bad := th
	bad.Path = "../../escape"
	hs.p.applySuggestion(d, bad, "x")
	if !strings.Contains(hs.h.Notices[len(hs.h.Notices)-1], "outside the project folder") {
		t.Errorf("notices %v", hs.h.Notices)
	}
}

func TestReviewModeSection(t *testing.T) {
	hs := newHarness(t)
	hs.be.viewed = map[string]bool{}
	hs.ready(t)
	tt := sideTester(hs.p)
	hs.h.Pump(t, hs.detailLoaded)
	tt.Frame()
	ghtest.MustHaveText(t, tt, "Changes in Pull Request", "0/1", "src/widget.go", "Open Pull Request", "Exit Review Mode")
	ghtest.MustClick(t, tt, "Viewed src/widget.go")
	hs.h.Pump(t, func() bool { return hs.be.Count("SetViewed") == 1 })
	tt.Advance(time.Second)
	ghtest.MustClick(t, tt, "src/widget.go")
	if hs.h.Opened[len(hs.h.Opened)-1] != diffViewID {
		t.Errorf("opened %v", hs.h.Opened)
	}
	hs.local.setCurrent("main", "origin/main")
	ghtest.MustClick(t, tt, "Exit Review Mode")
	hs.h.Pump(t, func() bool { return len(hs.h.Notices) > 0 })
	hs.local.mu.Lock()
	defer hs.local.mu.Unlock()
	if len(hs.local.checkouts) != 1 || hs.local.checkouts[0] != "main" || len(hs.p.diffs) != 0 {
		t.Errorf("checkouts %v diffs %d", hs.local.checkouts, len(hs.p.diffs))
	}
}

func TestReviewModeHidesViewedFilesAndHandlesStates(t *testing.T) {
	hs := newHarness(t)
	hs.be.viewed = map[string]bool{"src/widget.go": true}
	hs.ready(t)
	tt := sideTester(hs.p)
	hs.h.Pump(t, hs.detailLoaded)
	tt.Frame()
	ghtest.MustHaveText(t, tt, "1/1")
	ghtest.MustClick(t, tt, "Hide viewed")
	ghtest.MustHaveText(t, tt, "Every file is marked as viewed.")
	hs.p.details[prNumber].files = nil
	tt.Frame()
	ghtest.MustHaveText(t, tt, "No changed files.")
	d := hs.p.details[prNumber]
	d.loaded, d.err = false, &github.APIError{Status: 500, Message: "boom"}
	tt.Frame()
	ghtest.MustHaveText(t, tt, "GitHub answered 500: boom")
}

func TestExitReviewWithoutPR(t *testing.T) {
	hs := newHarness(t)
	hs.be.current = nil
	hs.ready(t)
	hs.p.Run(ActionExitReview)
	if len(hs.h.Notices) != 1 {
		t.Errorf("notices %v", hs.h.Notices)
	}
}

func TestLocalBranchesSection(t *testing.T) {
	hs := newHarness(t)
	topic := testPR(61, "Topic work")
	topic.HeadRef = "feature"
	hs.be.localPRs = []github.PullRequest{topic}
	hs.ready(t)
	tt := sideTester(hs.p)
	ghtest.MustHaveText(t, tt, "Local Pull Request Branches")
	if hs.be.Count("LocalBranchPRs") != 0 {
		t.Fatal("loaded while collapsed")
	}
	ghtest.MustClick(t, tt, "Local Pull Request Branches")
	hs.h.Pump(t, func() bool { return hs.p.side.local.loaded })
	tt.Frame()
	ghtest.MustHaveText(t, tt, "Topic work", "#61")
	if hs.p.side.local.more {
		t.Error("the local section pages")
	}
	if len(hs.p.locals) != 1 || hs.p.locals[0] != "feature" {
		t.Errorf("locals %v", hs.p.locals)
	}
}

func TestPRRowContextMenu(t *testing.T) {
	hs := newHarness(t)
	hs.ready(t)
	tt := sideTester(hs.p)
	hs.h.Pump(t, func() bool { return hs.p.side.review.loaded })
	tt.Frame()
	if err := tt.RightClick("Review me please"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Copy Link"); err != nil {
		t.Fatalf("%v (menu %q)", err, tt.Menu())
	}
	if tt.Clipboard() != "https://github.com/acme/widgets/pull/31" {
		t.Errorf("clipboard %q", tt.Clipboard())
	}
	if err := tt.RightClick("Review me please"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Open on GitHub"); err != nil {
		t.Fatal(err)
	}
	if urls := tt.OpenedURLs(); len(urls) != 1 || !strings.HasSuffix(urls[0], "/pull/31") {
		t.Errorf("urls %v", urls)
	}
	if err := tt.RightClick("Review me please"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Checkout"); err != nil {
		t.Fatal(err)
	}
	hs.h.Pump(t, func() bool { return len(hs.h.Notices) > 0 })
	hs.mu.Lock()
	defer hs.mu.Unlock()
	if len(hs.co) != 1 || hs.co[0] != 31 {
		t.Errorf("checkout %v", hs.co)
	}
}

func TestSavedQueriesPersistAndLoad(t *testing.T) {
	dir := t.TempDir()
	hs := newHarness(t)
	hs.h.UserData = dir
	hs.be.lists["search:label:bug"] = []github.PullRequest{testPR(9, "Bug hunt")}
	hs.ready(t)
	hs.p.Run(ActionSaveQuery)
	if len(hs.h.Asks) != 0 || len(hs.h.Notices) != 1 {
		t.Fatalf("asks %d notices %v", len(hs.h.Asks), hs.h.Notices)
	}
	hs.p.runSearch("label:bug")
	hs.p.Run(ActionSaveQuery)
	if len(hs.h.Asks) != 1 {
		t.Fatal("no prompt")
	}
	if err := hs.h.Asks[0].Submit(" "); err == nil {
		t.Error("an empty label was accepted")
	}
	if err := hs.h.Asks[0].Submit("Bugs"); err != nil {
		t.Fatal(err)
	}
	if err := hs.h.Asks[0].Submit("bugs"); err == nil {
		t.Error("a duplicate label was accepted")
	}
	hs.h.WaitFor(t, func() bool { return savedQueryCount(t, dir) == 1 })
	tt := sideTester(hs.p)
	tt.Frame()
	ghtest.MustHaveText(t, tt, "Bugs")

	second := newHarness(t)
	second.h.UserData = dir
	second.be.lists["search:label:bug"] = []github.PullRequest{testPR(9, "Bug hunt")}
	second.ready(t)
	if len(second.p.side.custom) != 1 || second.p.side.custom[0].title != "Bugs" {
		t.Fatalf("custom %+v", second.p.side.custom)
	}
	tt2 := sideTester(second.p)
	ghtest.MustClick(t, tt2, "Bugs")
	second.h.Pump(t, func() bool { return second.p.side.custom[0].loaded })
	tt2.Frame()
	ghtest.MustHaveText(t, tt2, "Bug hunt")

	sec := second.p.side.custom[0]
	if err := second.p.editQuery(sec, ""); err == nil {
		t.Error("an empty query was accepted")
	}
	if err := second.p.editQuery(sec, "label:docs"); err != nil {
		t.Fatal(err)
	}
	if got := second.p.side.custom[0]; got.query != "label:docs" || got.opts.Query != "label:docs" || got == sec {
		t.Errorf("%+v", got)
	}
	second.p.removeQuery(second.p.side.custom[0])
	if len(second.p.side.custom) != 0 || len(second.p.queries) != 0 {
		t.Error("query not removed")
	}
	second.h.WaitFor(t, func() bool { return savedQueryCount(t, dir) == 0 })
}

func TestAddQueryLimits(t *testing.T) {
	hs := newHarness(t)
	hs.ready(t)
	for i := range maxQueries {
		if err := hs.p.addQuery(strings.Repeat("q", i+1), "x"); err != nil {
			t.Fatal(err)
		}
	}
	if err := hs.p.addQuery("one more", "x"); err == nil {
		t.Error("the limit was not enforced")
	}
	hs.h.Drain()
}

func TestSectionMenuOfSearch(t *testing.T) {
	hs := newHarness(t)
	hs.ready(t)
	hs.p.runSearch("is:open")
	tt := sideTester(hs.p)
	hs.h.Pump(t, func() bool { return hs.p.side.search.loaded })
	tt.Frame()
	if err := tt.RightClick("Search Results"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Clear Search"); err != nil {
		t.Fatalf("%v (menu %q)", err, tt.Menu())
	}
	if hs.p.side.search != nil {
		t.Error("search not cleared")
	}
}

func TestTitleAndBadge(t *testing.T) {
	hs := newHarness(t)
	hs.start(t)
	if hs.p.Title() != "Pull Requests" || hs.p.NotificationsTitle() != "Notifications" {
		t.Error("titles")
	}
	if hs.p.Badge() != 0 {
		t.Error("badge before anything loaded")
	}
	tt := sideTester(hs.p)
	hs.h.Pump(t, func() bool { return hs.p.side.review.loaded })
	tt.Frame()
	if hs.p.Badge() != 1 {
		t.Errorf("badge %d", hs.p.Badge())
	}
	hs.p.Close()
	if hs.p.Badge() != 0 || hs.p.NotificationsBadge() != 0 {
		t.Error("badges after close")
	}
}

func TestNewActions(t *testing.T) {
	hs := newHarness(t)
	hs.ready(t)
	hs.p.Run(ActionCopyLink)
	hs.p.Run(ActionOpenOnGitHub)
	hs.mu.Lock()
	if len(hs.copied) != 1 || hs.copied[0] != "https://github.com/acme/widgets/pull/12" || len(hs.urls) != 1 {
		t.Errorf("copied %v urls %v", hs.copied, hs.urls)
	}
	hs.mu.Unlock()
	hs.be.current = nil
	hs.p.current.pr = nil
	hs.p.Run(ActionCopyLink)
	hs.p.Run(ActionOpenOnGitHub)
	if len(hs.h.Notices) != 3 {
		t.Errorf("notices %v", hs.h.Notices)
	}
	hs.local.setCurrent("feature", "origin/feature")
	hs.p.current.pr = prPtr(hs.be.pr)
	hs.p.Run(ActionExitReview)
	hs.h.Pump(t, func() bool { return hs.checkedOut("main") })
}

func (hs *harness) checkedOut(ref string) bool {
	hs.local.mu.Lock()
	defer hs.local.mu.Unlock()
	return len(hs.local.checkouts) > 0 && hs.local.checkouts[0] == ref
}

func TestCreateExtras(t *testing.T) {
	hs := newHarness(t)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".github"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".github/pull_request_template.md"), []byte("## What changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	hs.h.Root = root
	hs.be.users = []github.User{{Login: "bob"}, {Login: "me"}}
	hs.be.labels = []github.Label{{Name: "bug"}}
	hs.be.milestones = []github.Milestone{{Number: 4, Title: "v2"}}
	hs.be.compare = Comparison{Files: []github.PRFile{{Path: "a.go", Status: statusModified, Additions: 1}}}
	hs.local.ahead = []git.Commit{{Subject: "first commit"}, {Subject: "second commit"}}
	f, tt := hs.openCreateView(t)
	hs.h.Pump(t, func() bool { return f.body != "" && !f.cmpLoading && f.cmpFor == "main" })
	tt.Frame()
	ghtest.MustHaveText(t, tt, "Changes", "2 commits ahead of main", "first commit", "second commit", "a.go", "Choose Reviewers", "Assign to me")
	if f.body != "## What changed\n" {
		t.Errorf("template %q", f.body)
	}

	ghtest.MustClick(t, tt, "Choose Reviewers")
	hs.h.Pump(t, func() bool { return !hs.p.picker.loading })
	hs.p.picker.selected["bob"] = true
	hs.p.submitPicker()
	hs.h.Pump(t, func() bool { return !hs.p.picker.open })
	hs.p.pickFormLabels(f)
	hs.h.Pump(t, func() bool { return !hs.p.picker.loading })
	hs.p.picker.selected["bug"] = true
	hs.p.submitPicker()
	hs.h.Pump(t, func() bool { return !hs.p.picker.open })
	hs.p.pickFormMilestone(f)
	hs.h.Pump(t, func() bool { return !hs.p.picker.loading })
	hs.p.picker.selected["v2"] = true
	hs.p.submitPicker()
	hs.h.Pump(t, func() bool { return !hs.p.picker.open })
	hs.p.viewer = "me"
	f.assignMe = true
	f.autoMergeLabel = autoMergeChoices[2]
	hs.p.submitCreate(f)
	hs.h.Pump(t, func() bool { return hs.p.createForm == nil })
	if hs.be.created.Body != "## What changed\n" {
		t.Errorf("created %+v", hs.be.created)
	}
	hs.be.mu.Lock()
	defer hs.be.mu.Unlock()
	edit := hs.be.edit
	if edit.Labels == nil || (*edit.Labels)[0] != "bug" || edit.Assignees == nil || (*edit.Assignees)[0] != "me" || edit.Milestone == nil || *edit.Milestone != 4 {
		t.Errorf("edit %+v", edit)
	}
	if hs.be.arg != "squash:true" {
		t.Errorf("auto-merge %q", hs.be.arg)
	}
	if len(hs.h.Errs) != 0 {
		t.Errorf("errors %v", hs.h.Errs)
	}
}

func TestCreateReportsPartialFailures(t *testing.T) {
	hs := newHarness(t)
	hs.be.errs["RequestReviewers"] = &github.APIError{Status: 422, Message: "cannot review own PR"}
	f, _ := hs.openCreateView(t)
	f.reviewers = []string{"bob"}
	hs.p.submitCreate(f)
	hs.h.Pump(t, func() bool { return hs.p.createForm == nil && len(hs.h.Errs) == 1 })
	if !strings.Contains(hs.h.Errs[0], "#77 was created") || !strings.Contains(hs.h.Errs[0], "reviewers") {
		t.Errorf("%q", hs.h.Errs[0])
	}
}

func TestCreateUntypedBodyOnlyReceivesTemplate(t *testing.T) {
	hs := newHarness(t)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "pull_request_template.md"), []byte("template"), 0o644); err != nil {
		t.Fatal(err)
	}
	hs.h.Root = root
	hs.be.current = nil
	hs.ready(t)
	hs.p.openCreate()
	f := hs.p.createForm
	f.typedBody, f.body = true, "mine"
	hs.h.Pump(t, func() bool { return !f.loading })
	hs.h.Drain()
	if f.body != "mine" {
		t.Errorf("body %q", f.body)
	}
	if autoMergeMethodFor(autoMergeChoices[0]) != "" || autoMergeMethodFor(autoMergeChoices[3]) != github.MergeMethodRebase || autoMergeMethodFor("x") != "" {
		t.Error("auto-merge mapping")
	}
}

func TestReviewModeReloadsOnRefresh(t *testing.T) {
	hs := newHarness(t)
	hs.ready(t)
	tt := sideTester(hs.p)
	hs.h.Pump(t, hs.detailLoaded)
	base := hs.be.Count("Get")
	hs.p.refresh()
	tt.Frame()
	hs.h.Pump(t, func() bool { return hs.be.Count("Get") > base && !hs.p.details[prNumber].loading })
}
