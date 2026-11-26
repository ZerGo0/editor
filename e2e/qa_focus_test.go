//go:build e2e

package e2e

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/ZerGo0/editor/e2e/qa"
)

const (
	maxFocusSteps   = 60
	maxPartSteps    = 8
	minPartsVisited = 3
	rulePartCycle   = "focus-part-cycle"
	rulePartSame    = "focus-part-unchanged"
	ruleEscape      = "escape-not-closing"
	tabKey          = "Tab"
	shiftTabKey     = "Shift+Tab"
	partKey         = "F6"
	findKey         = "Cmd+F"
	goToLineKey     = "Ctrl+G"
	sidebarRegion   = "Primary Side Bar"
	panelRegion     = "Panel"
	moreActionsName = "More Actions..."
	partTitleBar    = "titlebar"
	partStatusBar   = "statusbar"
	partSidebar     = "sidebar"
	partPanel       = "panel"
	partEditor      = "editor"
)

var focusAllow = []qa.Allow{}

func isCodeEditor(n qa.Node) bool {
	return n.Role == "textField" && n.Has("multiline") && n.Label == "Code editor"
}

func (q *QAApp) Walk(key string, limit int) []qa.FocusStop {
	var stops []qa.FocusStop
	var first string
	for i := range limit {
		q.Press(key)
		stop := qa.NewStop(i, key, q.Snapshot())
		stops = append(stops, stop)
		if !stop.Found {
			continue
		}
		if isCodeEditor(stop.Focus) || (first != "" && stop.Focus.ID == first) {
			break
		}
		if first == "" {
			first = stop.Focus.ID
		}
	}
	return stops
}

func partOf(t *qa.Tree, n qa.Node) string {
	inRegion := func(label string) bool {
		region, ok := t.Find(func(r qa.Node) bool { return r.Role == "region" && r.Label == label })
		return ok && region.Bounds.Contains(n.Bounds, 1)
	}
	switch {
	case qa.ChromeZone(n.Bounds, t.Window) && n.Bounds.Y < t.Window.Height/2:
		return partTitleBar
	case t.Window.Height > 0 && n.Bounds.Y >= t.Window.Height-24:
		return partStatusBar
	case inRegion(panelRegion):
		return partPanel
	case inRegion(sidebarRegion):
		return partSidebar
	}
	return partEditor
}

func (q *QAApp) PartWalk() ([]qa.FocusStop, []string) {
	var stops []qa.FocusStop
	var parts []string
	for i := range maxPartSteps {
		q.Press(partKey)
		tree := q.Snapshot()
		stop := qa.NewStop(i, partKey, tree)
		stops = append(stops, stop)
		part := "none"
		if stop.Found {
			part = partOf(tree, stop.Focus)
		}
		parts = append(parts, part)
	}
	return stops, parts
}

func partFlags(view string, stops []qa.FocusStop, parts []string) []qa.Flag {
	var flags []qa.Flag
	seen := map[string]bool{}
	for i, part := range parts {
		seen[part] = true
		if i > 0 && part == parts[i-1] {
			flags = append(flags, qa.Flag{Rule: rulePartSame, View: view, Owner: qa.OwnerWorkspace, Role: "key", Label: partKey,
				Measured: fmt.Sprintf("press %d of %s left focus in the %s part (%s)", i+1, partKey, part, stops[i].Describe()), Expected: "each press moves focus to the next part (sidebar, editor, panel, status bar)", Fix: "bind workbench.action.focusNextPart to F6 and make takeFocus move the keyboard focus (workspace/focus.go)"})
		}
	}
	if visited := len(seen); visited < minPartsVisited {
		flags = append(flags, qa.Flag{Rule: rulePartCycle, View: view, Owner: qa.OwnerWorkspace, Role: "key", Label: partKey,
			Measured: fmt.Sprintf("%d presses reached %d parts: %v", len(parts), visited, parts), Expected: fmt.Sprintf("at least %d parts (sidebar, editor, panel or status bar)", minPartsVisited), Fix: "bind workbench.action.focusNextPart to F6 (internal/keymap defaults) and cycle the visible parts"})
	}
	return flags
}

func gainedField(before, after *qa.Tree) bool {
	known := map[string]bool{}
	for _, n := range before.Nodes {
		if n.Role == "textField" {
			known[n.Label] = true
		}
	}
	for _, n := range after.Nodes {
		if n.Role == "textField" && !known[n.Label] {
			return true
		}
	}
	return false
}

func escapeFlag(view, what string, tree *qa.Tree, still func(qa.Node) bool) []qa.Flag {
	if _, ok := tree.Find(still); !ok {
		return nil
	}
	return []qa.Flag{{Rule: ruleEscape, View: view, Owner: qa.Owner(view, qa.Node{}, qa.Rect{}), Role: "key", Label: escapeKey,
		Measured: what + " is still open after Escape", Expected: what + " closes on Escape", Fix: "handle Escape in the popup (c.Shortcut(0, ui.KeyEscape)) and restore focus"}}
}

