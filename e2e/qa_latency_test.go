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
	budgetTypeChar      = 50 * time.Millisecond
	budgetQuickOpen     = 100 * time.Millisecond
	budgetQuickQuery    = 100 * time.Millisecond
	budgetTabSwitch     = 100 * time.Millisecond
	budgetToggleSidebar = 100 * time.Millisecond
	budgetExpandFolder  = 100 * time.Millisecond
	budgetOpenSCM       = 500 * time.Millisecond
	budgetOpenGraph     = 800 * time.Millisecond
	budgetOpenDiffStack = 800 * time.Millisecond
	budgetFirstFrame    = 200 * time.Millisecond
	perfEnv             = "EDITOR_E2E_PERF"
	perfSkipReason      = "latency budgets need a quiet machine: set EDITOR_E2E_PERF=1 to run them"

	latencySamples    = 20
	slowSamples       = 8
	burstQuietShort   = 60 * time.Millisecond
	burstQuietAsync   = 300 * time.Millisecond
	burstMax          = 1500 * time.Millisecond
	burstPoll         = time.Millisecond
	blinkFramesPerSec = 2
	typedChar         = "x"
	quickQuery        = "utils"
	toggleSidebarName = "Toggle Primary Side Bar"
	rulePendingFrame  = "no-frame-after-input"
	ruleFrameStorm    = "frames-never-settle"
	fewTabs           = 3
	tabSwitchesNarrow = 12
	tabSwitchesWide   = 3
	maxTabSwitchFrame = 4
	nextEditorMenu    = "Next Editor"
	tabShownSlack     = 1.0
	editorTabsLabel   = "Editor Tabs"
	changedRowPoll    = 50 * time.Millisecond
	demoChangedFile   = "file00.go"
	findRowScroll     = 20
)

type Burst struct {
	First      time.Duration
	Settled    time.Duration
	Frames     []qa.Frame
	Continuous bool
}

