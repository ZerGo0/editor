package chrome

import (
	"slices"
	"testing"

	"github.com/ZerGo0/editor/internal/design/action"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const (
	testWidth        = 600
	testHeight       = 200
	narrowStripWidth = 300
)

type history map[string]action.CommandInfo

func (h history) Lookup(id string) (action.CommandInfo, bool) {
	info, ok := h[id]
	return info, ok
}

func (h history) Run(string) {}

func TestNavButtons(t *testing.T) {
	action.SetCommands(history{"back": {Title: "Go Back", Keys: "⌃-"}, "forward": {Title: "Go Forward", Disabled: true}})
	t.Cleanup(func() { action.SetCommands(nil) })
	menu := func(m *ui.Menu) { action.Item(m, "main.go", action.ItemOptions{}) }
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Row(c).Children(func() {
			NavButton(c, icons.ArrowLeft, "back", menu)
			NavButton(c, icons.ArrowRight, "forward", menu)
		})
	}, testWidth, testHeight)
	back, ok := tt.AccessByRole(ui.RoleButton, "Go Back")
	if !ok || back.Disabled || back.Description != "Go Back (⌃-)" {
		t.Fatalf("back = %+v", back)
	}
	if fwd, _ := tt.Access("Go Forward"); !fwd.Disabled {
		t.Fatal("forward shows enabled with no history")
	}
}

func TestNavButtonsSkipHistoryAnchor(t *testing.T) {
	action.SetCommands(history{"back": {Title: "Go Back", Disabled: true}, "forward": {Title: "Go Forward", Disabled: true}})
	t.Cleanup(func() { action.SetCommands(nil) })
	menu := func(m *ui.Menu) { action.Item(m, "main.go", action.ItemOptions{}) }
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Row(c).Children(func() {
			NavButton(c, icons.ArrowLeft, "back", menu)
			NavButton(c, icons.ArrowRight, "forward", menu)
			ui.Button(c, "Next")
		})
	}, testWidth, testHeight)
	tt.Key(0, ui.KeyTab)
	if !tt.Focused("Next") {
		t.Fatal("Tab stops at a hidden history anchor")
	}
}

func TestActivityBarPassesFocusToSelectedTab(t *testing.T) {
	selected, focus := 1, false
	items := []Activity{{Icon: icons.Folder, Label: "Explorer"}, {Icon: icons.Search, Label: "Search"}}
	tt := ui.NewTester(func(c *ui.Context) {
		bar := ActivityBar(c, "Activity Bar", &selected, items)
		if focus {
			focus = false
			bar.Focus()
		}
	}, testWidth, testHeight)
	focus = true
	tt.Frame()
	tt.Frame()
	if !tt.Focused("Search") || tt.Focused("Explorer") {
		t.Fatal("focus stays on the list, want the selected tab")
	}
}

func stripTester(selected *int, tabs *[]Tab, o TabStripOptions, width int) *ui.Tester {
	o.Label = "Editors"
	return ui.NewTester(func(c *ui.Context) { TabStrip(c, selected, *tabs, o) }, width, testHeight)
}

func TestTabStrip(t *testing.T) {
	selected, closed := 0, -1
	tabs := []Tab{{Label: "a.go"}, {Label: "b.go", Dirty: true}}
	tt := stripTester(&selected, &tabs, TabStripOptions{OnClose: func(i int) { closed = i }}, testWidth)
	if info, ok := tt.AccessByRole(ui.RoleTab, "a.go"); !ok || !info.Selected {
		t.Fatalf("tab = %+v, want the chosen tab", info)
	}
	if _, ok := tt.AccessByRole(ui.RoleTab, "b.go"+accessSeparator+dirtyLabel); !ok {
		t.Fatal("dirty tab shows no dirty mark")
	}
	if got := (Tab{Label: "c.go", Dirty: true, Deleted: true, ReadOnly: true}).accessName(); got != "c.go"+accessSeparator+dirtyLabel+accessSeparator+deletedLabel+accessSeparator+readOnlyText {
		t.Fatalf("access name = %q", got)
	}
	if err := tt.Click(closeLabel + " a.go"); err != nil {
		t.Fatal(err)
	}
	if closed != 0 {
		t.Fatalf("closed %d, want 0", closed)
	}
}

func TestTabStripOffersTheOpenedEditorsWhenTabsOverflow(t *testing.T) {
	selected := 0
	many := []Tab{{Label: "alpha.go"}, {Label: "bravo.go"}, {Label: "charlie.go"}, {Label: "delta.go"}, {Label: "echo.go"}, {Label: "foxtrot.go"}}
	few := many[:1]
	tabs := many
	tt := stripTester(&selected, &tabs, TabStripOptions{OnSelect: func(int) {}}, narrowStripWidth)
	tt.Frame()
	tt.Frame()
	if _, ok := tt.Access(openedLabel); !ok {
		t.Fatal("no opened editors button for overflowing tabs")
	}
	tabs = few
	tt.Frame()
	tt.Frame()
	if _, ok := tt.Access(openedLabel); ok {
		t.Fatal("opened editors button shows without overflow")
	}
}

func TestTabStripMiddleClickClosesTabAtOnce(t *testing.T) {
	selected := 0
	tabs := []Tab{{Label: "a.go"}, {Label: "b.go"}}
	tt := stripTester(&selected, &tabs, TabStripOptions{OnAuxClick: func(i int) { tabs = slices.Delete(tabs, i, i+1) }}, testWidth)
	at, ok := tt.Find("a.go")
	if !ok {
		t.Fatal("no a.go tab")
	}
	x, y := at.X+at.W/2, at.Y+at.H/2
	tt.Move(x, y)
	tt.PressButton(middleButton, x, y)
	tt.ReleaseButton(middleButton, x, y)
	if _, open := tt.AccessByRole(ui.RoleTab, "a.go"); open {
		t.Fatal("a.go tab still shows after a middle click")
	}
}

func TestStatusItemIsAButtonOnlyWhenItActs(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Row(c).Children(func() {
			StatusItem(c, "UTF-8", StatusOptions{Label: "File Encoding: UTF-8", OnClick: func() {}})
			StatusItem(c, "Large file", StatusOptions{Tooltip: "This file is large"})
		})
	}, 600, 40)
	tt.Frame()
	if _, ok := tt.AccessByRole(ui.RoleButton, "File Encoding: UTF-8"); !ok {
		t.Fatal("a status item with an action is not a button")
	}
	if _, ok := tt.AccessByRole(ui.RoleStatus, "Large file"); !ok {
		t.Fatal("a status item without an action is not a status")
	}
	if _, ok := tt.AccessByRole(ui.RoleButton, "Large file"); ok {
		t.Fatal("a status item without an action is a button")
	}
}
