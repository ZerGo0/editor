//go:build e2e

package e2e

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZerGo0/editor/e2e/qa"
)

const (
	keybindingsFileName   = "keybindings.json"
	stressUserKeys        = "abcdefghijklmnopqrstuvwxyz0123456789"
	stressUserKeyFunction = 12
	stressUserModifiers   = 4
	stressSettingsQuery   = "font"
	stressKeysQuery       = "save"
	stressListScroll      = 2000
	settingsMenuItem      = "Settings"
	keyboardMenuItem      = "Keyboard Shortcuts"
	settingsListLabel     = "Settings"
	settingsSearchLabel   = "Search settings"
	keysSearchLabel       = "Search keyboard shortcuts"
	keysListLabel         = "Keyboard Shortcuts"
)

var (
	stressUserCommands = []string{
		"workbench.action.splitEditorRight", "workbench.action.splitEditorDown", "workbench.action.nextEditor", "workbench.action.previousEditor",
		"workbench.action.closeActiveEditor", "workbench.action.files.save", "workbench.action.files.saveAll", "workbench.action.toggleSidebarVisibility",
		"workbench.action.showCommands", "workbench.action.openSettings", "workbench.action.quickOpen", "workbench.action.findInFiles",
		"workbench.action.searchEverywhere", "workbench.action.navigateBack", "workbench.action.navigateForward", "workbench.action.togglePanel",
		"workbench.view.explorer", "workbench.view.scm", "workbench.view.github", "workbench.action.zoomIn", "workbench.action.zoomOut", "editor.action.toggleWordWrap",
	}
	stressUserChordPrefixes = []string{"", "cmd+k ", "cmd+j "}
)

type userBinding struct {
	Key     string `json:"key"`
	Command string `json:"command"`
}

func stressUserKeyNames() []string {
	names := strings.Split(stressUserKeys, "")
	for i := 1; i <= stressUserKeyFunction; i++ {
		names = append(names, fmt.Sprintf("f%d", i))
	}
	return names
}

func stressUserBindings() []userBinding {
	var out []userBinding
	for _, prefix := range stressUserChordPrefixes {
		for mask := 1; mask < 1<<stressUserModifiers; mask++ {
			mods := []string{}
			for bit, name := range []string{"ctrl", "alt", "shift", "cmd"} {
				if mask>>bit&1 == 1 {
					mods = append(mods, name)
				}
			}
			for _, key := range stressUserKeyNames() {
				out = append(out, userBinding{Key: prefix + strings.Join(mods, "+") + "+" + key, Command: stressUserCommands[len(out)%len(stressUserCommands)]})
			}
		}
	}
	return out
}

func seedStressKeybindings(t *testing.T, root, userData string) {
	t.Helper()
	writeJSON(t, filepath.Join(userData, keybindingsFileName), stressUserBindings())
}

func stressKeybindingsOptions(d DemoProject) QAOptions {
	o := stressOptions(nil)(d)
	o.Seed = seedStressKeybindings
	return o
}

func openPrefsView(item, searchLabel string) func(q *QAApp, d DemoProject) (qa.Node, error) {
	return func(q *QAApp, d DemoProject) (qa.Node, error) {
		if err := q.TryMenu(editorMenu, item); err != nil {
			return qa.Node{}, err
		}
		search, err := q.TryWait(RoleLabel("textField", searchLabel), q.waitFor())
		if err != nil {
			return search, fmt.Errorf("%s search field: %w", item, err)
		}
		q.FocusNode(search)
		return search, nil
	}
}

func filterPrefsAction(open func(q *QAApp, d DemoProject) (qa.Node, error), query, searchLabel string) latencyAction {
	return latencyAction{
		Owner: qa.OwnerPrefs, IdleFrames: blinkFramesPerSec,
		Prepare: func(q *QAApp, d DemoProject) error {
			_, err := open(q, d)
			return err
		},
		Reset:  clearFocusedField,
		Do:     typeText(query),
		Verify: expectFieldValue(searchLabel, query),
	}
}

func scrollPrefsAction(open func(q *QAApp, d DemoProject) (qa.Node, error), listLabel string) latencyAction {
	k := &actionKit{}
	return latencyAction{
		Owner: qa.OwnerPrefs, IdleFrames: blinkFramesPerSec,
		Prepare: func(q *QAApp, d DemoProject) (err error) {
			if _, err = open(q, d); err != nil {
				return err
			}
			k.target, err = q.TryWait(RoleLabel("list", listLabel), q.waitFor())
			return err
		},
		Reset: k.scrollToTop,
		Do:    k.wheel(stressListScroll),
	}
}

func stressSettingsActions() []latencyAction {
	settingsView := openPrefsView(settingsMenuItem, settingsSearchLabel)
	keysView := openPrefsView(keyboardMenuItem, keysSearchLabel)
	bindings := fmt.Sprintf("%d user bindings", len(stressUserBindings()))
	onSmall := func(a latencyAction, name string, samples int) latencyAction {
		return a.onProject(stressSmallProject, name, stressQuietShort, samples)
	}
	return []latencyAction{
		onSmall(filterPrefsAction(settingsView, stressSettingsQuery, settingsSearchLabel), "filter the Settings list", latencySamples),
		onSmall(scrollPrefsAction(settingsView, settingsListLabel), "scroll the Settings list", stressSamples),
		onSmall(filterPrefsAction(keysView, stressKeysQuery, keysSearchLabel), "filter the Keyboard Shortcuts list ("+bindings+")", latencySamples).withOptions(stressKeybindingsOptions),
		onSmall(scrollPrefsAction(keysView, keysListLabel), "scroll the Keyboard Shortcuts list ("+bindings+")", stressSamples).withOptions(stressKeybindingsOptions),
	}
}
