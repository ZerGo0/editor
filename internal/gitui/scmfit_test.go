package gitui

import (
	"strings"
	"testing"
	"time"

	"github.com/ZerGo0/editor/internal/design/list"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/testutil"
	"github.com/egoist/mygo/ui"
)

const (
	sideWidth        = 260
	tightSideWidth   = 170
	sideHeightPx     = 700
	fitBranch        = "zergo0/Dev-1498-with-a-name-that-is-much-too-long-for-the-side-bar"
	fitUpstream      = "origin/" + fitBranch
	fitSubject       = "feat: rework the Buyers dialog so that the owner can edit every field without leaving the list"
	fitSubjectPrefix = "feat: rework the Buyers dialog"
	fitAuthor        = "Zergo Zerg-Rush"
	shareSlack       = 0.005
	ellipsisMark     = "…"
)

func fitGraph(t *testing.T, width int) *ui.Tester {
	t.Helper()
	now := time.Now()
	r := &fakeRepo{
		status:  &git.Status{Branch: fitBranch, OID: "c2", Upstream: fitUpstream},
		remotes: []git.Remote{{Name: "origin"}},
		commits: []git.Commit{
			{OID: "c2", Parents: []string{"c1"}, Author: fitAuthor, AuthorTime: now, Subject: fitSubject,
				Refs: []string{headRefMarker + fitBranch, fitUpstream, "origin/other-branch", "feature/one", "feature/two", "tag: v1.0.0"}},
			{OID: "c1", Author: "Ada", AuthorTime: now, Subject: "First change"},
		},
	}
	g, h := newFakeGit(t, r)
	tt := ui.NewTester(g.SideView, width, sideHeightPx)
	h.drain(t, func() bool { tt.Frame(); return tt.HasText("First change") })
	tt.Frame()
	tt.Frame()
	return tt
}

func TestGraphSubjectKeepsItsShareInNarrowSideBars(t *testing.T) {
	natural := testutil.NaturalTextWidth(sideWidth, sideHeightPx, fitSubject)
	for _, width := range []int{sideWidth, tightSideWidth} {
		tt := fitGraph(t, width)
		subject, ok := tt.AccessByRole(ui.RoleText, fitSubject)
		if !ok {
			t.Fatalf("%d px: the subject is not drawn", width)
		}
		rect := subject.Bounds
		if share := rect.W / float32(width); share < list.TitleShare-shareSlack {
			t.Errorf("%d px: the subject takes %.0f%% of the row, want at least %.0f%%", width, share*100, list.TitleShare*100)
		}
		if rect.W >= natural {
			t.Errorf("%d px: the subject is %.0f px wide, want it cut below %.0f px with an ellipsis", width, rect.W, natural)
		}
		if _, drawn := tt.AccessByRole(ui.RoleText, fitAuthor); drawn {
			t.Errorf("%d px: the author is drawn, want it hidden", width)
		}
		if name, ok := tt.Access(fitBranch); ok && name.Bounds.X+name.Bounds.W > float32(width) {
			t.Errorf("%d px: the branch pill ends at %.0f, outside the row", width, name.Bounds.X+name.Bounds.W)
		}
		if drawnText(tt, "tag: v1.0.0") || drawnText(tt, "feature/one") {
			t.Errorf("%d px: refs without a color are drawn in the row", width)
		}
	}
}

func drawnText(tt *ui.Tester, s string) bool {
	_, ok := tt.AccessByRole(ui.RoleText, s)
	return ok
}

func TestGraphShowsTheCurrentBranchAsAnIconPill(t *testing.T) {
	tt := fitGraph(t, sideWidth)
	pill, ok := tt.Access(fitBranch)
	if !ok {
		t.Fatal("the current branch pill is not drawn")
	}
	if limit := token.IconSizeGlyph.Px() + 2*(token.PadChipX.Px()+token.BorderWidth.Px()); pill.Bounds.W > limit {
		t.Errorf("the pill is %.0f px wide, want an icon only (%.0f px)", pill.Bounds.W, limit)
	}
}

func TestGraphKeepsAShortSubjectBesideItsAuthor(t *testing.T) {
	tt := fitGraph(t, sideWidth)
	author, ok := tt.AccessByRole(ui.RoleText, "Ada")
	if natural := testutil.NaturalTextWidth(sideWidth, sideHeightPx, "First change"); !ok || author.Bounds.W <= 0 || author.Bounds.X < natural {
		t.Errorf("author = %+v, want it shown after the %.0f px subject", author.Bounds, natural)
	}
}

func TestCommitPlaceholderIsOneLineWithAnEllipsis(t *testing.T) {
	for _, width := range []int{sideWidth, tightSideWidth} {
		tt := fitGraph(t, width)
		in, ok := tt.Access("Commit Message")
		if !ok {
			t.Fatalf("%d px: no commit message field", width)
		}
		if !strings.HasSuffix(in.Placeholder, ellipsisMark) {
			t.Errorf("%d px: the placeholder %q does not end with an ellipsis", width, in.Placeholder)
		}
		if !strings.HasPrefix(in.Placeholder, "Message (") {
			t.Errorf("%d px: the placeholder %q lost its start", width, in.Placeholder)
		}
		if h := in.Bounds.H; h > token.ControlHeight.Px()*2 {
			t.Errorf("%d px: the empty field is %.0f px high, want one line", width, h)
		}
	}
}
