//go:build e2e

package e2e

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/ZerGo0/editor/e2e/qa"
	"github.com/ZerGo0/editor/internal/session"
)

const (
	stressGroupCount   = 8
	stressFocusSamples = 2*stressGroupCount - 1
	stressGroupTabs    = 50
	splitRightMenu     = "Split Right"
	splitDownMenu      = "Split Down"
	editorGroupsSplits = stressGroupCount - 1
)

func groupsLayout(groups [][]string) *session.Node {
	return groupsNode(groups, false)
}

func groupsNode(groups [][]string, vertical bool) *session.Node {
	if len(groups) == 1 {
		tabs := make([]session.Tab, len(groups[0]))
		for i, p := range groups[0] {
			tabs[i] = session.Tab{Path: p}
		}
		return &session.Node{Group: &session.Group{Tabs: tabs}}
	}
	half := len(groups) / 2
	return &session.Node{Vertical: vertical, Percent: 50, First: groupsNode(groups[:half], !vertical), Second: groupsNode(groups[half:], !vertical)}
}

func stressSplitGroups(d DemoProject) [][]string {
	groups := make([][]string, stressGroupCount)
	for g := range groups {
		groups[g] = make([]string, stressGroupTabs)
		for t := range groups[g] {
			groups[g][t] = stressTreeTabFile(d.Root, g*stressGroupTabs+t)
		}
	}
	return groups
}

func stressSplitOptions(d DemoProject) QAOptions {
	o := stressOptions(nil)(d)
	o.Groups = stressSplitGroups(d)
	return o
}

func stressSingleGroupOptions(d DemoProject) QAOptions {
	return stressOptions(stressTreeTabs(stressGroupTabs))(d)
}

func editorGroupCount(q *QAApp) int {
	return countLabels(q.Snapshot(), "tabList", editorTabsLabel)
}

func countLabels(t *qa.Tree, role, label string) int {
	count := 0
	for _, n := range t.Nodes {
		if n.Role == role && n.Label == label {
			count++
		}
	}
	return count
}

func expectEditorGroups(want int) func(q *QAApp, d DemoProject) error {
	return func(q *QAApp, d DemoProject) error {
		if got := editorGroupCount(q); got != want {
			return fmt.Errorf("%d editor groups, want %d", got, want)
		}
		return nil
	}
}

func openGroupsAction() latencyAction {
	return latencyAction{Owner: qa.OwnerWorkspace, IdleFrames: blinkFramesPerSec, Verify: expectEditorGroups(stressGroupCount)}
}

func splitEditorAction() latencyAction {
	return latencyAction{
		Owner: qa.OwnerWorkspace, IdleFrames: blinkFramesPerSec,
		Verify: expectEditorGroups(stressGroupCount),
		Do: func(q *QAApp, d DemoProject, i int) {
			menuNow(q, viewMenu, []string{splitRightMenu, splitDownMenu}[i%2])
		},
	}
}

func expectActiveFile(group int) func(q *QAApp, d DemoProject) error {
	return func(q *QAApp, d DemoProject) error {
		name := filepath.Base(stressTreeTabFile(d.Root, group*stressGroupTabs))
		var title string
		q.Send(map[string]any{"type": pointerMoveEvent, "x": 0, "y": 0})
		for deadline := time.Now().Add(q.waitFor()); time.Now().Before(deadline); time.Sleep(navPoll) {
			for _, w := range q.Windows() {
				if w.ID == q.Win {
					title = w.Title
				}
			}
			if strings.HasPrefix(title, name) {
				return nil
			}
		}
		return fmt.Errorf("group %d is not the active group: the window title is %q, want it to start with %s", group+1, title, name)
	}
}

func focusGroupAction() latencyAction {
	k := &actionKit{}
	var editors []qa.Node
	return latencyAction{
		Owner: qa.OwnerWorkspace, IdleFrames: blinkFramesPerSec,
		Verify: expectActiveFile(stressFocusSamples % stressGroupCount),
		Prepare: func(q *QAApp, d DemoProject) error {
			for _, n := range q.Snapshot().Nodes {
				if isCodeEditor(n) {
					editors = append(editors, n)
				}
			}
			if len(editors) != stressGroupCount {
				return fmt.Errorf("%d editors, want %d", len(editors), stressGroupCount)
			}
			return nil
		},
		Do: func(q *QAApp, d DemoProject, i int) {
			k.target = editors[i%stressGroupCount]
			k.click(q, d, i)
		},
	}
}

func closeGroupsAction() latencyAction {
	a := closeAllTabsAction()
	a.Verify = expectEditorGroups(0)
	return a
}

func stressSplitActions() []latencyAction {
	tabs := fmt.Sprintf("%d editor groups with %d tabs each", stressGroupCount, stressGroupTabs)
	return []latencyAction{
		openGroupsAction().onFixture(stressFixtureTree, "open "+tabs, stressQuietLong, 0).withOptions(stressSplitOptions),
		splitEditorAction().onFixture(stressFixtureTree, fmt.Sprintf("split the editor into %d groups", stressGroupCount), stressQuietLong, editorGroupsSplits).withOptions(stressSingleGroupOptions).cold(),
		focusGroupAction().onFixture(stressFixtureTree, fmt.Sprintf("switch focus between %d editor groups", stressGroupCount), stressQuietLong, stressFocusSamples).withOptions(stressSplitOptions),
		closeGroupsAction().onFixture(stressFixtureTree, "close "+tabs, stressQuietLong, 1).withOptions(stressSplitOptions).cold(),
	}
}
