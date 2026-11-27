//go:build e2e

package e2e

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ZerGo0/editor/e2e/qa"
)

const (
	stressQuietTimeout    = 30 * time.Second
	stressWaitTimeout     = 10 * time.Second
	stressMaxWait         = 2 * time.Second
	stressScenarioTimeout = 3 * time.Minute
	stressLeakTimeout     = 30 * time.Minute
	stressTabCount        = 200
	stressLeakTabs        = 50
	stressLeakDiffs       = 5
	stressTabStride       = 397
	stressBigSamples      = 2
	stressSamples         = 5
	stressQuietLong       = 300 * time.Millisecond
	stressQuietShort      = 60 * time.Millisecond
	stressQueryFile       = "file77777"
	stressGraphScroll     = 5000
	closeAllEditorsLabel  = "Close All Editors"
	findInFilesMenu       = "Find in Files"
	goToFileMenu          = "Go to File..."
	cmdTop                = "Cmd+Up"
	cmdBottom             = "Cmd+Down"
)

type bigFile struct {
	label string
	name  string
}

var stressBigFiles = []bigFile{
	{"1 GB file", stressHugeName},
	{"50 MB single-line file", stressMinifiedName},
	{"200k-line file", stressLongName},
}

func stressTreeTabFile(root string, i int) string {
	return filepath.Join(root, filepath.FromSlash(stressTreePath(i*stressTabStride%(stressTreeLeafDirs*stressTreeLeafFiles))))
}

func stressTreeTabs(count int) func(d DemoProject) []string {
	return func(d DemoProject) []string {
		tabs := make([]string, count)
		for i := range tabs {
			tabs[i] = stressTreeTabFile(d.Root, i)
		}
		return tabs
	}
}

func stressOptions(tabs func(d DemoProject) []string) func(d DemoProject) QAOptions {
	return func(d DemoProject) QAOptions {
		o := QAOptions{Width: normalWindowW, Height: normalWindowH, SidebarWidth: defaultSidebar, Theme: themeDark, FrameStats: true, QuietTimeout: stressQuietTimeout, WaitTimeout: stressWaitTimeout}
		if tabs != nil {
			o.Tabs = tabs(d)
		}
		return o
	}
}

func stressOnFile(name string) func(d DemoProject) []string {
	return func(d DemoProject) []string { return []string{filepath.Join(d.Root, name)} }
}

func stressProject(fixture string) func(t *testing.T) DemoProject {
	return func(t *testing.T) DemoProject { return DemoProject{Root: stressRoot(fixture)} }
}

func (a latencyAction) onFixture(fixture, name string, quiet time.Duration, samples int) latencyAction {
	a.Fixture, a.Project = fixture, stressProject(fixture)
	a.Name, a.Samples, a.Quiet = name, samples, quiet
	a.MaxWait, a.Timeout = stressMaxWait, stressScenarioTimeout
	if a.Options == nil {
		a.Options = stressOptions(nil)
	}
	return a
}

func (a latencyAction) withOptions(options func(d DemoProject) QAOptions) latencyAction {
	a.Options = options
	return a
}

func (a latencyAction) cold() latencyAction {
	a.Cold = true
	return a
}

func (a latencyAction) leakCheck() latencyAction {
	a.Timeout = stressLeakTimeout
	return a
}

func openFromExplorer(q *QAApp, name string) {
	q.t.Helper()
	row, err := q.TryWait(hasRoleLabel("treeItem", name), q.waitFor())
	if err != nil {
		q.t.Fatalf("%s not in the explorer: %v", name, err)
	}
	q.Send(ClickEvent(row.Bounds.CenterX(), row.Bounds.CenterY()))
	if _, err := q.TryWait(hasRoleLabel("tab", name), q.waitFor()); err != nil {
		q.t.Fatalf("tab %s did not open: %v", name, err)
	}
}

func firstChangeRow(q *QAApp) (qa.Node, error) {
	return q.waitChangedRow(func(label string) bool { return strings.HasSuffix(label, ".go") })
}

func openFolderAction() latencyAction {
	return latencyAction{Owner: qa.OwnerWorkspace}
}

func closeAllTabsAction() latencyAction {
	k := &actionKit{}
	return latencyAction{
		Owner: qa.OwnerWorkspace,
		Prepare: func(q *QAApp, d DemoProject) (err error) {
			k.target, err = q.TryWait(hasLabel(closeAllEditorsLabel), q.waitFor())
			return err
		},
		Do: k.click,
	}
}

func closeTabsOneByOneAction() latencyAction {
	return latencyAction{
		Owner: qa.OwnerWorkspace, IdleFrames: blinkFramesPerSec,
		Do: func(q *QAApp, d DemoProject, i int) { menuNow(q, "File", "Close Editor") },
	}
}

