//go:build e2e

package e2e

import (
	"fmt"
	"strings"
	"time"

	"github.com/ZerGo0/editor/e2e/qa"
)

type actionKit struct {
	target qa.Node
}

func (k *actionKit) click(q *QAApp, d DemoProject, i int) {
	q.Send(ClickEvent(k.target.Bounds.CenterX(), k.target.Bounds.CenterY()))
}

func typeCharAction(options func(d DemoProject) QAOptions) latencyAction {
	return latencyAction{
		Owner: qa.OwnerWorkspace, IdleFrames: blinkFramesPerSec, Options: options,
		Prepare: func(q *QAApp, d DemoProject) error {
			_, err := focusCodeEditor(q)
			return err
		},
		Do: func(q *QAApp, d DemoProject, i int) { q.Send(TextEvent(typedChar)) },
	}
}

func scrollAction(options func(d DemoProject) QAOptions, startKey, endKey string) latencyAction {
	k := &actionKit{}
	a := latencyAction{
		Owner: qa.OwnerWorkspace, IdleFrames: blinkFramesPerSec, Options: options,
		Prepare: func(q *QAApp, d DemoProject) (err error) {
			k.target, err = focusCodeEditor(q)
			return err
		},
		Do: func(q *QAApp, d DemoProject, i int) {
			q.Send(map[string]any{"type": "wheel", "x": k.target.Bounds.CenterX(), "y": k.target.Bounds.CenterY(), "dy": scrollStep})
		},
	}
	if endKey != "" {
		a.Reset = func(q *QAApp, d DemoProject, i int) {
			q.Send(KeyEvent(startKey))
			q.Settle()
		}
		a.Do = func(q *QAApp, d DemoProject, i int) { q.Send(KeyEvent(endKey)) }
	}
	return a
}

func menuTitle(menu []string) string { return strings.TrimSuffix(menu[len(menu)-1], "...") }

func (k *actionKit) scrollToTop(q *QAApp, d DemoProject, i int) {
	q.Send(map[string]any{"type": "wheel", "x": k.target.Bounds.CenterX(), "y": k.target.Bounds.CenterY(), "dy": -scrollToTop})
	q.Settle()
}

func (k *actionKit) wheel(dy int) func(q *QAApp, d DemoProject, i int) {
	return func(q *QAApp, d DemoProject, i int) {
		q.Send(map[string]any{"type": "wheel", "x": k.target.Bounds.CenterX(), "y": k.target.Bounds.CenterY(), "dy": dy})
	}
}

func clearFocusedField(q *QAApp, d DemoProject, i int) {
	q.Send(KeyEvent("Cmd+A"))
	q.Send(KeyEvent("Backspace"))
	q.Settle()
}

func typeText(text string) func(q *QAApp, d DemoProject, i int) {
	return func(q *QAApp, d DemoProject, i int) { q.Send(TextEvent(text)) }
}

func expectFieldValue(label, value string) func(q *QAApp, d DemoProject) error {
	return func(q *QAApp, d DemoProject) error {
		if _, err := q.TryWait(FieldValue(label, value), q.waitFor()); err != nil {
			return fmt.Errorf("the %s field does not hold %q: %w", label, value, err)
		}
		return nil
	}
}

func quickQueryAction(menu []string, query string, hit func(n qa.Node) bool) latencyAction {
	title := menuTitle(menu)
	return latencyAction{
		Owner: qa.OwnerSearch, IdleFrames: blinkFramesPerSec,
		Prepare: func(q *QAApp, d DemoProject) error {
			if err := q.TryMenu(menu...); err != nil {
				return err
			}
			if _, err := q.TryWait(RoleLabel(paletteRole, title), q.waitFor()); err != nil {
				return fmt.Errorf("%s dialog did not open: %w", title, err)
			}
			return nil
		},
		Verify: func(q *QAApp, d DemoProject) error {
			if _, err := q.TryWait(func(n qa.Node) bool { return n.Role == "listItem" && hit(n) }, q.waitFor()); err != nil {
				return fmt.Errorf("%s shows no result row for %q: %w", title, query, err)
			}
			return nil
		},
		Reset: clearFocusedField,
		Do:    typeText(query),
	}
}

