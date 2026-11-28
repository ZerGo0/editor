//go:build e2e

package e2e

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/ZerGo0/editor/e2e/qa"
)

const (
	todoMenuItem        = "TODO"
	problemsMenuItem    = "Problems"
	panelToggleLabel    = "Toggle Panel"
	problemsFilterName  = "Filter Problems"
	problemsFilterText  = "file01"
	insightsScroll      = 2000
	problemMessage      = "is not closed"
	stressTabCycleStep  = 40 * time.Millisecond
	stressTodoLastGroup = "todo/dir19/file1999.go"
)

func stressProblemTabs(d DemoProject) []string {
	tabs := make([]string, stressProblemFiles)
	for i := range tabs {
		tabs[i] = filepath.Join(d.Root, filepath.FromSlash(stressProblemPath(i)))
	}
	return tabs
}

func stressProblemOptions(d DemoProject) QAOptions {
	return stressOptions(stressProblemTabs)(d)
}

func loadAllProblems(q *QAApp, d DemoProject) error {
	for range stressProblemFiles {
		menuNow(q, viewMenu, nextEditorMenu)
		time.Sleep(stressTabCycleStep)
	}
	q.Settle()
	want := fmt.Sprintf("Problems: %d errors", stressProblemCount)
	if _, err := q.TryWait(labelPrefix(want), q.waitFor()); err != nil {
		return fmt.Errorf("the status bar does not show %q: %w", want, err)
	}
	q.WaitCPUQuiet()
	return nil
}

func closePanelAfterFirstRun(q *QAApp, _ DemoProject, i int) {
	if i == 0 {
		return
	}
	n, err := q.TryWait(RoleLabel("button", panelToggleLabel), q.waitFor())
	if err != nil {
		q.t.Fatalf("panel toggle: %v", err)
	}
	q.ClickNode(n)
}

func showPanelNow(view string) func(q *QAApp, d DemoProject, i int) {
	return func(q *QAApp, d DemoProject, i int) { menuNow(q, viewMenu, view) }
}

func openPanelAction(view string, prepare func(q *QAApp, d DemoProject) error, verify func(q *QAApp, d DemoProject) error) latencyAction {
	return latencyAction{
		Owner: qa.OwnerWorkspace, IdleFrames: blinkFramesPerSec,
		Prepare: prepare,
		Reset:   closePanelAfterFirstRun,
		Do:      showPanelNow(view),
		Verify:  verify,
	}
}

func expectProblemEntry(q *QAApp, d DemoProject) error {
	_, err := q.TryWait(func(n qa.Node) bool { return n.Role == "treeItem" && strings.Contains(n.Label, problemMessage) }, q.waitFor())
	return err
}

func expectLastGroup(view, group string) func(q *QAApp, d DemoProject) error {
	return func(q *QAApp, d DemoProject) error {
		list, err := q.TryWait(RoleLabel("tree", view), q.waitFor())
		if err != nil {
			return err
		}
		q.Send(map[string]any{"type": "wheel", "x": list.Bounds.CenterX(), "y": list.Bounds.CenterY(), "dy": scrollToTop})
		if _, err := q.TryWait(RoleLabel("treeItem", group), q.waitFor()); err != nil {
			return fmt.Errorf("the last group %s is not in the %s panel: %w", group, view, err)
		}
		return nil
	}
}

func scrollPanelAction(view string, list func(n qa.Node) bool) latencyAction {
	k := &actionKit{}
	return latencyAction{
		Owner: qa.OwnerWorkspace, IdleFrames: blinkFramesPerSec,
		Verify: expectLastGroup(view, stressTodoLastGroup),
		Prepare: func(q *QAApp, d DemoProject) (err error) {
			if err = q.TryMenu(viewMenu, view); err != nil {
				return err
			}
			k.target, err = q.TryWait(list, q.waitFor())
			return err
		},
		Reset: k.scrollToTop,
		Do:    k.wheel(insightsScroll),
	}
}

func filterProblemsAction() latencyAction {
	return latencyAction{
		Owner: qa.OwnerWorkspace, IdleFrames: blinkFramesPerSec,
		Prepare: func(q *QAApp, d DemoProject) error {
			if err := loadAllProblems(q, d); err != nil {
				return err
			}
			if err := q.TryMenu(viewMenu, problemsMenuItem); err != nil {
				return err
			}
			field, err := q.TryWait(RoleLabel("textField", problemsFilterName), q.waitFor())
			if err != nil {
				return err
			}
			q.FocusNode(field)
			return nil
		},
		Reset:  clearFocusedField,
		Do:     typeText(problemsFilterText),
		Verify: expectFieldValue(problemsFilterName, problemsFilterText),
	}
}

func stressInsightsActions() []latencyAction {
	todos := fmt.Sprintf("%dk TODO comments", stressTodoCount/1000)
	problems := fmt.Sprintf("%dk problems", stressProblemCount/1000)
	insights := func(a latencyAction, name string) latencyAction {
		return a.onFixture(stressFixtureInsights, name, stressQuietLong, stressSamples)
	}
	return []latencyAction{
		insights(openPanelAction(todoMenuItem, nil, expectLastGroup(todoMenuItem, stressTodoLastGroup)), "open the TODO panel ("+todos+")"),
		insights(scrollPanelAction(todoMenuItem, RoleLabel("tree", todoMenuItem)), "scroll the TODO panel ("+todos+")"),
		insights(openPanelAction(problemsMenuItem, loadAllProblems, expectProblemEntry), "open the Problems panel ("+problems+")").withOptions(stressProblemOptions),
		insights(filterProblemsAction(), "filter the Problems panel ("+problems+")").withOptions(stressProblemOptions),
	}
}
