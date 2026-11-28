package workspace

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ZerGo0/editor/internal/jsonc"
)

const (
	settingsErrorDetail = "Some settings are not applied. Fix the file to apply them."
	keysErrorDetail     = "Some keyboard shortcuts are not applied. Fix the file to apply them."
	openSettingsJSON    = "Open Settings (JSON)"
	openKeybindingsJSON = "Open Keyboard Shortcuts (JSON)"
)

type configFile struct {
	path, detail, action string
	errs                 []jsonc.Problem
	open                 func()
}

func (w *Workspace) configFiles() []configFile {
	return []configFile{
		{w.env.Settings.Path, settingsErrorDetail, openSettingsJSON, w.env.Settings.Errors, w.OpenSettingsJSON},
		{w.settings.Path, settingsErrorDetail, openSettingsJSON, w.settings.Errors, w.OpenFolderSettingsJSON},
		{w.env.Keys.Path, keysErrorDetail, openKeybindingsJSON, w.env.Keys.Current.Errors, w.OpenKeybindingsJSON},
	}
}

func (w *Workspace) noticeConfigErrors() {
	for _, f := range w.configFiles() {
		sig := errorSignature(f.errs)
		if sig == w.insights.configShown[f.path] {
			continue
		}
		if sig == "" {
			delete(w.insights.configShown, f.path)
			continue
		}
		w.insights.configShown[f.path] = sig
		w.NotifyAction(filepath.Base(f.path)+" has errors", f.detail, f.action, f.open)
	}
}

func errorSignature(errs []jsonc.Problem) string {
	var b strings.Builder
	for _, e := range errs {
		fmt.Fprintf(&b, "%s:%d:%d;", e.Text, e.Line, e.Col)
	}
	return b.String()
}
