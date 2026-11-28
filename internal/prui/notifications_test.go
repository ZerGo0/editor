package prui

import (
	"errors"
	"testing"
	"time"

	"github.com/ZerGo0/editor/internal/ghshared/ghtest"
	"github.com/ZerGo0/editor/internal/github"
	"github.com/egoist/mygo/ui"
)

func notificationsTester(p *PRs) *ui.Tester {
	return ghtest.SideTester(p.NotificationsActions, p.NotificationsView, p.Overlay)
}

func (hs *harness) withNotifications() {
	hs.be.notifs = NotificationPage{
		Modified: "Thu, 26 Nov 2026 10:00:00 GMT",
		Poll:     90 * time.Second,
		Items: []Notification{
			{ID: "n1", Title: "Add caching", Number: 3, Reason: "review_requested", Unread: true, Updated: testNow.Add(-time.Hour)},
			{ID: "n2", Title: "Fix flaky test", Number: 4, Reason: "mention", Updated: testNow.Add(-2 * time.Hour)},
		},
	}
}

func TestNotificationsLoadAtConnectAndBadge(t *testing.T) {
	hs := newHarness(t)
	hs.withNotifications()
	hs.ready(t)
	hs.h.Pump(t, func() bool { return hs.p.notifs.loaded })
	if hs.p.NotificationsBadge() != 1 {
		t.Errorf("badge %d", hs.p.NotificationsBadge())
	}
	tt := notificationsTester(hs.p)
	ghtest.MustHaveText(t, tt, "Add caching", "acme/widgets #3 · 1h ago", "Fix flaky test", "acme/widgets #4 · 2h ago")
	if _, ok := tt.Access("Unread"); !ok {
		t.Error("the unread marker has no name")
	}
}

func TestNotificationClickOpensAndMarksRead(t *testing.T) {
	hs := newHarness(t)
	hs.withNotifications()
	hs.ready(t)
	hs.h.Pump(t, func() bool { return hs.p.notifs.loaded })
	tt := notificationsTester(hs.p)
	ghtest.MustClick(t, tt, "Add caching")
	if hs.h.Opened[len(hs.h.Opened)-1] != "pr/acme/widgets/3" {
		t.Errorf("opened %v", hs.h.Opened)
	}
	hs.h.Pump(t, func() bool { return hs.be.Count("MarkNotification") == 1 && hs.p.NotificationsBadge() == 0 })
	if hs.be.arg != "n1:false" {
		t.Errorf("%q", hs.be.arg)
	}
	tt.Advance(time.Second)
	ghtest.MustClick(t, tt, "Fix flaky test")
	hs.h.Drain()
	if hs.be.Count("MarkNotification") != 1 {
		t.Error("a read notification was marked again")
	}
}

func TestNotificationMenuMarkDoneAndOpen(t *testing.T) {
	hs := newHarness(t)
	hs.withNotifications()
	hs.ready(t)
	hs.h.Pump(t, func() bool { return hs.p.notifs.loaded })
	tt := notificationsTester(hs.p)
	if err := tt.RightClick("Fix flaky test"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Mark as Done"); err != nil {
		t.Fatalf("%v (menu %q)", err, tt.Menu())
	}
	hs.h.Pump(t, func() bool { return len(hs.p.notifs.items) == 1 })
	if hs.be.arg != "n2:true" {
		t.Errorf("%q", hs.be.arg)
	}
	tt.Frame()
	if tt.HasText("Fix flaky test") {
		t.Error("done notification still listed")
	}
	if err := tt.RightClick("Add caching"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Open on GitHub"); err != nil {
		t.Fatal(err)
	}
	if urls := tt.OpenedURLs(); len(urls) != 1 || urls[0] != "https://github.com/acme/widgets/pull/3" {
		t.Errorf("urls %v", urls)
	}
	if err := tt.RightClick("Add caching"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Mark as Read"); err != nil {
		t.Fatal(err)
	}
	hs.h.Pump(t, func() bool { return hs.p.NotificationsBadge() == 0 })
}

