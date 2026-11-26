package issueui

import (
	"slices"
	"testing"

	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/github"
	"github.com/egoist/mygo/ui"
)

func (hs *harness) readyWithIssue(t *testing.T) {
	t.Helper()
	hs.seedIssue()
	hs.ready(t)
}

func (hs *harness) startIssue(t *testing.T, number int) {
	t.Helper()
	hs.i.startWorking(number, nil)
	hs.h.pump(t, func() bool { return hs.i.working.busy == "" })
}

func TestStartWorkingActionAsksForNumber(t *testing.T) {
	hs := newHarness(t)
	hs.readyWithIssue(t)
	hs.i.Run(ActionStartWorking)
	if len(hs.h.asks) != 1 || hs.h.asks[0].title != "Start Working on Issue" {
		t.Fatalf("asks %+v", hs.h.asks)
	}
	if err := hs.h.asks[0].submit("nope"); err != errBadIssueNumber {
		t.Errorf("error %v", err)
	}
	if err := hs.h.asks[0].submit("#5"); err != nil {
		t.Fatal(err)
	}
	hs.h.pump(t, func() bool { return hs.i.work.Active != nil })
}

func TestStartWorkingUsesSelectedIssue(t *testing.T) {
	hs := newHarness(t)
	hs.readyWithIssue(t)
	issue := hs.be.issues[issueNumber]
	hs.i.side.rows = []sideRow{{kind: rowHeader, sec: hs.i.side.sections[0]}, {kind: rowIssue, sec: hs.i.side.sections[0], issue: &issue}}
	hs.i.side.row = 1
	hs.i.Run(ActionStartWorking)
	if len(hs.h.asks) != 0 {
		t.Fatalf("asked although an issue is selected: %+v", hs.h.asks)
	}
	hs.h.pump(t, func() bool { return hs.i.work.Active != nil })
	if hs.be.called("GetIssue") != 0 {
		t.Error("the selected issue was fetched again")
	}
}

func TestStartWorkingAssignsCreatesBranchAndTracksIssue(t *testing.T) {
	hs := newHarness(t)
	hs.readyWithIssue(t)
	hs.startIssue(t, issueNumber)
	active := hs.i.work.Active
	if active == nil || active.Number != 5 || active.Branch != "me/issue5" || active.Base != "main" || active.Title != "Crash on start" {
		t.Fatalf("active %+v", active)
	}
	if !slices.Contains(hs.local.called(), "CreateBranch:me/issue5::true") {
		t.Errorf("git calls %v", hs.local.called())
	}
	if len(hs.be.edits) != 1 || !slices.Equal(*hs.be.edits[0].Assignees, []string{"bob", "me"}) {
		t.Errorf("edits %+v", hs.be.edits)
	}
	if got := hs.h.lastNotice(); got != "Working on #5 in branch me/issue5" {
		t.Errorf("notice %q", got)
	}
	if got := hs.i.CommitTemplate(); got != "Crash on start\nFixes #5" {
		t.Errorf("commit template %q", got)
	}
	saved, err := loadWork(hs.h.data, hs.i.repo)
	if err != nil || saved.Active == nil || saved.Active.Branch != "me/issue5" {
		t.Errorf("saved %+v %v", saved, err)
	}
	if d := hs.i.details[issueNumber]; d != nil {
		t.Error("starting must not create a detail entry")
	}
}

func TestStartWorkingWithoutAssigning(t *testing.T) {
	hs := newHarness(t)
	hs.readyWithIssue(t)
	settings := DefaultSettings()
	settings.AssignWhenWorking = false
	hs.i.SetSettings(settings)
	hs.startIssue(t, issueNumber)
	if hs.be.called("EditIssue") != 0 || hs.i.work.Active == nil {
		t.Errorf("edit calls %d active %v", hs.be.called("EditIssue"), hs.i.work.Active)
	}
}

func TestStartWorkingAlreadyAssignedSkipsEdit(t *testing.T) {
	hs := newHarness(t)
	hs.readyWithIssue(t)
	hs.be.with(func() {
		issue := hs.be.issues[issueNumber]
		issue.Assignees = []string{"me"}
		hs.be.issues[issueNumber] = issue
	})
	hs.startIssue(t, issueNumber)
	if hs.be.called("EditIssue") != 0 {
		t.Error("assigned although already assigned")
	}
}

