package insights

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

const (
	testViewW = 480
	testViewH = 320
)

type opened struct {
	path      string
	line, col int
}

func testGroups() []Group {
	return []Group{
		{Path: "/p/main.go", Entries: []Entry{{Title: "'{' is not closed", Detail: "brackets", Line: 2, Col: 12}}},
		{Path: "/p/web/app.ts", Entries: []Entry{{Title: "Unmatched ')'", Detail: "brackets"}, {Title: "Tab used", Detail: "yaml"}}},
	}
}

func newTestList(t *testing.T) (*List, *ui.Tester, *[]opened) {
	t.Helper()
	l := &List{}
	var got []opened
	rel := func(p string) string { return p[len("/p/"):] }
	tt := ui.NewTester(func(c *ui.Context) {
		l.View(c, "Problems", "No problems.", 1, testGroups, rel, func(path string, line, col int) {
			got = append(got, opened{path, line, col})
		})
	}, testViewW, testViewH)
	return l, tt, &got
}

func TestListGroupsAndOpens(t *testing.T) {
	_, tt, got := newTestList(t)
	for _, name := range []string{"main.go", "web/app.ts", "'{' is not closed", "Tab used"} {
		if _, ok := tt.AccessByRole(ui.RoleTreeItem, name); !ok {
			t.Fatalf("row %q is not shown", name)
		}
	}
	if !tt.HasText("[Ln 3, Col 13]") {
		t.Fatal("the position is not shown")
	}
	if err := tt.Click("'{' is not closed"); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 1 || (*got)[0] != (opened{"/p/main.go", 2, 12}) {
		t.Fatalf("opened %v", *got)
	}
}

func TestListFilterAndCollapse(t *testing.T) {
	l, tt, _ := newTestList(t)
	l.Filter = "tab"
	tt.Frame()
	if _, ok := tt.AccessByRole(ui.RoleTreeItem, "main.go"); ok {
		t.Fatal("the filter keeps main.go")
	}
	if _, ok := tt.AccessByRole(ui.RoleTreeItem, "Tab used"); !ok {
		t.Fatal("the filter hides a match")
	}
	l.Filter = "nothing matches"
	tt.Frame()
	if !tt.HasText("No matching results") {
		t.Fatal("no empty filter state")
	}
	if err := tt.Click("Clear Filter"); err != nil {
		t.Fatal(err)
	}
	l.CollapseAll()
	tt.Frame()
	if _, ok := tt.AccessByRole(ui.RoleTreeItem, "Tab used"); ok {
		t.Fatal("Collapse All keeps entries")
	}
	if _, ok := tt.AccessByRole(ui.RoleTreeItem, "web/app.ts"); !ok {
		t.Fatal("Collapse All hides groups")
	}
}

func TestListCopiesEntry(t *testing.T) {
	_, tt, _ := newTestList(t)
	if err := tt.RightClick("Tab used"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Copy Message"); err != nil {
		t.Fatal(err)
	}
	if got := tt.Clipboard(); got != "Tab used" {
		t.Fatalf("clipboard %q", got)
	}
	if err := tt.RightClick("Tab used"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Copy"); err != nil {
		t.Fatal(err)
	}
	if got := tt.Clipboard(); got != "web/app.ts:1:1: Tab used (yaml)" {
		t.Fatalf("clipboard %q", got)
	}
}