func TestNotificationMarkFailureReportsError(t *testing.T) {
	hs := newHarness(t)
	hs.withNotifications()
	hs.be.errs["MarkNotification"] = errors.New("github: nope")
	hs.ready(t)
	hs.h.Pump(t, func() bool { return hs.p.notifs.loaded })
	hs.p.markNotification(hs.p.notifs.items[0], true)
	hs.h.Pump(t, func() bool { return hs.h.LastError() != "" })
	if len(hs.p.notifs.items) != 2 {
		t.Error("the notification was removed after a failure")
	}
}

func TestNotificationsEmptyAndError(t *testing.T) {
	hs := newHarness(t)
	hs.ready(t)
	hs.h.Pump(t, func() bool { return hs.p.notifs.loaded })
	tt := notificationsTester(hs.p)
	ghtest.MustHaveText(t, tt, "You are all caught up")

	failing := newHarness(t)
	failing.be.errs["Notifications"] = &github.APIError{Status: 403, Message: "no scope"}
	failing.ready(t)
	failing.h.Pump(t, func() bool { return failing.p.notifs.err != nil })
	tt = notificationsTester(failing.p)
	ghtest.MustHaveText(t, tt, "GitHub denied access. Check that your account can access this repository.", "Retry")
	failing.be.clearErr("Notifications")
	failing.withNotifications()
	ghtest.MustClick(t, tt, "Retry")
	failing.h.Pump(t, func() bool { return failing.p.notifs.loaded && len(failing.p.notifs.items) == 2 })
}

func TestNotificationsPollingObeysInterval(t *testing.T) {
	hs := newHarness(t)
	hs.withNotifications()
	hs.ready(t)
	hs.h.Pump(t, func() bool { return hs.p.notifs.loaded })
	now := testNow
	hs.p.env.Now = func() time.Time { return now }
	hs.p.notifs.fetchedAt = now
	tt := notificationsTester(hs.p)
	hs.h.Drain()
	base := hs.be.Count("Notifications")
	now = now.Add(80 * time.Second)
	tt.Frame()
	hs.h.Drain()
	if hs.be.Count("Notifications") != base {
		t.Error("polled before X-Poll-Interval elapsed")
	}
	hs.be.notifs.NotModified = true
	now = now.Add(11 * time.Second)
	tt.Frame()
	hs.h.Pump(t, func() bool { return hs.be.Count("Notifications") > base && !hs.p.notifs.loading })
	if hs.be.arg != "Thu, 26 Nov 2026 10:00:00 GMT" {
		t.Errorf("If-Modified-Since %q", hs.be.arg)
	}
	if len(hs.p.notifs.items) != 2 {
		t.Error("a 304 answer dropped the list")
	}
}

func TestNotificationsRefreshButton(t *testing.T) {
	hs := newHarness(t)
	hs.withNotifications()
	hs.ready(t)
	hs.h.Pump(t, func() bool { return hs.p.notifs.loaded })
	tt := notificationsTester(hs.p)
	hs.h.Drain()
	base := hs.be.Count("Notifications")
	ghtest.MustClick(t, tt, "Refresh Notifications")
	hs.h.Pump(t, func() bool { return hs.be.Count("Notifications") > base })
	if hs.be.arg != "" {
		t.Errorf("a manual refresh must not be conditional: %q", hs.be.arg)
	}
}

func TestNotificationsViewBeforeReady(t *testing.T) {
	hs := newHarness(t)
	hs.token = github.ErrNoGH
	hs.start(t)
	tt := notificationsTester(hs.p)
	ghtest.MustHaveText(t, tt, "GitHub CLI not found")
	if hs.p.NotificationsBadge() != 0 {
		t.Error("badge before ready")
	}
}

func TestNotificationsEnterOpensSelected(t *testing.T) {
	hs := newHarness(t)
	hs.withNotifications()
	hs.ready(t)
	hs.h.Pump(t, func() bool { return hs.p.notifs.loaded })
	tt := notificationsTester(hs.p)
	ghtest.MustClick(t, tt, "Fix flaky test")
	before := len(hs.h.Opened)
	tt.Key(0, ui.KeyEnter)
	if len(hs.h.Opened) != before+1 || hs.h.Opened[before] != "pr/acme/widgets/4" {
		t.Errorf("opened %v", hs.h.Opened)
	}
}
