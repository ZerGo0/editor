package workspace

import "github.com/ZerGo0/editor/internal/prefs"

const (
	settingsStateKey = "prefs.settings"
	keysStateKey     = "prefs.keys"
)

func (w *Workspace) trackPrefsState() {
	state := w.ProjectState()
	var s prefs.SettingsState
	if state.Load(settingsStateKey, &s) {
		w.settingsView.Restore(s)
	}
	var k prefs.KeysState
	if state.Load(keysStateKey, &k) {
		w.keysView.Restore(k)
	}
	state.Track(settingsStateKey, func() any { return w.settingsView.State() })
	state.Track(keysStateKey, func() any { return w.keysView.State() })
}
