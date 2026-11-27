//go:build e2e

package e2e

import (
	"fmt"
	"strings"
	"time"

	"github.com/ZerGo0/editor/e2e/qa"
)

const (
	navTimeout       = 4 * time.Second
	navPoll          = 40 * time.Millisecond
	paletteKey       = "Cmd+Shift+P"
	paletteRole      = "dialog"
	findInFilesQuery = "func"
	matchRowPrefix   = "line "
	matchRowInfix    = " line "
	commandMenuPath  = "Command Palette..."
	escapeKey        = "Escape"
	viewMenu         = "View"
	goMenu           = "Go"
	gitMenu          = "Git"
	editorMenu       = "Editor"
	scrollToTop      = 1000000
)

const graphLoadingLabel = "Loading commits"

func (q *QAApp) TryWait(match func(qa.Node) bool, timeout time.Duration) (qa.Node, error) {
	deadline := time.Now().Add(timeout)
	for {
		if n, ok := q.Snapshot().Find(match); ok {
			return n, nil
		}
		if time.Now().After(deadline) {
			return qa.Node{}, fmt.Errorf("not found within %s", timeout)
		}
		time.Sleep(navPoll)
	}
}

func (q *QAApp) TryWaitGone(match func(qa.Node) bool, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for q.Has(match) {
		if time.Now().After(deadline) {
			return fmt.Errorf("still present after %s", timeout)
		}
		time.Sleep(navPoll)
	}
	return nil
}

func (q *QAApp) TryMenu(path ...string) error {
	if _, err := q.Call("menu", map[string]any{"window": q.Win, "path": path}); err != nil {
		return err
	}
	q.Settle()
	return nil
}

func (q *QAApp) Palette(query string) error {
	q.Send(KeyEvent(paletteKey))
	q.Settle()
	if _, err := q.TryWait(func(n qa.Node) bool { return n.Role == paletteRole }, navTimeout); err != nil {
		return fmt.Errorf("command palette did not open: %w", err)
	}
	q.Send(TextEvent(query))
	q.Settle()
	q.Send(KeyEvent("Enter"))
	q.Settle()
	return nil
}

func (q *QAApp) Command(paletteQuery string, menuPath ...string) error {
	if len(menuPath) > 0 {
		if err := q.TryMenu(menuPath...); err == nil {
			return nil
		}
	}
	return q.Palette(paletteQuery)
}

func (q *QAApp) Escape() {
	q.Send(KeyEvent(escapeKey))
	q.Settle()
}

func hasLabel(label string) func(qa.Node) bool {
	return func(n qa.Node) bool { return n.Label == label }
}

func hasRoleLabel(role, label string) func(qa.Node) bool {
	return func(n qa.Node) bool { return n.Role == role && n.Label == label }
}

func labelPrefix(prefix string) func(qa.Node) bool {
	return func(n qa.Node) bool { return strings.HasPrefix(n.Label, prefix) }
}

func countRoles(t *qa.Tree, roles ...string) int {
	count := 0
	for _, n := range t.Nodes {
		for _, r := range roles {
			if n.Role == r {
				count++
			}
		}
	}
	return count
}

func (q *QAApp) FocusNode(n qa.Node) {
	q.t.Helper()
	q.Act(q.Win, n.ID, "focus", "")
	q.Settle()
}

func (q *QAApp) searchDialog(name string, menu ...string) error {
	q.Escape()
	if err := q.TryMenu(menu...); err != nil {
		return err
	}
	if _, err := q.TryWait(hasRoleLabel(paletteRole, name), navTimeout); err != nil {
		return err
	}
	q.Send(TextEvent(findInFilesQuery))
	q.Settle()
	_, err := q.TryWait(func(n qa.Node) bool {
		return strings.HasPrefix(n.Label, matchRowPrefix) || strings.Contains(n.Label, matchRowInfix)
	}, navTimeout)
	return err
}

var modalSteps = map[string]bool{"quickinput": true, "search-all": true, "search-text": true, "search-text-replace": true}

type viewStep struct {
	Name  string
	Enter func(q *QAApp, d DemoProject) error
}