func TestStartWorkingAssignFailureIsAWarning(t *testing.T) {
	hs := newHarness(t)
	hs.readyWithIssue(t)
	hs.be.errs["EditIssue"] = errBoom
	hs.startIssue(t, issueNumber)
	if hs.i.work.Active == nil {
		t.Fatal("work did not start")
	}
	if got := hs.h.lastNotice(); got != "Working on #5 in branch me/issue5. Could not assign you: boom" {
		t.Errorf("notice %q", got)
	}
}

func TestStartWorkingCustomTemplates(t *testing.T) {
	hs := newHarness(t)
	hs.readyWithIssue(t)
	settings := DefaultSettings()
	settings.IssueBranchTitle = "feature/${issueNumber}-${sanitizedIssueTitle}"
	settings.WorkingIssueFormatSCM = "${issueNumberLabel}: ${issueTitle} (${user})"
	hs.i.SetSettings(settings)
	hs.startIssue(t, issueNumber)
	if hs.i.work.Active.Branch != "feature/5-crash-on-start" {
		t.Errorf("branch %q", hs.i.work.Active.Branch)
	}
	if got := hs.i.CommitTemplate(); got != "#5: Crash on start (me)" {
		t.Errorf("commit template %q", got)
	}
}

func TestStartWorkingChecksOutExistingBranches(t *testing.T) {
	hs := newHarness(t)
	hs.readyWithIssue(t)
	hs.local.branches = append(hs.local.branches, git.Branch{Name: "me/issue5"})
	hs.startIssue(t, issueNumber)
	if !slices.Contains(hs.local.called(), "Checkout:me/issue5") {
		t.Errorf("git calls %v", hs.local.called())
	}

	hs2 := newHarness(t)
	hs2.readyWithIssue(t)
	hs2.local.branches = append(hs2.local.branches, git.Branch{Name: "origin/me/issue5", Remote: true})
	hs2.startIssue(t, issueNumber)
	if !slices.Contains(hs2.local.called(), "CheckoutNewTracking:origin/me/issue5") {
		t.Errorf("git calls %v", hs2.local.called())
	}

	hs3 := newHarness(t)
	hs3.readyWithIssue(t)
	hs3.local.status.Branch = "me/issue5"
	hs3.startIssue(t, issueNumber)
	for _, call := range hs3.local.called() {
		if call != "Remotes" && call != "Status" && call != "Branches" {
			t.Errorf("unexpected git call %q while already on the branch", call)
		}
	}
	if hs3.i.work.Active == nil {
		t.Error("work did not start")
	}
}

func TestStartWorkingLinkedBranch(t *testing.T) {
	hs := newHarness(t)
	hs.readyWithIssue(t)
	settings := DefaultSettings()
	settings.LinkBranchOnGitHub = true
	hs.i.SetSettings(settings)
	hs.startIssue(t, issueNumber)
	if len(hs.be.linked) != 1 || hs.be.linked[0] != "5:me/issue5:abc123" {
		t.Errorf("linked %v", hs.be.linked)
	}
	calls := hs.local.called()
	if !slices.Contains(calls, "Fetch:origin") || !slices.Contains(calls, "CheckoutNewTracking:origin/me/issue5") {
		t.Errorf("git calls %v", calls)
	}
}

func TestStartWorkingLinkedBranchUsesDefaultBranchWhenHeadIsUnpushed(t *testing.T) {
	hs := newHarness(t)
	hs.local.outgoing = []git.Commit{{OID: "abc123"}}
	hs.readyWithIssue(t)
	settings := DefaultSettings()
	settings.LinkBranchOnGitHub = true
	hs.i.SetSettings(settings)
	hs.startIssue(t, issueNumber)
	if len(hs.be.linked) != 1 || hs.be.linked[0] != "5:me/issue5:" {
		t.Errorf("linked %v", hs.be.linked)
	}
}

func TestStartWorkingLinkedBranchFailureFallsBackToLocal(t *testing.T) {
	hs := newHarness(t)
	hs.readyWithIssue(t)
	hs.be.errs["CreateLinkedBranch"] = errBoom
	settings := DefaultSettings()
	settings.LinkBranchOnGitHub = true
	hs.i.SetSettings(settings)
	hs.startIssue(t, issueNumber)
	if hs.i.work.Active == nil || !slices.Contains(hs.local.called(), "CreateBranch:me/issue5::true") {
		t.Errorf("active %v calls %v", hs.i.work.Active, hs.local.called())
	}
	if got := hs.h.lastNotice(); got != "Working on #5 in branch me/issue5. Could not create the linked branch: boom" {
		t.Errorf("notice %q", got)
	}
}

