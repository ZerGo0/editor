package prui

import (
	"testing"

	"github.com/ZerGo0/editor/internal/github"
	"github.com/egoist/mygo/ui"
)

var namedRoles = map[ui.Role]string{
	ui.RoleButton:       "button",
	ui.RoleMenuButton:   "menu button",
	ui.RoleCheckBox:     "check box",
	ui.RoleToggleButton: "toggle",
	ui.RoleTextField:    "text field",
	ui.RoleSwitch:       "switch",
	ui.RolePopUpButton:  "select",
	ui.RoleLink:         "link",
}

func requireNamedControls(t *testing.T, name string, tt *ui.Tester) {
	t.Helper()
	checked := 0
	for _, a := range tt.AccessAll() {
		kind, ok := namedRoles[a.Role]
		if !ok {
			continue
		}
		checked++
		if a.Name == "" {
			t.Errorf("%s: a %s at %v has no accessible name", name, kind, a.Bounds)
		}
	}
	if checked == 0 {
		t.Errorf("%s: no controls found, the test checks nothing", name)
	}
}

func TestEveryControlHasAnAccessibleName(t *testing.T) {
	hs := newHarness(t)
	hs.withNotifications()
	hs.withDiffFixtures()
	hs.be.checks = []github.Check{{Name: "build", Status: statusCompleted, Conclusion: "failure", URL: "https://ci"}}
	hs.be.ov.Checks = []CheckRun{{Name: "build", JobID: 5, RunID: 6, Required: true}}
	hs.be.timeline = []TimelineEvent{{Kind: "IssueComment", ID: "c", Actor: "me", Body: "x", Time: testNow, CanEdit: true, CanDelete: true, Reactions: []Reaction{{Content: "HEART", Count: 1}}}}
	hs.be.reviews = []github.Review{{ID: "p", Author: "me", State: reviewPending}, {ID: "r", Author: "me", State: reviewApproved, CommitID: "old", Submitted: testNow.Add(-1)}}
	hs.be.compare = Comparison{Files: []github.PRFile{{Path: "n.go", Status: statusAdded}}}
	hs.be.commits = []github.Commit{{SHA: "abcdef123", Message: "m", URL: "https://c"}}
	hs.be.viewed = map[string]bool{}
	hs.be.localPRs = []github.PullRequest{testPR(61, "Local")}
	d, detail := hs.openDetail(t)
	hs.p.viewer = "me"
	hs.p.side.local.open = true

	side := sideTester(hs.p)
	hs.h.Pump(t, func() bool { return hs.p.side.review.loaded && hs.p.side.local.loaded })
	side.Frame()
	requireNamedControls(t, "side view", side)

	detail.Frame()
	requireNamedControls(t, "detail view", detail)

	hs.p.openDiff(d, d.files[0], diffSpec{mode: modePR})
	_, diffTester := hs.awaitDiff(t, diffViewID, diffPath)
	requireNamedControls(t, "diff view", diffTester)

	hs.p.openLogs(d, checkRow{Check: github.Check{Name: "build"}, JobID: 5})
	hs.h.Pump(t, func() bool { return hs.p.logs["pr/acme/widgets/12/log/5"].state != nil })
	logs := viewTester(hs, "pr/acme/widgets/12/log/5")
	if logs == nil {
		t.Fatal("no log view")
	}

	notifs := notificationsTester(hs.p)
	hs.h.Pump(t, func() bool { return hs.p.notifs.loaded })
	notifs.Frame()
	requireNamedControls(t, "notifications", notifs)

	hs.p.openCreate()
	hs.h.Pump(t, func() bool { return !hs.p.createForm.loading })
	requireNamedControls(t, "create view", viewTester(hs, createViewID))

	hs.p.openReviewDialog(d, 1)
	requireNamedControls(t, "review dialog", viewTester(hs, detailID(hs.p.repo, prNumber)))
	hs.p.dialog.open = false
	hs.p.openLabelPicker(d)
	hs.h.Pump(t, func() bool { return !hs.p.picker.loading })
	requireNamedControls(t, "picker", viewTester(hs, detailID(hs.p.repo, prNumber)))
}

func TestStateViewsHaveNamedControls(t *testing.T) {
	hs := newHarness(t)
	hs.token = github.ErrNotLoggedIn
	hs.start(t)
	requireNamedControls(t, "sign in state", sideTester(hs.p))
}
