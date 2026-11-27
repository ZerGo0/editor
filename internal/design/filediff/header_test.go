package filediff

import (
	"testing"

	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const (
	testPath    = "internal/ui/widget.go"
	testWidth   = 900
	testHeight  = 120
	testAdds    = 12
	testDels    = 3
	testOldPath = "internal/ui/old.go"
)

func TestHeaderTogglesAndMarksViewed(t *testing.T) {
	open, viewed := true, false
	tt := ui.NewTester(func(c *ui.Context) {
		Header(c, HeaderOptions{
			Path: testPath, Status: token.GitModified, Adds: testAdds, Dels: testDels, Open: open, Viewed: viewed,
			OnToggle: func() { open = !open },
			OnViewed: func(on bool) { viewed = on },
		})
	}, testWidth, testHeight)
	for _, want := range []string{"widget.go", "internal/ui/", "+12", "-3"} {
		if !tt.HasText(want) {
			t.Errorf("no %q in %v", want, tt.Texts())
		}
	}
	if err := tt.Click("Collapse " + testPath); err != nil || open {
		t.Fatalf("collapse: %v, open %v", err, open)
	}
	if err := tt.Click("Viewed"); err != nil || !viewed {
		t.Fatalf("viewed: %v, viewed %v", err, viewed)
	}
}

func TestHeaderShowsRenamesAndHidesUnknownCounts(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) {
		Header(c, HeaderOptions{Path: testPath, OldPath: testOldPath, Status: token.GitRenamed, Adds: CountHidden, Dels: CountHidden})
	}, testWidth, testHeight)
	if !tt.HasText(testOldPath + " \u2192 ") {
		t.Errorf("no rename in %v", tt.Texts())
	}
	if tt.HasText("+0") {
		t.Errorf("counts shown: %v", tt.Texts())
	}
	if _, ok := tt.AccessByRole(ui.RoleCheckBox, "Viewed"); ok {
		t.Error("Viewed offered without a handler")
	}
}

func TestHeaderActionsAreInAToolbar(t *testing.T) {
	ran := false
	tt := ui.NewTester(func(c *ui.Context) {
		Header(c, HeaderOptions{Path: testPath, Open: true, Adds: 1, Dels: 1, Actions: func(c *ui.Context) {
			action.IconButton(c, icons.Plus, "Stage Changes", action.IconOptions{}).OnClick(func() { ran = true })
		}})
	}, testWidth, testHeight)
	if _, ok := tt.AccessByRole(ui.RoleToolbar, testPath+" Actions"); !ok {
		t.Fatalf("no toolbar: %v", tt.Texts())
	}
	if err := tt.Click("Stage Changes"); err != nil || !ran {
		t.Fatalf("action: %v, ran %v", err, ran)
	}
}