func TestStartWorkingGitFailureShowsError(t *testing.T) {
	hs := newHarness(t)
	hs.readyWithIssue(t)
	hs.local.errs["CreateBranch"] = errBoom
	hs.startIssue(t, issueNumber)
	if hs.i.work.Active != nil || hs.h.lastError() == "" {
		t.Errorf("active %v error %q", hs.i.work.Active, hs.h.lastError())
	}
	if hs.i.CommitTemplate() != "" {
		t.Error("commit template without a working issue")
	}
}

func TestStartWorkingTwiceNotifies(t *testing.T) {
	hs := newHarness(t)
	hs.readyWithIssue(t)
	hs.startIssue(t, issueNumber)
	hs.i.startWorking(issueNumber, nil)
	if got := hs.h.lastNotice(); got != "You are already working on #5." {
		t.Errorf("notice %q", got)
	}
}

func TestStartWorkingUnknownIssueShowsError(t *testing.T) {
	hs := newHarness(t)
	hs.readyWithIssue(t)
	hs.startIssue(t, 404)
	if hs.i.work.Active != nil || hs.h.lastError() == "" {
		t.Errorf("active %v error %q", hs.i.work.Active, hs.h.lastError())
	}
}

func TestStopWorkingSwitchesBack(t *testing.T) {
	hs := newHarness(t)
	hs.readyWithIssue(t)
	hs.startIssue(t, issueNumber)
	hs.i.Run(ActionStopWorking)
	hs.h.pump(t, func() bool { return hs.i.work.Active == nil })
	if !slices.Contains(hs.local.called(), "Checkout:main") {
		t.Errorf("git calls %v", hs.local.called())
	}
	if hs.i.CommitTemplate() != "" || len(hs.i.work.Recent) != 1 || hs.i.work.Recent[0].Number != 5 {
		t.Errorf("work %+v", hs.i.work)
	}
	if got := hs.h.lastNotice(); got != "Stopped working on #5" {
		t.Errorf("notice %q", got)
	}
	saved, _ := loadWork(hs.h.data, hs.i.repo)
	if saved.Active != nil || len(saved.Recent) != 1 {
		t.Errorf("saved %+v", saved)
	}
}

func TestStopWorkingWithoutIssueNotifies(t *testing.T) {
	hs := newHarness(t)
	hs.ready(t)
	hs.i.Run(ActionStopWorking)
	if got := hs.h.lastNotice(); got != "You are not working on an issue." {
		t.Errorf("notice %q", got)
	}
}

func TestStopWorkingSwitchBackFailureStillStops(t *testing.T) {
	hs := newHarness(t)
	hs.readyWithIssue(t)
	hs.startIssue(t, issueNumber)
	hs.local.errs["Checkout"] = errBoom
	hs.i.stopWorking()
	hs.h.pump(t, func() bool { return hs.i.work.Active == nil })
	if hs.h.lastError() == "" {
		t.Error("the failed switch was not reported")
	}
}

func TestStopWorkingStaysOnBranchWhenAlreadyElsewhere(t *testing.T) {
	hs := newHarness(t)
	hs.readyWithIssue(t)
	hs.startIssue(t, issueNumber)
	hs.local.setBranch("other")
	hs.i.stopWorking()
	hs.h.pump(t, func() bool { return hs.i.work.Active == nil })
	if slices.Contains(hs.local.called(), "Checkout:main") {
		t.Error("switched although the user was on another branch")
	}
}

func TestContinueWorking(t *testing.T) {
	hs := newHarness(t)
	hs.readyWithIssue(t)
	hs.i.Run(ActionContinueWorking)
	if got := hs.h.lastNotice(); got != "No earlier issue to continue. Start working on an issue first." {
		t.Errorf("notice %q", got)
	}
	hs.startIssue(t, issueNumber)
	hs.i.stopWorking()
	hs.h.pump(t, func() bool { return hs.i.work.Active == nil })
	hs.i.Run(ActionContinueWorking)
	ask := hs.h.asks[len(hs.h.asks)-1]
	if ask.title != "Continue Working on Issue" || ask.value != "5" {
		t.Fatalf("ask %+v", ask)
	}
	if err := ask.submit("6"); err == nil {
		t.Error("an issue never worked on must be refused")
	}
	if err := ask.submit("5"); err != nil {
		t.Fatal(err)
	}
	hs.h.pump(t, func() bool { return hs.i.work.Active != nil })
	if hs.i.work.Active.Branch != "me/issue5" || hs.local.status.Branch != "me/issue5" {
		t.Errorf("active %+v branch %s", hs.i.work.Active, hs.local.status.Branch)
	}
	hs.i.Run(ActionContinueWorking)
	if got := hs.h.lastNotice(); got != "You are already working on #5." {
		t.Errorf("notice %q", got)
	}
}