func TestKeyboardFocusWalk(t *testing.T) {
	t.Parallel()
	var all []qa.Flag
	var mu sync.Mutex
	t.Cleanup(func() {
		for _, a := range qa.Stale(all, focusAllow) {
			t.Errorf("stale allowlist entry (remove it): %+v", a)
		}
	})
	collect := func(t *testing.T, q *QAApp, name string, flags []qa.Flag) {
		t.Helper()
		kept, _ := qa.Filter(qa.Dedupe(flags), focusAllow)
		mu.Lock()
		all = append(all, flags...)
		mu.Unlock()
		report := qa.Format(kept)
		q.Report("focus-"+name, report)
		if len(kept) > 0 {
			t.Errorf("%d flags:\n%s", len(kept), report)
		}
	}
	t.Run("window-start", func(t *testing.T) {
		t.Parallel()
		demo := NewDemoProject(t)
		q := LaunchQA(t, demo.Root, QAOptions{Width: normalWindowW, Height: normalWindowH, SidebarWidth: defaultSidebar, Tabs: demo.TabFiles, Theme: themeDark})
		var flags []qa.Flag
		forward := q.Walk(tabKey, maxFocusSteps)
		flags = append(flags, qa.CheckFocusWalk("explorer", forward)...)
		if len(forward) > 0 && forward[len(forward)-1].Found && !isCodeEditor(forward[len(forward)-1].Focus) {
			t.Logf("Tab walk ended without reaching the editor or a cycle after %d steps", len(forward))
		}
		q2 := LaunchQA(t, demo.Root, QAOptions{Width: normalWindowW, Height: normalWindowH, SidebarWidth: defaultSidebar, Tabs: demo.TabFiles, Theme: themeDark})
		backward := q2.Walk(shiftTabKey, maxFocusSteps)
		flags = append(flags, qa.CheckFocusWalk("explorer", backward)...)
		q3 := LaunchQA(t, demo.Root, QAOptions{Width: normalWindowW, Height: normalWindowH, SidebarWidth: defaultSidebar, Tabs: demo.TabFiles, Theme: themeDark, PanelOpen: true, Panel: "problems"})
		stops, parts := q3.PartWalk()
		flags = append(flags, partFlags("explorer", stops, parts)...)
		collect(t, q, "window-start", flags)
	})
	t.Run("views", func(t *testing.T) {
		t.Parallel()
		demo := NewDemoProject(t)
		q := LaunchQA(t, demo.Root, QAOptions{Width: normalWindowW, Height: normalWindowH, SidebarWidth: defaultSidebar, Tabs: demo.TabFiles, Theme: themeDark})
		var flags []qa.Flag
		for _, step := range demoViewSteps() {
			if step.Name == "quickinput" || step.Name == "explorer-big-folder" {
				continue
			}
			if err := step.Enter(q, demo); err != nil {
				flags = append(flags, unreachableFlag(step.Name, "focus", err))
				continue
			}
			q.Settle()
			stops := q.Walk(tabKey, maxFocusSteps)
			flags = append(flags, qa.CheckFocusWalk(step.Name, stops)...)
			flags = append(flags, qa.CheckInteractiveLabels(step.Name, q.Snapshot())...)
		}
		collect(t, q, "views", flags)
	})
	t.Run("escape", func(t *testing.T) {
		t.Parallel()
		demo := NewDemoProject(t)
		q := LaunchQA(t, demo.Root, QAOptions{Width: normalWindowW, Height: normalWindowH, SidebarWidth: defaultSidebar, Tabs: demo.TabFiles, Theme: themeDark})
		var flags []qa.Flag
		focusEditor := func() qa.FocusStop {
			editor, err := q.TryWait(isCodeEditor, navTimeout)
			if err != nil {
				t.Fatalf("code editor: %v", err)
			}
			q.FocusNode(editor)
			return qa.NewStop(0, "focus", q.Snapshot())
		}
		isDialog := func(n qa.Node) bool {
			return n.Role == paletteRole || n.Role == "alertDialog" || n.Role == "popup" || n.Role == "menu"
		}
		cases := []struct {
			view, what string
			open       func() error
		}{
			{"quickinput", "Go to File", func() error { return q.TryMenu(goMenu, "Go to File...") }},
			{"quickinput", "Command Palette", func() error { return q.TryMenu(viewMenu, commandMenuPath) }},
			{"quickinput", "Go to Line", func() error { q.Press(goToLineKey); return nil }},
			{"editor", "Find widget", func() error { q.Press(findKey); return nil }},
			{"tabs", "New File prompt", func() error { return q.TryMenu("File", "New File...") }},
			{"tabs", "More Actions menu", func() error {
				button, err := q.TryWait(hasRoleLabel("menuButton", moreActionsName), navTimeout)
				if err != nil {
					return err
				}
				q.Act(q.Win, button.ID, "press", "")
				q.Settle()
				return nil
			}},
		}
		for _, c := range cases {
			before := focusEditor()
			q0 := q.Snapshot()
			if err := c.open(); err != nil {
				flags = append(flags, unreachableFlag(c.view, "escape", fmt.Errorf("%s did not open: %w", c.what, err)))
				q.Escape()
				continue
			}
			opened := q.Snapshot()
			if c.what == "Find widget" {
				if _, ok := opened.Find(hasRoleLabel("textField", "Find")); !ok {
					flags = append(flags, unreachableFlag(c.view, "escape", fmt.Errorf("Find widget did not open")))
					continue
				}
			} else if _, ok := opened.Find(isDialog); !ok && c.what != "More Actions menu" && !gainedField(q0, opened) {
				flags = append(flags, unreachableFlag(c.view, "escape", fmt.Errorf("%s did not open a dialog", c.what)))
				continue
			}
			q.Escape()
			after := qa.NewStop(1, escapeKey, q.Snapshot())
			flags = append(flags, qa.CheckRestored(c.view, c.what+" Escape", before, after)...)
			closed := q.Snapshot()
			flags = append(flags, escapeFlag(c.view, c.what, closed, func(n qa.Node) bool {
				return isDialog(n) || (c.what == "Find widget" && n.Role == "textField" && strings.EqualFold(n.Label, "Find"))
			})...)
			if c.what == "More Actions menu" {
				q.Escape()
			}
		}
		collect(t, q, "escape", flags)
	})
}
