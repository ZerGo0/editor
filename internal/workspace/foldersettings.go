package workspace

import (
	"os"
	"path/filepath"
	"slices"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/fsutil"
)

const (
	folderSettingsDir  = ".editor"
	folderSettingsFile = "settings.json"
)

func (w *Workspace) folderSettingsPath() string {
	return filepath.Join(w.Root, folderSettingsDir, folderSettingsFile)
}

func (w *Workspace) OpenFolderSettings() {
	w.settingsView.ShowFolder()
	w.OpenFile(SettingsTab, false, false)
}

func (w *Workspace) OpenFolderSettingsJSON() {
	p := w.folderSettingsPath()
	if err := os.MkdirAll(filepath.Dir(p), dirPerm); err != nil {
		w.showError("Cannot create "+p, err)
		return
	}
	w.openUserFile(p, emptySettings)
}

func (w *Workspace) folderSettingsChanged(dirs []string) {
	p := w.folderSettingsPath()
	if !slices.ContainsFunc(dirs, func(d string) bool { return fsutil.Within(d, p) }) {
		return
	}
	old := w.folderStamp
	applog.Go(func() {
		stamp, _ := statFile(p)
		if stamp.same(old) {
			return
		}
		w.update(func() {
			w.folderStamp = stamp
			w.settings.Reload()
			w.SettingsChanged()
		})
	})
}
