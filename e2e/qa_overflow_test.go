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
	ruleUnreachable = "view-unreachable"
	ruleWording     = "label-wording"
	minTitleWords   = 2
)

type overflowConfig struct {
	Name    string
	Width   int
	Height  int
	Sidebar float32
}

var overflowConfigs = []overflowConfig{
	{"smallest-window", smallestWindowW, smallestWindowH, narrowestSidebar},
	{"narrow-sidebar", normalWindowW, normalWindowH, narrowestSidebar},
	{"normal", normalWindowW, normalWindowH, defaultSidebar},
}

var overflowThemes = []string{themeDark, themeLight}

var overflowAllow = []qa.Allow{
	{Rule: qa.RuleTextCut, View: "search-all", Role: "text", LabelPrefix: "notes-with-a-rather-long-name", Reason: "internal/search/finder: at the 640 px smallest window a file row's name and directory share about 580 px of text that needs 1030 px, so both cannot keep 60% of their width; the row cuts both with an ellipsis as Go to File does"},
	{Rule: qa.RuleTextCut, View: "search-all", Role: "text", LabelPrefix: "internal/a-very-long-directory-name", Reason: "internal/search/finder: the directory of the same file row, see above"},
}

func themeSuffix(theme string) string { return strings.ToLower(theme) }

func contextName(c overflowConfig, theme string) string { return c.Name + "-" + themeSuffix(theme) }

func checkSnapshot(q *QAApp, view, context string, measure qa.TextMeasurer) []qa.Flag {
	tree := q.Snapshot()
	flags := qa.CheckLayout(tree, qa.LayoutOptions{View: view, Measurer: measure})
	flags = append(flags, qa.CheckInteractiveLabels(view, tree)...)
	flags = append(flags, labelWordingFlags(view, tree)...)
	for i := range flags {
		flags[i].Context = context
	}
	return qa.Dedupe(flags)
}

var wordingRoles = map[string]bool{"button": true, "menuButton": true, "menuItem": true, "toggleButton": true, "popUpButton": true}

func labelWordingFlags(view string, tree *qa.Tree) []qa.Flag {
	var flags []qa.Flag
	for _, n := range tree.Nodes {
		if !wordingRoles[n.Role] || n.Label == "" {
			continue
		}
		for _, issue := range qa.CheckTitle(n.Label, false, false) {
			if issue.Rule == qa.RuleTitleCase && dataLike(n.Label) {
				continue
			}
			flags = append(flags, qa.Flag{Rule: ruleWording + "/" + issue.Rule, View: view, Owner: qa.Owner(view, n, tree.Window), Role: n.Role, Label: n.Label, Bounds: n.Bounds,
				Measured: issue.Message, Expected: "Title Case labels without trailing punctuation or double spaces", Fix: "rename the label as VS Code words it"})
		}
	}
	return flags
}

func dataLike(label string) bool {
	return len(strings.Fields(label)) < minTitleWords || strings.ContainsAny(label, "0123456789:/.")
}

func unreachableFlag(view, context string, err error) qa.Flag {
	return qa.Flag{Rule: ruleUnreachable, View: view, Context: context, Owner: qa.Owner(view, qa.Node{}, qa.Rect{}), Role: "view", Label: view,
		Measured: err.Error(), Expected: "the view opens from its menu item or command", Fix: "restore the menu item, command or the label the QA step waits for (e2e/qa_nav_test.go)"}
}

func TestOverflowAndEdgeCases(t *testing.T) {
	t.Parallel()
	measure := uiTextMeasurer()
	var all []qa.Flag
	var mu sync.Mutex
	t.Cleanup(func() {
		for _, a := range qa.Stale(all, overflowAllow) {
			t.Errorf("stale allowlist entry (remove it): %+v", a)
		}
	})
	for _, cfg := range overflowConfigs {
		for _, theme := range overflowThemes {
			context := contextName(cfg, theme)
			t.Run(context, func(t *testing.T) {
				t.Parallel()
				demo := NewDemoProject(t)
				q := LaunchQA(t, demo.Root, QAOptions{Width: cfg.Width, Height: cfg.Height, SidebarWidth: cfg.Sidebar, Theme: theme, Tabs: demo.TabFiles, ActiveTab: 0})
				var flags []qa.Flag
				visit := func(view string) {
					flags = append(flags, checkSnapshot(q, view, context, measure)...)
					q.SaveShot(fmt.Sprintf("%s-%s-%s", view, cfg.Name, themeSuffix(theme)))
				}
				visit("tabs")
				for _, step := range demoViewSteps() {
					if err := step.Enter(q, demo); err != nil {
						flags = append(flags, unreachableFlag(step.Name, context, err))
						continue
					}
					q.Settle()
					visit(step.Name)
				}
				flags = qa.Dedupe(flags)
				kept, _ := qa.Filter(flags, overflowAllow)
				mu.Lock()
				all = append(all, flags...)
				mu.Unlock()
				report := qa.Format(kept)
				q.Report("overflow-"+context, report)
				q.SaveCrops("overflow-"+context, kept, q.Snapshot().Window)
				if len(kept) > 0 {
					t.Errorf("%d flags:\n%s", len(kept), report)
				}
			})
		}
	}
}