func TestBranchChangedStopsTracking(t *testing.T) {
	hs := newHarness(t)
	hs.readyWithIssue(t)
	hs.startIssue(t, issueNumber)
	hs.i.BranchChanged()
	hs.h.drain()
	if hs.i.work.Active == nil {
		t.Fatal("tracking stopped although the branch did not change")
	}
	hs.local.setBranch("elsewhere")
	hs.i.BranchChanged()
	hs.h.pump(t, func() bool { return hs.i.work.Active == nil })
	if len(hs.i.work.Recent) != 1 {
		t.Errorf("recent %+v", hs.i.work.Recent)
	}
}

func TestWorkingStateSurvivesRestart(t *testing.T) {
	hs := newHarness(t)
	hs.readyWithIssue(t)
	hs.startIssue(t, issueNumber)

	again := NewWith(hs.h, hs.env)
	t.Cleanup(again.Close)
	hs.h.pump(t, func() bool { return again.phase == phaseReady })
	if again.work.Active == nil || again.work.Active.Number != 5 || again.CommitTemplate() != "Crash on start\nFixes #5" {
		t.Errorf("restored %+v", again.work)
	}

	hs.local.setBranch("other")
	third := NewWith(hs.h, hs.env)
	t.Cleanup(third.Close)
	hs.h.pump(t, func() bool { return third.phase == phaseReady })
	if third.work.Active != nil || len(third.work.Recent) != 1 {
		t.Errorf("a branch switch outside the editor must end the tracking: %+v", third.work)
	}
}

func statusTester(i *Issues) *ui.Tester {
	return ui.NewTester(func(c *ui.Context) { ui.Row(c).Children(func() { i.StatusItems(c) }) }, sideWidth, 40)
}

func TestStatusItems(t *testing.T) {
	hs := newHarness(t)
	hs.readyWithIssue(t)
	tt := statusTester(hs.i)
	if len(tt.Texts()) != 0 {
		t.Errorf("status shows %q without a working issue", tt.Texts())
	}
	hs.startIssue(t, issueNumber)
	tt.Frame()
	mustHaveText(t, tt, "#5")
	if _, ok := tt.AccessByRole(ui.RoleButton, "Working on issue #5"); !ok {
		t.Error("status item has no accessible name")
	}
	mustClick(t, tt, "#5")
	if hs.h.opened[len(hs.h.opened)-1] != issueViewID {
		t.Errorf("opened %v", hs.h.opened)
	}
}

func TestDetailWorkingButtons(t *testing.T) {
	hs := newHarness(t)
	d, tt := hs.openDetail(t)
	_ = d
	mustHaveText(t, tt, "Start Working")
	mustClick(t, tt, "Start Working")
	hs.h.pump(t, func() bool { return hs.i.work.Active != nil && hs.i.working.busy == "" })
	tt.Frame()
	mustHaveText(t, tt, "Stop Working")
	mustClick(t, tt, "Stop Working")
	hs.h.pump(t, func() bool { return hs.i.work.Active == nil })
	tt.Frame()
	mustHaveText(t, tt, "Continue Working")
	mustClick(t, tt, "Continue Working")
	hs.h.pump(t, func() bool { return hs.i.work.Active != nil && hs.i.working.busy == "" })
	if d.issue.Assignees[len(d.issue.Assignees)-1] != "me" {
		t.Errorf("assignees %v", d.issue.Assignees)
	}
}

func TestIssueMenuStartsWorking(t *testing.T) {
	hs := newHarness(t)
	hs.seedIssue()
	hs.be.pages[queryAssigned] = []github.IssuePage{{Total: 1, Issues: []github.Issue{hs.be.issues[issueNumber]}}}
	hs.ready(t)
	tt := sideTester(hs.i)
	hs.h.pump(t, func() bool { return hs.i.side.sections[0].loaded })
	tt.Frame()
	if err := tt.RightClick("Crash on start, #5 by alice"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Start Working"); err != nil {
		t.Fatal(err)
	}
	hs.h.pump(t, func() bool { return hs.i.work.Active != nil })
}
