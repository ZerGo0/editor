//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ZerGo0/editor/e2e/qa"
)

const (
	perfEnv        = "EDITOR_E2E_PERF"
	perfSkipReason = "latency budgets need a quiet machine and windows that show: set EDITOR_E2E_PERF=1 to run them"

	latencySamples    = 10
	slowSamples       = 5
	perfConcurrency   = 8
	burstQuietShort   = 60 * time.Millisecond
	burstQuietAsync   = 300 * time.Millisecond
	burstMax          = 1500 * time.Millisecond
	burstPoll         = time.Millisecond
	blinkFramesPerSec = 2
	typedChar         = "x"
	saveKey           = "Cmd+S"
	quickQuery        = "utils"
	toggleSidebarName = "Toggle Primary Side Bar"
	rulePendingFrame  = "no-frame-after-input"
	ruleFrameStorm    = "frames-never-settle"
	fewTabs           = 3
	tabSwitchesNarrow = 12
	tabSwitchesWide   = 3
	maxTabSwitchFrame = 4
	tabSwitchQuiet    = 100 * time.Millisecond
	nextEditorMenu    = "Next Editor"
	tabShownSlack     = 1.0
	editorTabsLabel   = "Editor Tabs"
	changedRowPoll    = 50 * time.Millisecond
	demoChangedFile   = "file00.go"
	findRowScroll     = 20
	scrollStep        = 600
)

type Burst struct {
	First      time.Duration
	Settled    time.Duration
	Frames     []qa.Frame
	Continuous bool
}

func closeEditorAfterFirstRun(q *QAApp, _ DemoProject, i int) {
	if i > 0 {
		menuOrFail(q, "File", "Close Editor")
		q.Settle()
	}
}

func (q *QAApp) Burst(quiet, maxWait time.Duration, do func()) Burst {
	q.t.Helper()
	offset := q.outputLen()
	start := time.Now()
	do()
	var first, last time.Time
	seen := 0
	continuous := true
	for {
		if n := qa.CountBuiltFrames(q.outputFrom(offset)); n > seen {
			seen = n
			last = time.Now()
			if first.IsZero() {
				first = last
			}
		}
		if !last.IsZero() && time.Since(last) >= quiet {
			continuous = false
			break
		}
		if time.Since(start) >= maxWait {
			break
		}
		time.Sleep(burstPoll)
	}
	b := Burst{Frames: qa.Built(qa.ParseFrames(q.outputFrom(offset))), Continuous: continuous && !last.IsZero()}
	if !first.IsZero() {
		b.First, b.Settled = first.Sub(start), last.Sub(start)
	}
	return b
}

type latencyAction struct {
	Name       string
	Owner      string
	Samples    int
	Quiet      time.Duration
	IdleFrames int
	Options    func(d DemoProject) QAOptions
	Prepare    func(q *QAApp, d DemoProject) error
	Reset      func(q *QAApp, d DemoProject, i int)
	Do         func(q *QAApp, d DemoProject, i int)
	Verify     func(q *QAApp, d DemoProject) error
	MaxWait    time.Duration
	Timeout    time.Duration
	Cold       bool
	Fixture    string
	Project    func(t *testing.T) DemoProject
}

const largeFileEditorLabel = "Large file editor"

func isTextEditor(n qa.Node) bool {
	return isCodeEditor(n) || n.Role == "textField" && n.Label == largeFileEditorLabel
}

func focusCodeEditor(q *QAApp) (qa.Node, error) {
	editor, err := q.TryWait(isTextEditor, q.waitFor())
	if err != nil {
		return editor, fmt.Errorf("code editor: %w", err)
	}
	q.FocusNode(editor)
	return editor, nil
}

func allTabsOptions(d DemoProject) QAOptions {
	o := defaultLatencyOptions(d)
	o.Tabs = d.TabFiles
	return o
}

func bigFileOptions(d DemoProject) QAOptions {
	o := defaultLatencyOptions(d)
	o.Tabs = []string{d.BigFile}
	return o
}

func defaultLatencyOptions(d DemoProject) QAOptions {
	return QAOptions{Width: normalWindowW, Height: normalWindowH, SidebarWidth: defaultSidebar, Theme: themeDark, Tabs: d.TabFiles[:fewTabs], FrameStats: perfRun()}
}

func menuOrFail(q *QAApp, path ...string) {
	q.t.Helper()
	if err := q.TryMenu(path...); err != nil {
		q.t.Fatalf("menu %v: %v", path, err)
	}
}

