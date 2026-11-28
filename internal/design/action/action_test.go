package action

import (
	"testing"

	"github.com/ZerGo0/editor/internal/design/token"
	"github.com/ZerGo0/editor/internal/icons"
	"github.com/egoist/mygo/ui"
)

const (
	testWidth  = 400
	testHeight = 200
)

type commandsStub struct {
	infos map[string]CommandInfo
	ran   []string
}

func (s *commandsStub) Lookup(id string) (CommandInfo, bool) {
	info, ok := s.infos[id]
	return info, ok
}

func (s *commandsStub) Run(id string) { s.ran = append(s.ran, id) }

func TestIconButtonLabelAndTooltip(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) {
		IconButton(c, icons.Refresh, "Refresh", IconOptions{Keys: "⌘R"})
	}, testWidth, testHeight)
	info, ok := tt.AccessByRole(ui.RoleButton, "Refresh")
	if !ok {
		t.Fatal("icon button has no accessible name")
	}
	if info.Description != "Refresh (⌘R)" {
		t.Fatalf("tooltip = %q, want the label with its keys", info.Description)
	}
}

func TestToggleIconButtonKeyboard(t *testing.T) {
	on := false
	tt := ui.NewTester(func(c *ui.Context) {
		ToggleIconButton(c, icons.CaseSensitive, "Match Case", &on, IconOptions{})
	}, testWidth, testHeight)
	tt.Key(0, ui.KeyTab)
	tt.Key(0, ui.KeySpace)
	if !on {
		t.Fatal("Space does not turn the toggle on")
	}
	if info, _ := tt.Access("Match Case"); !info.Checked {
		t.Fatal("toggle does not report its pressed state")
	}
}

func TestExpandButton(t *testing.T) {
	open := false
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Row(c).Height(token.ToolbarHeight.Px()).Children(func() {
			if ExpandButton(c, "Toggle Replace", open, IconOptions{Keys: "⌥⌘F"}).Clicked() {
				open = !open
			}
		})
	}, testWidth, testHeight)
	info, ok := tt.AccessByRole(ui.RoleDisclosure, "Toggle Replace")
	if !ok || info.Expanded || info.Description != "Toggle Replace (⌥⌘F)" {
		t.Fatalf("expand button = %+v, want a collapsed button with its keys in the tooltip", info)
	}
	if info.Bounds.W != token.HitTargetMin.Px() || info.Bounds.H != token.ToolbarHeight.Px() {
		t.Fatalf("expand button is %vx%v, want %v wide and as high as its row", info.Bounds.W, info.Bounds.H, token.HitTargetMin.Px())
	}
	tt.Key(0, ui.KeyTab)
	tt.Key(0, ui.KeySpace)
	tt.Frame()
	if info, _ := tt.AccessByRole(ui.RoleDisclosure, "Toggle Replace"); !open || !info.Expanded {
		t.Fatal("Space does not expand the button")
	}
}

func TestCheckboxAndSwitch(t *testing.T) {
	checked, on := false, false
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Children(func() {
			Checkbox(c, "Auto Save", &checked, ChoiceOptions{})
			Switch(c, "Minimap", &on, ChoiceOptions{})
		})
	}, testWidth, testHeight)
	if _, ok := tt.AccessByRole(ui.RoleCheckBox, "Auto Save"); !ok {
		t.Fatal("checkbox has no check box role")
	}
	if err := tt.Click("Auto Save"); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyTab)
	tt.Key(0, ui.KeySpace)
	if !checked || !on {
		t.Fatalf("checked=%v on=%v, want both on", checked, on)
	}
}

func TestBareSwitchMeetsHitTarget(t *testing.T) {
	on := false
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Children(func() { Switch(c, "", &on, ChoiceOptions{ID: "bare"}).Label("Minimap") })
	}, testWidth, testHeight)
	sw, ok := tt.AccessByRole(ui.RoleSwitch, "Minimap")
	if !ok || sw.Bounds.H < token.HitTargetMin.Px() {
		t.Fatalf("switch = %+v, want at least %v high", sw.Bounds, token.HitTargetMin.Px())
	}
}

func TestCommandButtons(t *testing.T) {
	stub := &commandsStub{infos: map[string]CommandInfo{
		"refresh": {Title: "Refresh Explorer", Keys: "⌘R"},
		"push":    {Title: "Push", Disabled: true},
	}}
	SetCommands(stub)
	t.Cleanup(func() { SetCommands(nil) })
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Row(c).Children(func() {
			CommandIconButton(c, icons.Refresh, "refresh")
			CommandIconButton(c, icons.Push, "push")
			CommandIconButton(c, icons.Pull, "unknown")
		})
	}, testWidth, testHeight)
	info, ok := tt.Access("Refresh Explorer")
	if !ok || info.Description != "Refresh Explorer (⌘R)" {
		t.Fatalf("command button = %+v, want title and keys from the command", info)
	}
	if info, _ := tt.Access("Push"); !info.Disabled {
		t.Fatal("disabled command shows enabled")
	}
	if info, ok := tt.Access("unknown"); !ok || !info.Disabled {
		t.Fatal("unknown command does not show disabled")
	}
	if err := tt.Click("Refresh Explorer"); err != nil {
		t.Fatal(err)
	}
	if len(stub.ran) != 1 || stub.ran[0] != "refresh" {
		t.Fatalf("ran %v, want [refresh]", stub.ran)
	}
}

func TestSplitButtonParts(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) {
		SplitButton(c, "Commit", SplitOptions{Menu: func(m *ui.Menu) { Item(m, "Commit & Push", ItemOptions{}) }})
	}, testWidth, testHeight)
	if _, ok := tt.AccessByRole(ui.RoleButton, "Commit"); !ok {
		t.Fatal("split button has no main button")
	}
	if _, ok := tt.Access(defaultMenuLabel); !ok {
		t.Fatal("split button arrow has no label")
	}
}
