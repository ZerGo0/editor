package list

import (
	"strings"
	"testing"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/ZerGo0/editor/internal/testutil"
	"github.com/egoist/mygo/ui"
)

const (
	sideWidth   = 260
	tightWidth  = 170
	titlePrefix = "feat: rework the Buyers dialog"
	longTitle   = titlePrefix + " so that the owner can edit every field without leaving the list"
	longRefName = "zergo0/Dev-1498-an-extremely-long-branch-name-that-cannot-fit"
	shareSlack  = 0.005
)

var iconPillWidth = token.IconSizeGlyph.Px() + 2*(token.PadChipX.Px()+token.BorderWidth.Px())

func manyPills() []Pill {
	return []Pill{
		{Text: longRefName, Icon: icons.GitBranch, Tooltip: longRefName},
		{Text: "2", Short: "2", Icon: icons.Github, Tooltip: "origin/a, origin/b"},
		{Text: "suggestion", Tooltip: "suggestion"},
	}
}

func fitTester(width int, it Item) *ui.Tester {
	selected := -1
	tt := ui.NewTester(func(c *ui.Context) {
		View(c, &ui.ListState{Selected: &selected, Label: fixedName}, 1, ViewOptions{Label: "Rows"}, func(int) Item { return it })
	}, width, testHeight)
	tt.Frame()
	tt.Frame()
	return tt
}

func drawn(tt *ui.Tester, prefix string) (string, ui.Rect, bool) {
	for _, text := range tt.Texts() {
		if strings.HasPrefix(text, prefix) {
			r, ok := tt.Find(text)
			return text, r, ok
		}
	}
	return "", ui.Rect{}, false
}

func TestTitleKeepsItsShareOfTheRow(t *testing.T) {
	for _, width := range []int{sideWidth, tightWidth} {
		tt := fitTester(width, Item{
			Label:         longTitle,
			Description:   "#128 · 3 days ago",
			Icon:          icons.Dot,
			Pills:         manyPills(),
			Tooltip:       longTitle,
			TrailingWidth: token.AvatarSize.Px(),
			Trailing: func(c *ui.Context) {
				ui.Box(c).Size(token.AvatarSize.Px(), token.AvatarSize.Px()).Shrink(0)
			},
		})
		_, title, ok := drawn(tt, titlePrefix)
		if !ok {
			t.Fatalf("%d px: the title is not drawn", width)
		}
		if share := title.W / float32(width); share < TitleShare-shareSlack {
			t.Errorf("%d px: the title takes %.0f%% of the row, want at least %.0f%%", width, share*100, TitleShare*100)
		}
		if natural := testutil.NaturalTextWidth(sideWidth, testHeight, longTitle); title.W >= natural {
			t.Errorf("%d px: the title is %.0f px wide, want it cut below its natural %.0f px", width, title.W, natural)
		}
		if tt.HasText("#128 · 3 days ago") {
			t.Errorf("%d px: the description is drawn, want it hidden", width)
		}
		if _, pill, ok := drawn(tt, longRefName); ok && pill.W > iconPillWidth {
			t.Errorf("%d px: the ref name pill is %.0f px wide, want an icon only (%.0f px)", width, pill.W, iconPillWidth)
		}
		if tt.HasText("suggestion") {
			t.Errorf("%d px: the label pill is visible, want it dropped", width)
		}
	}
}

func TestPillsShrinkToIconsBeforeTheyAreDropped(t *testing.T) {
	const roomy = 480
	tt := fitTester(roomy, Item{Label: "short", Pills: manyPills()[:2]})
	_, pill, ok := drawn(tt, "zergo0")
	if !ok {
		t.Fatalf("%d px: the ref name pill is not drawn", roomy)
	}
	if limit := token.PillNameMaxWidth.Px(); pill.W > limit {
		t.Errorf("the ref name is %.0f px wide, want at most %.0f", pill.W, limit)
	}
	tt = fitTester(sideWidth, Item{Label: longTitle, Pills: manyPills()[:2], LeadingWidth: token.GraphLaneWidth.Px(), Leading: func(c *ui.Context) {
		ui.Box(c).Size(token.GraphLaneWidth.Px(), token.RowHeight.Px()).Shrink(0)
	}})
	if _, pill, ok := drawn(tt, longRefName); !ok || pill.W > iconPillWidth {
		t.Errorf("%d px: the ref pill is %.0f px wide, want an icon only (%.0f px)", sideWidth, pill.W, iconPillWidth)
	}
	if _, title, ok := drawn(tt, titlePrefix); !ok || title.W/float32(sideWidth) < TitleShare-shareSlack {
		t.Errorf("%d px: the title is too short next to the icon pills", sideWidth)
	}
}