func scrollGraphAction() latencyAction {
	k := &actionKit{}
	return latencyAction{
		Owner: qa.OwnerGit,
		Prepare: func(q *QAApp, d DemoProject) (err error) {
			menuOrFail(q, viewMenu, "Source Control")
			k.target, err = q.TryWait(hasRoleLabel("list", "Graph"), q.waitFor())
			return err
		},
		Do: func(q *QAApp, d DemoProject, i int) {
			q.Send(map[string]any{"type": "wheel", "x": k.target.Bounds.CenterX(), "y": k.target.Bounds.CenterY(), "dy": stressGraphScroll})
		},
	}
}

func openBigFileAction(f bigFile) latencyAction {
	return latencyAction{
		Owner: qa.OwnerWorkspace, IdleFrames: blinkFramesPerSec,
		Reset: closeEditorAfterFirstRun,
		Do:    func(q *QAApp, d DemoProject, i int) { openFromExplorer(q, f.name) },
	}
}

func searchInFileAction(options func(d DemoProject) QAOptions, query string) latencyAction {
	return latencyAction{
		Owner: qa.OwnerSearch, IdleFrames: blinkFramesPerSec, Options: options,
		Prepare: func(q *QAApp, d DemoProject) error {
			_, err := focusCodeEditor(q)
			return err
		},
		Reset: func(q *QAApp, d DemoProject, i int) {
			q.Escape()
		},
		Do: func(q *QAApp, d DemoProject, i int) {
			q.Send(KeyEvent(findKey))
			q.Send(TextEvent(query))
			q.Send(KeyEvent("Enter"))
		},
	}
}

func stressActions() []latencyAction {
	tabs200 := stressTreeTabs(stressTabCount)
	actions := []latencyAction{
		openFolderAction().onFixture(stressFixtureTree, fmt.Sprintf("open the %dk-file folder", stressTreeFiles/1000), stressQuietLong, 0),
		expandFolderAction(stressFlatDir).onFixture(stressFixtureTree, fmt.Sprintf("expand a %dk-entry folder", stressFlatFiles/1000), stressQuietLong, stressSamples),
		quickQueryAction(goToFileMenu, stressQueryFile).onFixture(stressFixtureTree, "type a query in Go to File over the big folder", stressQuietLong, stressSamples),
		quickQueryAction(findInFilesMenu, stressNeedle).onFixture(stressFixtureTree, "text search over all files", stressQuietLong, stressBigSamples),
		switchTabAction(stressOptions(tabs200)).onFixture(stressFixtureTree, fmt.Sprintf("switch tab with %d tabs open", stressTabCount), stressQuietLong, stressSamples),
		closeAllTabsAction().onFixture(stressFixtureTree, fmt.Sprintf("close %d tabs", stressTabCount), stressQuietLong, 1).cold(),
		openGraphAction().onFixture(stressFixtureHistory, fmt.Sprintf("open the Graph (%dk commits)", stressHistoryCommits/1000), stressQuietLong, stressBigSamples),
		scrollGraphAction().onFixture(stressFixtureHistory, fmt.Sprintf("scroll the Graph (%dk commits)", stressHistoryCommits/1000), stressQuietLong, stressSamples),
		openSourceControlAction().onFixture(stressFixtureHistory, fmt.Sprintf("open Source Control (%dk commits)", stressHistoryCommits/1000), stressQuietLong, stressBigSamples),
		openSourceControlAction().onFixture(stressFixtureChanges, fmt.Sprintf("open Source Control with %dk changes", stressChangedFiles/1000), stressQuietLong, stressBigSamples),
		stackedDiffAction(firstChangeRow).onFixture(stressFixtureChanges, fmt.Sprintf("open the stacked Staged Changes diff (%dk files)", stressChangedFiles/1000), stressQuietLong, stressBigSamples),
	}
	for _, f := range stressBigFiles {
		onFile := stressOptions(stressOnFile(f.name))
		actions = append(actions,
			openBigFileAction(f).onFixture(stressFixtureFiles, "open the "+f.label, stressQuietLong, stressBigSamples).cold(),
			typeCharAction(onFile).onFixture(stressFixtureFiles, "type a character in the "+f.label, stressQuietLong, stressSamples),
			scrollAction(onFile, cmdTop, cmdBottom).onFixture(stressFixtureFiles, "scroll to the end of the "+f.label, stressQuietLong, stressBigSamples),
			searchInFileAction(onFile, stressEndNeedle).onFixture(stressFixtureFiles, "search in the "+f.label, stressQuietLong, stressBigSamples),
		)
	}
	return append([]latencyAction{
		closeTabsOneByOneAction().withOptions(stressOptions(stressTreeTabs(stressLeakTabs))).onFixture(stressFixtureTree, fmt.Sprintf("leak check: open and close %d tabs", stressLeakTabs), stressQuietShort, stressLeakTabs).leakCheck(),
		stackedDiffAction(firstChangeRow).onFixture(stressFixtureChanges, fmt.Sprintf("leak check: open the stacked diff %d times", stressLeakDiffs), stressQuietLong, stressLeakDiffs).leakCheck(),
	}, actions...)
}
