package app

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	importViewTitle          = "VS Code Keybindings Import"
	importButton             = 0
	notNowButton             = 1
)

var errNoVSCodeKeybindings = errors.New("no VS Code keybindings.json was found in Code, Code - Insiders or VSCodium")

func (a *App) registerKeymapCommands() {
	a.keys.Settings = a.settings
	a.keys.Reload()
	inWorkspace := func(fn func(*workspace.Workspace)) func(*mygo.Window) {
		return func(win *mygo.Window) {
			if pw := a.workspaceFor(win); pw != nil {
				pw.win.Update(func() { fn(pw.ws) })
			}
		}
	}
	a.commands.Add(
		&command.Command{ID: importKeybindingsCommand, Category: categoryPreferences, Title: "Import VS Code Keybindings", Run: a.importVSCodeKeybindings},
		&command.Command{ID: replaceInFilesCommand, Category: categorySearch, Title: "Replace in Files", Run: inWorkspace((*workspace.Workspace).ReplaceInFiles)},
	)
	statePath := filepath.Join(userDataDir(), dataDir, vscodeimport.PromptStateFile)
	go a.offerVSCodeImport(statePath)
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
		pw.win.Update(func() { pw.ws.ShowError("Cannot import VS Code keybindings", err) })
		return
	}
	go mygo.Dialog.Error("Cannot import VS Code keybindings", err.Error())
}

func (a *App) offerVSCodeImport(statePath string) {
	if a.options.Hidden {
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	src, ok := vscodeimport.ShouldOffer(home, statePath, a.keys.Path)
	if !ok {
		return
	}
	res, err := mygo.Dialog.Message(mygo.MessageOptions{
		Type:          mygo.MessageQuestion,
		Message:       "Import your VS Code keybindings?",
		Detail:        "Copies the keybindings from " + src.Product + " into this editor's keybindings.json. You can also do this later with Preferences: Import VS Code Keybindings.",
		Buttons:       []string{"Import", "Not Now"},
		DefaultButton: importButton,
		CancelButton:  notNowButton,
	})
	if err != nil {
		log.Printf("import prompt: %v", err)
		return
	}
	choice := vscodeimport.ChoiceNotNow
	if res.Button == importButton {
		choice = vscodeimport.ChoiceImport
	}
	if err := vscodeimport.Remember(statePath, choice); err != nil {
		log.Printf("import prompt: %v", err)
	}
	if choice == vscodeimport.ChoiceImport {
		mygo.RunOnMain(func() { a.runImport(src, a.anyProject()) })
	}
}

func (a *App) anyProject() *projectWindow {
	for _, pw := range a.projects {
		return pw
	}
	return nil
}

func (a *App) runImport(src vscodeimport.Source, pw *projectWindow) {
	known := map[string]bool{}
	for _, c := range a.commands.All() {
		known[c.ID] = true
	}
	dst := a.keys.Path
	summary := &vscodeimport.Summary{Loading: true}
	var start func()
	start = func() {
		summary.Loading, summary.Err = true, nil
		go func() {
			r, err := vscodeimport.Import(src, dst, func(id string) bool { return known[id] }, time.Now())
			mygo.RunOnMain(func() { a.importDone(pw, summary, r, err) })
		}()
	}
	summary.Retry = start
	if pw != nil {
		summary.OpenKeybindings = pw.ws.OpenKeybindingsJSON
		pw.win.Update(func() { pw.ws.OpenView(importViewID, importViewTitle, summary.View) })
	}
	start()
}

func (a *App) importDone(pw *projectWindow, summary *vscodeimport.Summary, r vscodeimport.Report, err error) {
	if err == nil && r.DispatchFound {
		if setErr := a.settings.Set(keymap.SettingDispatch, string(r.Dispatch)); setErr != nil {
			r.DispatchError = setErr
		}
	}
	a.keys.Reload()
	a.settingsChanged()
	if pw != nil {
		pw.win.Update(func() { summary.Loading, summary.Report, summary.Err = false, r, err })
		return
	}
	if err != nil {
		go mygo.Dialog.Error("Cannot import VS Code keybindings", err.Error())
		return
	}
	go func() {
		_, _ = mygo.Dialog.Message(mygo.MessageOptions{Type: mygo.MessageInfo, Message: r.Headline(), Detail: strings.Join(r.Details(), "\n")})
	}()
}
