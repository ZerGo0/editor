package action

import (
	"testing"

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
