package prui

import (
	"testing"

	"github.com/ZerGo0/editor/internal/design/list"
	"github.com/ZerGo0/editor/internal/ghshared/ghtest"
	"github.com/egoist/mygo/ui"
)

const (
	narrowSideWidth = 260
	tightSideWidth  = 170
	fitTitle        = "Add a retry with exponential backoff to the webhook delivery worker and cover it with tests"
	fitShareSlack   = 0.005
)

func TestNotificationTitleKeepsItsShareInNarrowSideBars(t *testing.T) {
	natural := ghtest.NaturalWidth(fitTitle, narrowSideWidth, ghtest.SideHeight)
	for _, width := range []int{narrowSideWidth, tightSideWidth} {
		hs := newHarness(t)
		hs.be.notifs = NotificationPage{Items: []Notification{{ID: "n1", Title: fitTitle, Number: 3, Reason: "review_requested", Updated: testNow}}}
		hs.ready(t)
		hs.h.Pump(t, func() bool { return hs.p.notifs.loaded })
		tt := ui.NewTester(func(c *ui.Context) {
			ui.Column(c).Fill().Children(func() { hs.p.NotificationsView(c) })
		}, width, ghtest.SideHeight)
		tt.Frame()
		tt.Frame()
		title, ok := tt.Find(fitTitle)
		if !ok {
			t.Fatalf("%d px: the title is not drawn: %q", width, tt.Texts())
		}
		if share := title.W / float32(width); share < list.TitleShare-fitShareSlack {
			t.Errorf("%d px: the title takes %.0f%% of the row, want at least %.0f%%", width, share*100, list.TitleShare*100)
		}
		if title.W >= natural {
			t.Errorf("%d px: the title is %.0f px wide, want it cut below %.0f px with an ellipsis", width, title.W, natural)
		}
		description := "#3" + prMetaSeparator + reasonLabel("review_requested") + prMetaSeparator + "just now"
		if tt.HasText(description) {
			t.Errorf("%d px: the description is drawn, want it hidden and in the tooltip", width)
		}
	}
}
