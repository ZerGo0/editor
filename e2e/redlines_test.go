//go:build e2e

package e2e

import (
	"fmt"
	"image"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ZerGo0/editor/e2e/qa"
	"github.com/ZerGo0/editor/internal/design/redline"
)

const (
	redlineAllowFile = "testdata/redline_allowlist.txt"
	redlineUpdateEnv = "EDITOR_REDLINE_UPDATE"
	stateDefault     = "default"
	stateHover       = "hover"
	stateDisabled    = "disabled"
	stateOffscreen   = "offscreen"
	pointerMoveEvent = "pointerMove"
	tooltipRole      = "tooltip"
	tooltipShowWait  = 5 * time.Second
	captureTries     = 5
)

func treeBoxes(tree *qa.Tree) []redline.Box {
	boxes := make([]redline.Box, 0, len(tree.Nodes))
	for _, n := range tree.Nodes {
		if n.Has(stateOffscreen) {
			continue
		}
		boxes = append(boxes, redline.Box{ID: n.ID, Parent: n.Parent, Role: n.Role, Label: n.Name(), Disabled: n.Has(stateDisabled),
			Rect: redline.Rect{X: n.Bounds.X, Y: n.Bounds.Y, W: n.Bounds.Width, H: n.Bounds.Height}})
	}
	return boxes
}

func (q *QAApp) Redlines(name string) (redline.Result, error) {
	q.t.Helper()
	img, tree, err := q.stableCapture()
	if err != nil {
		return redline.Result{}, fmt.Errorf("capture %s: %w", name, err)
	}
	scale, err := redline.ScaleOf(img, tree.Window.Width)
	if err != nil {
		return redline.Result{}, fmt.Errorf("scale of %s: %w", name, err)
	}
	res, err := redline.Analyze(redline.Input{Name: name, Image: img, Boxes: treeBoxes(tree), Scale: scale})
	if err != nil {
		return redline.Result{}, fmt.Errorf("redlines %s: %w", name, err)
	}
	if dir := artifactDir(); dir != "" {
		if err := writePNG(filepath.Join(dir, name+".png"), img); err != nil {
			return res, fmt.Errorf("write %s: %w", name, err)
		}
		if _, err := res.Write(dir); err != nil {
			return res, err
		}
	}
	return res, nil
}

func (q *QAApp) stableCapture() (image.Image, *qa.Tree, error) {
	q.t.Helper()
	before := q.Snapshot()
	for range captureTries {
		img, err := q.Capture()
		if err != nil {
			return nil, nil, err
		}
		after := q.Snapshot()
		if reflect.DeepEqual(before.Nodes, after.Nodes) {
			return img, after, nil
		}
		q.Settle()
		before = after
	}
	return nil, nil, fmt.Errorf("the accessibility tree changed during each of %d captures", captureTries)
}

func (q *QAApp) hover(n qa.Node, wantTooltip bool) error {
	q.Send(map[string]any{"type": pointerMoveEvent, "x": n.Bounds.CenterX(), "y": n.Bounds.CenterY()})
	q.Settle()
	if wantTooltip {
		if _, err := q.TryWait(func(t qa.Node) bool { return t.Role == tooltipRole }, tooltipShowWait); err != nil {
			return fmt.Errorf("tooltip after hover on %q: %w", n.Label, err)
		}
	}
	q.Settle()
	return nil
}

var redlineAllowMu sync.Mutex

func allowedFindings(allow *redline.Allowlist, res redline.Result) []int {
	redlineAllowMu.Lock()
	defer redlineAllowMu.Unlock()
	return allow.Check(res)
}

func checkViewRedlines(t *testing.T, q *QAApp, allow *redline.Allowlist, view, state, theme string) {
	t.Helper()
	res, err := q.Redlines(fmt.Sprintf("%s-%s-%s", view, state, themeSuffix(theme)))
	if err != nil {
		t.Error(err)
		return
	}
	if bad := allowedFindings(allow, res); len(bad) > 0 {
		t.Errorf("%d redline findings in %s (fix them, or add a reviewed entry to e2e/%s):\n%s",
			len(bad), res.Name, redlineAllowFile, res.Describe(bad))
	}
}

var redlineTooltipViews = map[string]bool{"tabs": true, "search-text": true}

var redlineHoverTargets = map[string]func(qa.Node) bool{
	"tabs":        func(n qa.Node) bool { return n.Role == "tab" },
	"explorer":    func(n qa.Node) bool { return n.Role == "treeItem" },
	"scm":         func(n qa.Node) bool { return n.Role == "treeItem" || n.Role == "listItem" },
	"search-text": func(n qa.Node) bool { return strings.HasPrefix(n.Label, matchRowPrefix) },
}

func TestViewRedlines(t *testing.T) {
	t.Parallel()
	allow, err := redline.LoadAllowlist(redlineAllowFile)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { allow.Settle(t, redlineUpdateEnv) })
	for _, theme := range []string{themeDark, themeLight} {
		t.Run(theme, func(t *testing.T) {
			t.Parallel()
			demo := githubDemoProject(t)
			q := LaunchQA(t, demo.Root, githubOptions(demo, QAOptions{Width: normalWindowW, Height: normalWindowH, SidebarWidth: defaultSidebar, Theme: theme}))
			visit := func(view string) {
				checkViewRedlines(t, q, allow, view, stateDefault, theme)
				target, ok := redlineHoverTargets[view]
				if !ok {
					return
				}
				if n, found := q.Snapshot().Find(target); found {
					if err := q.hover(n, redlineTooltipViews[view]); err != nil {
						t.Errorf("%s: %v", view, err)
						return
					}
					checkViewRedlines(t, q, allow, view, stateHover, theme)
					q.Send(map[string]any{"type": pointerMoveEvent, "x": 0, "y": 0})
					q.Settle()
				}
			}
			visit("tabs")
			for _, step := range allViewSteps() {
				if err := step.Enter(q, demo); err != nil {
					t.Errorf("open %s: %v", step.Name, err)
					continue
				}
				q.Settle()
				visit(step.Name)
			}
		})
	}
}