func demoViewSteps() []viewStep {
	return []viewStep{
		{"explorer", func(q *QAApp, d DemoProject) error {
			if err := q.Command("Show Explorer", viewMenu, "Explorer"); err != nil {
				return err
			}
			_, err := q.TryWait(func(n qa.Node) bool { return n.Role == "treeItem" }, navTimeout)
			return err
		}},
		{"explorer-big-folder", func(q *QAApp, d DemoProject) error {
			files, err := q.TryWait(func(n qa.Node) bool { return n.Role == "tree" }, navTimeout)
			if err != nil {
				return err
			}
			q.Send(map[string]any{"type": "wheel", "x": files.Bounds.CenterX(), "y": files.Bounds.CenterY(), "dy": -scrollToTop})
			q.Settle()
			folder, err := q.TryWait(hasRoleLabel("treeItem", d.ChangelogDir), navTimeout)
			if err != nil {
				return err
			}
			q.Act(q.Win, folder.ID, "expand", "")
			q.Settle()
			_, err = q.TryWait(labelPrefix("entry-"), navTimeout)
			return err
		}},
		{"search-all", func(q *QAApp, d DemoProject) error {
			return q.searchDialog("Search Everywhere", goMenu, "Search Everywhere")
		}},
		{"search-text", func(q *QAApp, d DemoProject) error {
			return q.searchDialog("Find in Files", goMenu, "Find in Files")
		}},
		{"search-text-replace", func(q *QAApp, d DemoProject) error {
			return q.searchDialog("Replace in Files", goMenu, "Replace in Files")
		}},
		{"scm", func(q *QAApp, d DemoProject) error {
			q.Escape()
			if err := q.Command("Show Source Control", viewMenu, "Source Control"); err != nil {
				return err
			}
			_, err := q.TryWait(hasLabel("Commit Message"), navTimeout)
			return err
		}},
		{"changes-diff", func(q *QAApp, d DemoProject) error {
			q.Escape()
			if err := q.Command("Show Source Control", viewMenu, "Source Control"); err != nil {
				return err
			}
			row, err := q.waitChangedRow(isDemoChangedFile)
			if err != nil {
				return err
			}
			q.ClickNode(row)
			if _, err = q.TryWait(hasRoleLabel("toggleButton", "Toggle File Tree"), navTimeout); err != nil {
				return err
			}
			q.Send(map[string]any{"type": pointerMoveEvent, "x": 0, "y": 0})
			q.Settle()
			return nil
		}},
		{"graph", func(q *QAApp, d DemoProject) error {
			if err := q.Command("Git: Show Log", gitMenu, "Show Log"); err != nil {
				return err
			}
			_, err := q.TryWait(func(n qa.Node) bool {
				return strings.Contains(n.Label, "feat(") || strings.Contains(n.Label, "Merge branch")
			}, navTimeout)
			if err != nil {
				return err
			}
			return q.TryWaitGone(hasLabel(graphLoadingLabel), navTimeout)
		}},
		{"github", func(q *QAApp, d DemoProject) error {
			return q.Command("Show GitHub", viewMenu, "GitHub")
		}},
		{"settings", func(q *QAApp, d DemoProject) error {
			if err := q.Command("Open Settings (UI)", editorMenu, "Settings"); err != nil {
				return err
			}
			_, err := q.TryWait(hasRoleLabel("textField", "Search settings"), navTimeout)
			return err
		}},
		{"keybindings", func(q *QAApp, d DemoProject) error {
			if err := q.Command("Open Keyboard Shortcuts", editorMenu, "Keyboard Shortcuts"); err != nil {
				return err
			}
			_, err := q.TryWait(hasRoleLabel("tab", "Keyboard Shortcuts"), navTimeout)
			return err
		}},
		{"quickinput", func(q *QAApp, d DemoProject) error {
			if err := q.Command("Go to File", goMenu, "Go to File..."); err != nil {
				return err
			}
			_, err := q.TryWait(func(n qa.Node) bool { return n.Role == paletteRole }, navTimeout)
			return err
		}},
		{"panel", func(q *QAApp, d DemoProject) error {
			q.Escape()
			return q.Command("Show Problems", viewMenu, "Problems")
		}},
	}
}
