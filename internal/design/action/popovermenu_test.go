package action

import (
	"testing"
	"time"

	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const (
	menuFieldLabel = "Name"
	moreLabel      = "More"
	menuItemLabel  = "First"
	selectLabel    = "Pick"
	selectChoice   = "B"
	hoverSettle    = 2 * time.Second
)

type menuBed struct {
	tt   *ui.Tester
	text string
	pick string
}

func newMenuBed() *menuBed {
	b := &menuBed{pick: "a"}
	b.tt = ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() {
			ui.TextInputBase(c.Key("field"), &b.text).Label(menuFieldLabel)
			MenuIconButton(c, icons.Filter, moreLabel, IconOptions{}, func(m *ui.Menu) {
				Item(m, menuItemLabel, ItemOptions{})
			})
			Select(c, selectLabel, &b.pick, []Option[string]{{"a", "A"}, {"b", selectChoice}}, SelectOptions{})
		})
	}, 400, 300)
	b.tt.Frame()
	return b
}

func (b *menuBed) clickOn(t *testing.T, label string) {
	t.Helper()
	a, ok := b.tt.Access(label)
	if !ok {
		t.Fatalf("no %q", label)
	}
	b.tt.ClickAt(a.Bounds.X+a.Bounds.W/2, a.Bounds.Y+a.Bounds.H/2)
	b.tt.Frame()
}

func (b *menuBed) menuOpen() bool {
	_, ok := b.tt.AccessByRole(ui.RoleMenuItem, menuItemLabel)
	return ok
}

func (b *menuBed) focusedName() string {
	for _, a := range b.tt.AccessAll() {
		if a.Focused {
			return a.Name
		}
	}
	return ""
}

func TestPopoverMenuKeepsTheFocusWhereItWasAndReturnsIt(t *testing.T) {
	b := newMenuBed()
	b.clickOn(t, menuFieldLabel)
	b.clickOn(t, moreLabel)
	if !b.menuOpen() {
		t.Fatal("the menu did not open")
	}
	b.tt.Key(0, ui.KeyEscape)
	b.tt.Frame()
	if b.menuOpen() {
		t.Fatal("Escape left the menu open")
	}
	if got := b.focusedName(); got != menuFieldLabel {
		t.Fatalf("focus is on %q, want the field it was in", got)
	}
}

func TestPopoverMenuClosesWhenTheWindowDeactivates(t *testing.T) {
	b := newMenuBed()
	b.clickOn(t, moreLabel)
	if !b.menuOpen() {
		t.Fatal("the menu did not open")
	}
	b.tt.SetWindowActive(false)
	if b.menuOpen() {
		t.Fatal("the menu stayed open in an inactive window")
	}
}

func TestSelectClosesWhenTheWindowDeactivates(t *testing.T) {
	b := newMenuBed()
	b.clickOn(t, selectLabel)
	if _, ok := b.tt.AccessByRole(ui.RoleOption, selectChoice); !ok {
		t.Fatal("the select did not open")
	}
	b.tt.SetWindowActive(false)
	if _, ok := b.tt.AccessByRole(ui.RoleOption, selectChoice); ok {
		t.Fatal("the select stayed open in an inactive window")
	}
}

func TestPopoverMenuClosesOnAClickOutside(t *testing.T) {
	b := newMenuBed()
	b.clickOn(t, moreLabel)
	b.tt.ClickAt(5, 290)
	b.tt.Frame()
	if b.menuOpen() {
		t.Fatal("a click outside left the menu open")
	}
}

func TestOpeningAMenuClosesTheTooltip(t *testing.T) {
	b := newMenuBed()
	more, _ := b.tt.Access(moreLabel)
	x, y := more.Bounds.X+more.Bounds.W/2, more.Bounds.Y+more.Bounds.H/2
	b.tt.Move(x, y)
	b.tt.Advance(hoverSettle)
	if _, ok := b.tt.AccessByRole(ui.RoleTooltip, moreLabel); !ok {
		t.Fatal("hovering the button showed no tooltip")
	}
	b.tt.ClickAt(x, y)
	b.tt.Frame()
	if _, ok := b.tt.AccessByRole(ui.RoleTooltip, moreLabel); ok {
		t.Fatal("the tooltip stayed under the open menu")
	}
}