func switchTabAction(options func(d DemoProject) QAOptions) latencyAction {
	return latencyAction{
		Owner: qa.OwnerWorkspace, IdleFrames: blinkFramesPerSec, Options: options,
		Do: func(q *QAApp, d DemoProject, i int) { menuNow(q, viewMenu, nextEditorMenu) },
	}
}

func expandFolderAction(dir string) latencyAction {
	k := &actionKit{}
	return latencyAction{
		Owner: qa.OwnerWorkspace, IdleFrames: blinkFramesPerSec,
		Prepare: func(q *QAApp, d DemoProject) error {
			files, err := q.TryWait(func(n qa.Node) bool { return n.Role == "tree" }, q.waitFor())
			if err != nil {
				return err
			}
			q.Send(map[string]any{"type": "wheel", "x": files.Bounds.CenterX(), "y": files.Bounds.CenterY(), "dy": -scrollToTop})
			q.Settle()
			return nil
		},
		Reset: func(q *QAApp, d DemoProject, i int) {
			n, err := q.TryWait(RoleLabel("treeItem", dir), q.waitFor())
			if err != nil {
				q.t.Fatalf("folder %s not in the tree: %v", dir, err)
			}
			q.Act(q.Win, n.ID, "collapse", "")
			q.Settle()
			k.target = n
		},
		Do: func(q *QAApp, d DemoProject, i int) { q.Act(q.Win, k.target.ID, "expand", "") },
	}
}

func openSourceControlAction() latencyAction {
	return latencyAction{
		Owner: qa.OwnerGit,
		Reset: func(q *QAApp, d DemoProject, i int) {
			menuOrFail(q, viewMenu, "Explorer")
			q.Settle()
		},
		Do: func(q *QAApp, d DemoProject, i int) { menuNow(q, viewMenu, "Source Control") },
	}
}

func stackedDiffAction(row func(q *QAApp) (qa.Node, error)) latencyAction {
	k := &actionKit{}
	return latencyAction{
		Owner: qa.OwnerGit,
		Prepare: func(q *QAApp, d DemoProject) error {
			menuOrFail(q, viewMenu, "Source Control")
			_, err := q.TryWait(Label("Commit Message"), q.waitFor())
			return err
		},
		Reset: func(q *QAApp, d DemoProject, i int) {
			closeEditorAfterFirstRun(q, d, i)
			n, err := row(q)
			if err != nil {
				q.t.Fatal(err)
			}
			k.target = n
		},
		Do: k.click,
	}
}

func openGraphAction() latencyAction {
	return latencyAction{
		Owner: qa.OwnerGit,
		Reset: closeEditorAfterFirstRun,
		Do:    func(q *QAApp, d DemoProject, i int) { menuNow(q, gitMenu, "Show Log") },
	}
}

func toggleSidebarAction() latencyAction {
	k := &actionKit{}
	return latencyAction{
		Owner: qa.OwnerWorkspace,
		Reset: func(q *QAApp, d DemoProject, i int) {
			n, err := q.TryWait(RoleLabel("button", toggleSidebarName), navTimeout)
			if err != nil {
				q.t.Fatalf("toggle sidebar button: %v", err)
			}
			k.target = n
		},
		Do: k.click,
	}
}

func saveAction(options func(d DemoProject) QAOptions) latencyAction {
	a := typeCharAction(options)
	a.Reset = func(q *QAApp, d DemoProject, i int) {
		q.Send(TextEvent(typedChar))
		q.Settle()
	}
	a.Do = func(q *QAApp, d DemoProject, i int) { q.Send(KeyEvent(saveKey)) }
	return a
}

func (a latencyAction) with(name string, samples int, quiet time.Duration) latencyAction {
	a.Name, a.Samples, a.Quiet = name, samples, quiet
	return a
}

func (a latencyAction) isolated() latencyAction {
	a.Project = NewDemoProject
	return a
}
