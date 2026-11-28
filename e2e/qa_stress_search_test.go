//go:build e2e

package e2e

import (
	"fmt"
	"strings"

	"github.com/ZerGo0/editor/e2e/qa"
)

const (
	searchEverywhereMenu   = "Search Everywhere"
	commandPaletteQuery    = "show"
	commandPaletteLabel    = "Command Palette"
	resultsLabel           = "Results"
	stressResultsQuery     = "file1"
	stressResultsScroll    = 2000
	stressPaletteScroll    = 600
	searchEverywhereFilter = "Files"
)

var (
	paletteMenuPath          = []string{viewMenu, "Command Palette..."}
	searchEverywhereMenuPath = []string{goMenu, searchEverywhereMenu}
)

func chooseTab(q *QAApp, tab string) error {
	n, err := q.TryWait(RoleLabel("tab", tab), q.waitFor())
	if err != nil {
		return fmt.Errorf("tab %s: %w", tab, err)
	}
	q.ClickNode(n)
	dialog, err := q.TryWait(func(n qa.Node) bool { return n.Role == paletteRole }, q.waitFor())
	if err != nil {
		return fmt.Errorf("dialog after the %s tab: %w", tab, err)
	}
	field, err := q.TryWait(RoleLabel("textField", dialog.Label), q.waitFor())
	if err != nil {
		return fmt.Errorf("query field of %s: %w", dialog.Label, err)
	}
	q.FocusNode(field)
	return nil
}

func (a latencyAction) inTab(tab string) latencyAction {
	prepare := a.Prepare
	a.Prepare = func(q *QAApp, d DemoProject) error {
		if err := prepare(q, d); err != nil {
			return err
		}
		return chooseTab(q, tab)
	}
	return a
}

func resultsList(q *QAApp) (qa.Node, error) {
	return q.TryWait(RoleLabel("list", resultsLabel), q.waitFor())
}

func openDialogAction(open func(q *QAApp, i int), title string) latencyAction {
	return latencyAction{
		Owner: qa.OwnerSearch, IdleFrames: blinkFramesPerSec,
		Reset: func(q *QAApp, d DemoProject, i int) { q.Escape() },
		Do:    func(q *QAApp, d DemoProject, i int) { open(q, i) },
		Verify: func(q *QAApp, d DemoProject) error {
			_, err := q.TryWait(RoleLabel(paletteRole, title), q.waitFor())
			return err
		},
	}
}

func openPaletteAction() latencyAction {
	return openDialogAction(func(q *QAApp, i int) { q.Send(KeyEvent(paletteKey)) }, commandPaletteLabel)
}

func openSearchEverywhereAction() latencyAction {
	return openDialogAction(func(q *QAApp, i int) { menuNow(q, goMenu, searchEverywhereMenu) }, searchEverywhereTitle)
}

func scrollResultsAction(openMenu []string, tab, query string, dy int) latencyAction {
	k := &actionKit{}
	return latencyAction{
		Owner: qa.OwnerSearch, IdleFrames: blinkFramesPerSec,
		Prepare: func(q *QAApp, d DemoProject) (err error) {
			if err = q.TryMenu(openMenu...); err != nil {
				return err
			}
			if _, err = q.TryWait(RoleLabel(paletteRole, menuTitle(openMenu)), q.waitFor()); err != nil {
				return err
			}
			if tab != "" {
				if err = chooseTab(q, tab); err != nil {
					return err
				}
			}
			if query != "" {
				q.Send(TextEvent(query))
				q.Settle()
			}
			if _, err = q.TryWait(func(n qa.Node) bool { return n.Role == "listItem" && n.Label != "" }, q.waitFor()); err != nil {
				return fmt.Errorf("no result for %q: %w", query, err)
			}
			k.target, err = resultsList(q)
			q.WaitCPUQuiet()
			return err
		},
		Reset: k.scrollToTop,
		Do:    k.wheel(dy),
	}
}

func stressSearchActions() []latencyAction {
	tree := func(a latencyAction, name string, samples int) latencyAction {
		return a.onFixture(stressFixtureTree, name, stressQuietLong, samples)
	}
	treeShort := func(a latencyAction, name string, samples int) latencyAction {
		return a.onFixture(stressFixtureTree, name, stressQuietShort, samples)
	}
	small := func(a latencyAction, name string, samples int) latencyAction {
		return a.onProject(stressSmallProject, name, stressQuietShort, samples)
	}
	return []latencyAction{
		small(openPaletteAction(), "open the Command Palette", latencySamples),
		small(quickQueryAction(paletteMenuPath, commandPaletteQuery, isCommandRow), "type a query in the Command Palette", latencySamples),
		small(scrollResultsAction(paletteMenuPath, "", "", stressPaletteScroll), "scroll the Command Palette results", stressSamples),
		treeShort(openSearchEverywhereAction(), "open Search Everywhere over the big folder", stressSamples),
		tree(quickQueryAction(searchEverywhereMenuPath, stressQueryFile, labelPrefix(stressQueryFile)), "type a query in Search Everywhere (All) over the big folder", stressSamples),
		tree(quickQueryAction(searchEverywhereMenuPath, stressQueryFile, labelPrefix(stressQueryFile)).inTab(searchEverywhereFilter), "type a query in Search Everywhere (Files) over the big folder", stressSamples),
		treeShort(scrollResultsAction(searchEverywhereMenuPath, searchEverywhereFilter, stressResultsQuery, stressResultsScroll), "scroll the Search Everywhere (Files) results over the big folder", stressSamples),
	}
}

func isCommandRow(n qa.Node) bool {
	return strings.Contains(strings.ToLower(n.Label), commandPaletteQuery)
}
