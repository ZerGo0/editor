package app

import (
	"errors"
	"os"
	"strings"
	"time"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/command"
	"github.com/ZerGo0/editor/internal/keymap"
	"github.com/ZerGo0/editor/internal/keymap/vscodeimport"
	"github.com/ZerGo0/editor/internal/workspace"
	"github.com/egoist/mygo"
)

const (
	importKeybindingsCommand = "workbench.action.importVSCodeKeybindings"
	replaceInFilesCommand    = "workbench.action.replaceInFiles"
	importViewID             = "vscode-keybindings-import"
	importErrorTitle         = "Cannot import VS Code keybindings"
	showDetails              = "Show Details"
)

var errNoVSCodeKeybindings = errors.New("no VS Code keybindings.json was found in Code, Code - Insiders or VSCodium")

func (a *App) registerKeymapCommands() {
	a.keys.Settings = a.settings
	a.keys.Reload()
	a.commands.Add(
		&command.Command{ID: importKeybindingsCommand, Category: categoryPreferences, Title: "Import VS Code Keybindings...", Run: a.importVSCodeKeybindings},
		&command.Command{ID: replaceInFilesCommand, Category: categorySearch, Title: "Replace in Files", Run: a.inWorkspace((*workspace.Workspace).ReplaceInFiles)},
	)
}

func (a *App) importVSCodeKeybindings(win *mygo.Window) {
	pw := a.workspaceFor(win)
	home, err := os.UserHomeDir()
	if err != nil {
		a.showImportError(pw, err)
		return
	}
	src, ok := vscodeimport.Find(home)
	if !ok {
		a.showImportError(pw, errNoVSCodeKeybindings)
		return
	}
	a.runImport(src, pw)
}

func (a *App) showImportError(pw *projectWindow, err error) {
	if pw != nil {
		pw.ws.Update(func() { pw.ws.ShowError(importErrorTitle, err) })
		return
	}
	go mygo.Dialog.Error(importErrorTitle, err.Error())
}

func (a *App) runImport(src vscodeimport.Source, pw *projectWindow) {
	known := map[string]bool{}
	for _, c := range a.commands.All() {
		known[c.ID] = true
	}
	dst := a.keys.Path
	applog.Go(func() {
		r, err := vscodeimport.Import(src, dst, func(id string) bool { return known[id] }, time.Now())
		mygo.RunOnMain(func() { a.importDone(pw, r, err) })
	})
}

func (a *App) importDone(pw *projectWindow, r vscodeimport.Report, err error) {
	if err == nil && r.DispatchFound {
		if setErr := a.settings.Set(keymap.SettingDispatch, string(r.Dispatch)); setErr != nil {
			r.DispatchError = setErr
		}
	}
	a.keys.Reload()
	a.settingsChanged()
	switch {
	case pw != nil && err != nil:
		pw.ws.Update(func() { pw.ws.ShowError(importErrorTitle, err) })
	case pw != nil:
		summary := &vscodeimport.Summary{Report: r, OpenKeybindings: pw.ws.OpenKeybindingsJSON}
		show := func() { pw.ws.OpenView(importViewID, vscodeimport.ViewTitle, summary.View) }
		pw.ws.Update(func() {
			pw.ws.NotifyAction(r.Headline(), strings.Join(r.Details(), "\n"), showDetails, show)
		})
	case err != nil:
		go mygo.Dialog.Error(importErrorTitle, err.Error())
	default:
		applog.Go(func() {
			_, _ = mygo.Dialog.Message(mygo.MessageOptions{Type: mygo.MessageInfo, Message: r.Headline(), Detail: strings.Join(r.Details(), "\n")})
		})
	}
}