func (q *QAApp) Burst(quiet time.Duration, do func()) Burst {
	q.t.Helper()
	offset := q.outputLen()
	start := time.Now()
	do()
	var first, last time.Time
	seen := 0
	continuous := true
	for {
		if n := qa.CountFrames(q.outputFrom(offset)); n > seen {
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
		if time.Since(start) >= burstMax {
			break
		}
		time.Sleep(burstPoll)
	}
	b := Burst{Frames: qa.ParseFrames(q.outputFrom(offset)), Continuous: continuous && !last.IsZero()}
	if !first.IsZero() {
		b.First, b.Settled = first.Sub(start), last.Sub(start)
	}
	return b
}

type latencyAction struct {
	Name        string
	Owner       string
	Budget      time.Duration
	Samples     int
	Quiet       time.Duration
	IdleFrames  int
	Options     func(d DemoProject) QAOptions
	Prepare     func(q *QAApp, d DemoProject) error
	Reset       func(q *QAApp, d DemoProject, i int)
	Do          func(q *QAApp, d DemoProject, i int)
	EditorFocus bool
}

func focusCodeEditor(q *QAApp) error {
	editor, err := q.TryWait(isCodeEditor, navTimeout)
	if err != nil {
		return fmt.Errorf("code editor: %w", err)
	}
	q.FocusNode(editor)
	return nil
}

func defaultLatencyOptions(d DemoProject) QAOptions {
	return QAOptions{Width: normalWindowW, Height: normalWindowH, SidebarWidth: defaultSidebar, Theme: themeDark, Tabs: d.TabFiles[:fewTabs], FrameStats: true}
}

func menuOrFail(q *QAApp, path ...string) {
	q.t.Helper()
	if err := q.TryMenu(path...); err != nil {
		q.t.Fatalf("menu %v: %v", path, err)
	}
}

func clickLabel(q *QAApp, role, label string) error {
	n, err := q.TryWait(hasRoleLabel(role, label), navTimeout)
	if err != nil {
		return fmt.Errorf("%s %q: %w", role, label, err)
	}
	q.ClickNode(n)
	return nil
}

func latencyActions() []latencyAction {
	return []latencyAction{
		{
			Name: "type a character in a 5k-line file", Owner: qa.OwnerWorkspace, Budget: budgetTypeChar, Samples: latencySamples, Quiet: burstQuietShort, IdleFrames: blinkFramesPerSec, EditorFocus: true,
			Options: func(d DemoProject) QAOptions {
				o := defaultLatencyOptions(d)
				o.Tabs = []string{d.BigFile}
				return o
			},
			Prepare: func(q *QAApp, d DemoProject) error { return focusCodeEditor(q) },
			Do:      func(q *QAApp, d DemoProject, i int) { q.Send(TextEvent(typedChar)) },
		},
		{
			Name: "open Go to File", Owner: qa.OwnerSearch, Budget: budgetQuickOpen, Samples: latencySamples, Quiet: burstQuietShort,
			Reset: func(q *QAApp, d DemoProject, i int) { q.Escape() },
			Do:    func(q *QAApp, d DemoProject, i int) { menuOrFail(q, goMenu, "Go to File...") },
		},
		{
			Name: "type a query in Go to File", Owner: qa.OwnerSearch, Budget: budgetQuickQuery, Samples: latencySamples, Quiet: burstQuietShort,
			Prepare: func(q *QAApp, d DemoProject) error { return q.TryMenu(goMenu, "Go to File...") },
			Reset: func(q *QAApp, d DemoProject, i int) {
				q.Send(KeyEvent("Cmd+A"))
				q.Send(KeyEvent("Backspace"))
				q.Settle()
			},
			Do: func(q *QAApp, d DemoProject, i int) { q.Send(TextEvent(quickQuery)) },
		},
		{
			Name: "switch tab", Owner: qa.OwnerWorkspace, Budget: budgetTabSwitch, Samples: latencySamples, Quiet: burstQuietShort,
			Options: func(d DemoProject) QAOptions {
				o := defaultLatencyOptions(d)
				o.Tabs = d.TabFiles
				return o
			},
			Do: func(q *QAApp, d DemoProject, i int) { menuNow(q, viewMenu, nextEditorMenu) },
		},
		{
			Name: "toggle sidebar", Owner: qa.OwnerWorkspace, Budget: budgetToggleSidebar, Samples: latencySamples, Quiet: burstQuietShort,
			Do: func(q *QAApp, d DemoProject, i int) {
				n, err := q.TryWait(hasRoleLabel("button", toggleSidebarName), navTimeout)
				if err != nil {
					q.t.Fatalf("toggle sidebar button: %v", err)
				}
				q.Send(ClickEvent(n.Bounds.CenterX(), n.Bounds.CenterY()))
			},
		},
		{
			Name: "expand a 2000-entry folder", Owner: qa.OwnerWorkspace, Budget: budgetExpandFolder, Samples: latencySamples, Quiet: burstQuietShort,
			Prepare: func(q *QAApp, d DemoProject) error {
				files, err := q.TryWait(func(n qa.Node) bool { return n.Role == "tree" }, navTimeout)
				if err != nil {
					return err
				}
				q.Send(map[string]any{"type": "wheel", "x": files.Bounds.CenterX(), "y": files.Bounds.CenterY(), "dy": -scrollToTop})
				q.Settle()
				return nil
			},
			Reset: func(q *QAApp, d DemoProject, i int) {
				if n, ok := q.Snapshot().Find(hasRoleLabel("treeItem", demoChangelogDir)); ok {
					q.Act(q.Win, n.ID, "collapse", "")
					q.Settle()
				}
			},
			Do: func(q *QAApp, d DemoProject, i int) {
				n, ok := q.Snapshot().Find(hasRoleLabel("treeItem", demoChangelogDir))
				if !ok {
					q.t.Fatalf("folder %s not in the tree", demoChangelogDir)
				}
				q.Act(q.Win, n.ID, "expand", "")
			},
		},
		{
			Name: "open Source Control with 500+ changes", Owner: qa.OwnerGit, Budget: budgetOpenSCM, Samples: slowSamples, Quiet: burstQuietAsync,
			Reset: func(q *QAApp, d DemoProject, i int) {
				menuOrFail(q, viewMenu, "Explorer")
				q.Settle()
			},
			Do: func(q *QAApp, d DemoProject, i int) { menuOrFail(q, viewMenu, "Source Control") },
		},
		{
			Name: "open the stacked Staged Changes diff (210 files)", Owner: qa.OwnerGit, Budget: budgetOpenDiffStack, Samples: slowSamples, Quiet: burstQuietAsync,
			Prepare: func(q *QAApp, d DemoProject) error {
				menuOrFail(q, viewMenu, "Source Control")
				_, err := q.TryWait(hasLabel("Commit Message"), navTimeout)
				return err
			},
			Reset: func(q *QAApp, d DemoProject, i int) {
				if i > 0 {
					menuOrFail(q, "File", "Close Editor")
					q.Settle()
				}
			},
			Do: func(q *QAApp, d DemoProject, i int) {
				row, err := q.waitChangedRow()
				if err != nil {
					q.t.Fatal(err)
				}
				q.ClickNode(row)
			},
		},
		{
			Name: "open the Graph (2100 commits)", Owner: qa.OwnerGit, Budget: budgetOpenGraph, Samples: slowSamples, Quiet: burstQuietAsync,
			Reset: func(q *QAApp, d DemoProject, i int) {
				if i > 0 {
					menuOrFail(q, "File", "Close Editor")
					q.Settle()
				}
			},
			Do: func(q *QAApp, d DemoProject, i int) { menuOrFail(q, gitMenu, "Show Log") },
		},
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
				offset := q.outputLen()
				menuNow(q, viewMenu, nextEditorMenu)
				tab, shown := activeTabShown(q.Snapshot())
				time.Sleep(idleWatchWindow)
				if frames := qa.CountFrames(q.outputFrom(offset)); frames > maxTabSwitchFrame {
					t.Errorf("switch %d: %d frames in %s, want at most %d", i, frames, idleWatchWindow, maxTabSwitchFrame)
				}
				if !shown {
					t.Errorf("switch %d: active tab %q at %+v is not scrolled into the tab strip", i, tab.Label, tab.Bounds)
				}
			}
		})
	}
}

