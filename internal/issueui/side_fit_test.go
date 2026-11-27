package issueui

import (
	"strings"
	"testing"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/github"
	"github.com/egoist/mygo/ui"
)

const (
	narrowSideWidth = 260
	tightSideWidth  = 170
	fitTitle        = "[Buyers] rework the dialog so that the owner can edit every field without leaving the list"
	fitPixelSlack   = 1
)

var fitLabels = []string{"suggestion", "enhancement", "good first issue", "needs design review", "backend"}

func fitIssue() github.Issue {
	issue := testIssue(21, fitTitle)
	issue.Comments = 12
	for _, name := range fitLabels {
		issue.Labels = append(issue.Labels, github.Label{Name: name, Color: "0e8a16"})
	}
	return issue
}

func fitSide(t *testing.T, width int) *ui.Tester {
	t.Helper()
	hs := newHarness(t)
	hs.be.pages[queryAssigned] = []github.IssuePage{{Total: 1, Issues: []github.Issue{fitIssue()}}}
	hs.ready(t)
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() { hs.i.SideView(c) })
	}, width, sideHeight)
	hs.h.pump(t, func() bool { return hs.i.side.sections[0].loaded })
	tt.Frame()
	tt.Frame()
	tt.Frame()
	return tt
}

func TestIssueTitleKeepsItsShareInNarrowSideBars(t *testing.T) {
	var natural float32
	probe := ui.NewTester(func(c *ui.Context) {
		natural, _ = c.MeasureText(0, ui.Span{Text: fitTitle, Size: token.TextBody.Px(), Weight: token.TextBody.Weight()})
	}, narrowSideWidth, sideHeight)
	probe.Frame()
	for _, width := range []int{narrowSideWidth, tightSideWidth} {
		tt := fitSide(t, width)
		title, ok := tt.Find(fitTitle)
		if !ok {
			t.Fatalf("%d px: the title is not drawn: %q", width, tt.Texts())
		}
		avatar, ok := tt.Find("A")
		if !ok {
			t.Fatalf("%d px: the avatar is missing", width)
		}
		count, ok := tt.Find("12")
		if !ok {
			t.Fatalf("%d px: the comment count is missing", width)
		}
		avatarLeft := avatar.X + avatar.W/2 - token.AvatarSize.Px()/2
		if gap := avatarLeft - (title.X + title.W); gap > token.GapRowIcon.Px()+fitPixelSlack {
			t.Errorf("%d px: %.0f px of the row stay empty after the title, want the title to take all room the avatar and the count leave", width, gap)
		}
		if count.X < avatarLeft {
			t.Errorf("%d px: the count at %.0f is left of the avatar at %.0f", width, count.X, avatarLeft)
		}
		if title.W >= natural {
			t.Errorf("%d px: the title is %.0f px wide, want it cut below %.0f px with an ellipsis", width, title.W, natural)
		}
		if tt.HasText("#21 · 3h ago") {
			t.Errorf("%d px: the description is drawn, want it hidden and in the tooltip", width)
		}
		for _, name := range fitLabels {
			if tt.HasText(name) {
				t.Errorf("%d px: the label %q is drawn in the row, want it in the tooltip only", width, name)
			}
		}
	}
}

func TestIssueTooltipListsTheLabels(t *testing.T) {
	tip := issueTooltip(fitIssue())
	if want := strings.Join(fitLabels, ", "); !strings.HasSuffix(tip, want) {
		t.Errorf("tooltip %q does not end with the labels %q", tip, want)
	}
}
