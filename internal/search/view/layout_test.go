package searchview

import (
	"math"
	"testing"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/egoist/mygo/ui"
)

const (
	narrowSidebarW = 170
	layoutTol      = 0.5
)

func openLayoutFixture(t *testing.T, width int) *fixture {
	t.Helper()
	f := newFixture(t, map[string]string{"a.go": "needle\nneedle\n", "sub/b.go": "needle\n"})
	f.view.st.ShowReplace = true
	f.view.st.ShowFilters = true
	f.tt.SetSize(width, 640)
	f.search(ScopeText, "needle")
	return f
}

func box(t *testing.T, f *fixture, role ui.Role, name string) ui.Rect {
	t.Helper()
	a, ok := f.tt.AccessByRole(role, name)
	if !ok {
		t.Fatalf("no %v named %q: %q", role, name, f.tt.Texts())
	}
	return a.Bounds
}

func near(a, b float32) bool { return math.Abs(float64(a-b)) <= layoutTol }

func TestWidgetMatchesVSCodeLayout(t *testing.T) {
	f := openLayoutFixture(t, 360)
	chevron := box(t, f, ui.RoleButton, "Toggle Replace")
	find := box(t, f, ui.RoleTextField, "Search")
	replace := box(t, f, ui.RoleTextField, "Replace")
	regex := box(t, f, ui.RoleToggleButton, "Use Regular Expression")
	replaceAll := box(t, f, ui.RoleToggleButton, "Replace All")
	details := box(t, f, ui.RoleToggleButton, "Toggle Search Details")
	if !near(chevron.X, token.GapInline.Px()) || chevron.H != token.IconButtonSize.Px() {
		t.Errorf("replace chevron %v", chevron)
	}
	mid := (find.Y + replace.Y + replace.H) / 2
	if !near(chevron.Y+chevron.H/2, mid) {
		t.Errorf("chevron centre %.1f is not centred across the find and replace rows (%.1f)", chevron.Y+chevron.H/2, mid)
	}
	if find.X != replace.X {
		t.Errorf("find and replace boxes start at %v and %v", find.X, replace.X)
	}
	if !near(regex.X+regex.W, replaceAll.X+replaceAll.W) {
		t.Errorf("find toggles end at %v, replace toggles at %v", regex.X+regex.W, replaceAll.X+replaceAll.W)
	}
	if details.X != regex.X || details.W != regex.W {
		t.Errorf("details toggle %v is not aligned with the find toggles %v", details, regex)
	}
	if find.X+find.W > regex.X || replace.X+replace.W > replaceAll.X {
		t.Errorf("toggles are not inside the boxes: %v %v %v %v", find, regex, replace, replaceAll)
	}
}

func TestStatusLineKeepsRoomAtNarrowSidebar(t *testing.T) {
	f := openLayoutFixture(t, narrowSidebarW)
	status := box(t, f, ui.RoleStatus, "3 results in 2 files")
	list := box(t, f, ui.RoleTree, labelResults)
	if status.W < token.ColumnMedium.Px() {
		t.Errorf("the result count is squeezed to %v px", status.W)
	}
	if status.X+status.W > narrowSidebarW {
		t.Errorf("the result count leaves the sidebar: %v", status)
	}
	if list.Y < status.Y+status.H {
		t.Errorf("results start at %v, above the message line end %v", list.Y, status.Y+status.H)
	}
}

func TestEnterSearchesAndControlEnterAddsLine(t *testing.T) {
	f := openLayoutFixture(t, 360)
	f.focusField()
	f.tt.Type("a")
	f.tt.Key(ui.Ctrl, ui.KeyEnter)
	f.tt.Type("b")
	if f.view.st.Query != "a\nb" {
		t.Fatalf("Ctrl+Enter did not insert a line: %q", f.view.st.Query)
	}
	f.view.replaceField.Focus()
	f.tt.Frame()
	f.tt.Type("x")
	f.tt.Key(ui.Ctrl, ui.KeyEnter)
	f.tt.Type("y")
	if f.view.st.Replace != "x\ny" {
		t.Fatalf("Ctrl+Enter did not insert a line in the replace box: %q", f.view.st.Replace)
	}
}

func TestReplaceControlsAreIconsWithShortcuts(t *testing.T) {
	f := openLayoutFixture(t, 360)
	for _, name := range []string{"Replace All", "Preserve Case"} {
		a, ok := f.tt.AccessByRole(ui.RoleToggleButton, name)
		if !ok || a.Description == "" {
			t.Errorf("%s: %+v %v", name, a, ok)
		}
	}
	if a, _ := f.tt.AccessByRole(ui.RoleToggleButton, "Replace All"); a.Description != "Replace All ("+keysReplaceAll+")" {
		t.Errorf("Replace All tooltip %q", a.Description)
	}
	f.view.selected = f.view.firstSelectable()
	if a, ok := f.tt.Access("Replace"); ok && a.Role == ui.RoleButton && a.Description != "Replace ("+keysReplace+")" {
		t.Errorf("Replace tooltip %q", a.Description)
	}
}