func TestInputLatencyBudget(t *testing.T) {
	if os.Getenv(perfEnv) == "" {
		t.Skip(perfSkipReason)
	}
	demo := NewDemoProject(t)
	var report strings.Builder
	fmt.Fprintf(&report, "%-42s %-44s %-44s %-12s %s\n", "action", "input to last frame", "input to first frame", "frames", "slowest frame / idle frames")
	var flags []qa.Flag
	for _, action := range latencyActions() {
		t.Run(strings.ReplaceAll(action.Name, " ", "-"), func(t *testing.T) {
			opts := defaultLatencyOptions(demo)
			if action.Options != nil {
				opts = action.Options(demo)
			}
			q := LaunchQA(t, demo.Root, opts)
			if action.Prepare != nil {
				if err := action.Prepare(q, demo); err != nil {
					t.Fatalf("prepare: %v", err)
				}
			}
			var settled, first []time.Duration
			var frameCounts []int
			var slowest qa.Frame
			for i := range action.Samples + 1 {
				if action.Reset != nil {
					action.Reset(q, demo, i)
				}
				b := q.Burst(action.Quiet, func() { action.Do(q, demo, i) })
				if i == 0 {
					continue
				}
				if len(b.Frames) == 0 {
					flags = append(flags, qa.Flag{Rule: rulePendingFrame, View: action.Name, Owner: action.Owner, Role: "action", Label: action.Name, Measured: "no frame drawn after the input", Expected: "the input draws a frame", Fix: "check that the action reaches the UI"})
					continue
				}
				if b.Continuous {
					flags = append(flags, qa.Flag{Rule: ruleFrameStorm, View: action.Name, Owner: action.Owner, Role: "action", Label: action.Name, Measured: fmt.Sprintf("%d frames in %s after one input, still drawing", len(b.Frames), burstMax), Expected: "the input settles within a few frames (no continuous redraw)", Fix: "find the view that requests a frame every frame (scroll-into-view retry, After loop, Animate) and make it converge"})
				}
				settled, first = append(settled, b.Settled), append(first, b.First)
				frameCounts = append(frameCounts, len(b.Frames))
				if s := qa.SlowestFrame(b.Frames); s.Total > slowest.Total {
					slowest = s
				}
			}
			stats, firstStats := qa.Summarize(settled), qa.Summarize(first)
			if !action.EditorFocus {
				q.Settle()
			}
			idle := q.FramesWhileIdle()
			fmt.Fprintf(&report, "%-42s %-44s %-44s %-12d %s / %d\n", action.Name, stats, firstStats, frameCounts[len(frameCounts)/2], slowest.Total.Round(time.Microsecond*100), idle)
			flags = append(flags, qa.OverBudget(action.Name, action.Owner, stats, action.Budget)...)
			flags = append(flags, qa.OverBudget(action.Name+" (first frame)", action.Owner, firstStats, budgetFirstFrame)...)
			flags = append(flags, qa.IdleFlag(action.Name, action.Owner, max(idle-action.IdleFrames, 0), idleWatchWindow)...)
			q.Report("latency-"+strings.ReplaceAll(action.Name, " ", "-"), stats.String()+"\n"+firstStats.String())
		})
	}
	t.Logf("\n%s", report.String())
	if dir := artifactDir(); dir != "" {
		writeReport(t, "latency", report.String())
	}
	if len(flags) > 0 {
		t.Errorf("%d flags:\n%s", len(qa.Dedupe(flags)), qa.Format(qa.Dedupe(flags)))
	}
}

func firstChangedRow(tree *qa.Tree) (qa.Node, bool) {
	list, ok := tree.Find(hasRoleLabel("list", "Changes"))
	if !ok {
		return qa.Node{}, false
	}
	return tree.Find(func(n qa.Node) bool {
		return n.Parent == list.ID && n.Role == "listItem" && n.Label == demoChangedFile && list.Bounds.Contains(n.Bounds, 0)
	})
}

func (q *QAApp) waitChangedRow() (qa.Node, error) {
	deadline := time.Now().Add(navTimeout)
	for time.Now().Before(deadline) {
		tree := q.Snapshot()
		if n, ok := firstChangedRow(tree); ok {
			return n, nil
		}
		if list, ok := tree.Find(hasRoleLabel("list", "Changes")); ok {
			q.Send(map[string]any{"type": "wheel", "x": list.Bounds.CenterX(), "y": list.Bounds.CenterY(), "dy": findRowScroll})
		}
		time.Sleep(changedRowPoll)
	}
	return qa.Node{}, fmt.Errorf("no changed file row in Source Control within %s", navTimeout)
}