func latencyActions() []latencyAction {
	return []latencyAction{
		typeCharAction(bigFileOptions).with("type a character in a 5k-line file", latencySamples, burstQuietShort),
		scrollAction(bigFileOptions, "", "").with("scroll the 5k-line file", latencySamples, burstQuietShort),
		latencyAction{
			Owner: qa.OwnerSearch, IdleFrames: blinkFramesPerSec,
			Reset: func(q *QAApp, d DemoProject, i int) { q.Escape() },
			Do:    func(q *QAApp, d DemoProject, i int) { menuNow(q, goMenu, "Go to File...") },
		}.with("open Go to File", latencySamples, burstQuietShort),
		quickQueryAction(goToFileMenuPath, quickQuery, labelPrefix(quickQuery)).with("type a query in Go to File", latencySamples, burstQuietShort),
		switchTabAction(allTabsOptions).with("switch tab", latencySamples, burstQuietShort),
		toggleSidebarAction().with("toggle sidebar", latencySamples, burstQuietShort),
		expandFolderAction(demoChangelogDir).with("expand a 2000-entry folder", latencySamples, burstQuietShort),
		openSourceControlAction().with("open Source Control with 500+ changes", slowSamples, burstQuietAsync),
		stackedDiffAction(func(q *QAApp) (qa.Node, error) { return q.waitChangedRow(isDemoChangedFile) }).with("open the stacked Staged Changes diff (210 files)", slowSamples, burstQuietAsync),
		openGraphAction().with("open the Graph (2100 commits)", slowSamples, burstQuietAsync),
		saveAction(bigFileOptions).with("save the 5k-line file", slowSamples, burstQuietAsync).isolated(),
	}
}

func menuNow(q *QAApp, path ...string) {
	q.t.Helper()
	if _, err := q.Call("menu", map[string]any{"window": q.Win, "path": path}); err != nil {
		q.t.Fatalf("menu %v: %v", path, err)
	}
}

func activeTabShown(tree *qa.Tree) (qa.Node, bool) {
	tab, ok := tree.Find(func(n qa.Node) bool {
		list, found := tree.ByID(n.Parent)
		return n.Role == "tab" && n.Has("checked") && found && list.Label == editorTabsLabel
	})
	if !ok {
		return tab, false
	}
	clip, ok := tree.ByID(tab.Parent)
	for ok && clip.Role != "scroll" {
		clip, ok = tree.ByID(clip.Parent)
	}
	return tab, ok && clip.Bounds.Contains(tab.Bounds, tabShownSlack)
}

func TestTabSwitchSettles(t *testing.T) {
	t.Parallel()
	demo := NewDemoProject(t)
	for _, c := range []struct{ width, switches int }{{smallestWindowW, tabSwitchesNarrow}, {normalWindowW, tabSwitchesWide}} {
		t.Run(fmt.Sprint(c.width), func(t *testing.T) {
			o := defaultLatencyOptions(demo)
			o.Width, o.Tabs = c.width, demo.TabFiles
			q := LaunchQA(t, demo.Root, o)
			for i := range c.switches {
				frames := len(q.Burst(tabSwitchQuiet, idleWatchWindow, func() { menuNow(q, viewMenu, nextEditorMenu) }).Frames)
				tab, shown := activeTabShown(q.Snapshot())
				if o.FrameStats && frames > maxTabSwitchFrame {
					t.Errorf("switch %d: %d frames in %s, want at most %d", i, frames, idleWatchWindow, maxTabSwitchFrame)
				}
				if !shown {
					t.Errorf("switch %d: active tab %q at %+v is not scrolled into the tab strip", i, tab.Label, tab.Bounds)
				}
			}
		})
	}
}

func perfRun() bool { return os.Getenv(perfEnv) != "" }

func TestInputLatencyBudget(t *testing.T) {
	if !perfRun() {
		t.Skip(perfSkipReason)
	}
	t.Parallel()
	stressPrepare(t)
	run := newLatencyRun(NewDemoProject(t))
	slots := make(chan struct{}, perfConcurrency)
	t.Run("scenarios", func(t *testing.T) {
		for i, action := range append(stressActions(), latencyActions()...) {
			t.Run(strings.ReplaceAll(action.Name, " ", "-"), func(t *testing.T) {
				t.Parallel()
				slots <- struct{}{}
				defer func() { <-slots }()
				run.action(t, i, action)
			})
		}
	})
	run.finish(t)
}

func isDemoChangedFile(label string) bool { return hasName(label, demoChangedFile) }

func firstChangedRow(tree *qa.Tree, match func(label string) bool) (qa.Node, bool) {
	list, ok := tree.Find(RoleLabel("list", "Changes"))
	if !ok {
		return qa.Node{}, false
	}
	return tree.Find(func(n qa.Node) bool {
		return n.Parent == list.ID && n.Role == "listItem" && match(n.Label) && list.Bounds.Contains(n.Bounds, 0)
	})
}

func (q *QAApp) waitChangedRow(match func(label string) bool) (qa.Node, error) {
	deadline := time.Now().Add(q.waitFor())
	for time.Now().Before(deadline) {
		tree := q.Snapshot()
		if n, ok := firstChangedRow(tree, match); ok {
			return n, nil
		}
		if list, ok := tree.Find(RoleLabel("list", "Changes")); ok {
			q.Send(map[string]any{"type": "wheel", "x": list.Bounds.CenterX(), "y": list.Bounds.CenterY(), "dy": findRowScroll})
		}
		time.Sleep(changedRowPoll)
	}
	return qa.Node{}, fmt.Errorf("no changed file row in Source Control within %s", q.waitFor())
}
