package app

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"log"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/command"
	"github.com/ZerGo0/editor/internal/git"
	"github.com/ZerGo0/editor/internal/git/diffmode"
	"github.com/ZerGo0/editor/internal/history"
	"github.com/ZerGo0/editor/internal/keymap"
	"github.com/ZerGo0/editor/internal/session"
	"github.com/ZerGo0/editor/internal/settings"
	"github.com/ZerGo0/editor/internal/workspace"
	"github.com/egoist/mygo"
)

const (
	settingsFile    = "settings.json"
	keybindingsFile = "keybindings.json"
	dataDir         = "data"
	historyDir      = "history"
	backupsDir      = "backups"
)

func userDataDir() string {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		log.Printf("user data dir: %v", err)
		return "."
	}
	return dir
}

func (a *App) loadConfig() {
	dir := userDataDir()
	a.settings = settings.Load(filepath.Join(dir, settingsFile))
	a.keys = keymap.Load(filepath.Join(dir, keybindingsFile))
	a.commands = command.NewRegistry()
	a.commands.SetRecent(a.store.State.Commands)
	a.env = &workspace.Env{
		Settings:      a.settings,
		Keys:          a.keys,
		Commands:      a.commands,
		UserData:      dir,
		RecentFolders: func() []string { return a.store.State.Recent },
		OpenFolders:   func() []string { return slices.Collect(maps.Keys(a.projects)) },
		RecentFiles:   func() []string { return slices.Clone(a.store.State.Files) },
		TouchFile:     a.store.AddRecentFile,
		ForgetFile:    a.store.RemoveRecentFile,
		OpenFolder:    a.openFolder,
		ForgetFolder:  a.removeRecent,
		PickFolder:    func(parent *mygo.Window, title string) ([]string, error) { return a.pickPaths(parent, title, true) },
		Global:        session.NewMemento(a.store.State.Views),
	}
	if !a.options.NoBackups {
		a.env.BackupDir = filepath.Join(dir, dataDir, backupsDir)
	}
	a.registerCommands()
	a.applyDiffSettings()
	applog.Go(func() { a.watchConfig(context.Background(), dir) })
	applog.Go(func() { a.openHistory(dir) })
}

func (a *App) watchConfig(ctx context.Context, dir string) {
	watcher, err := mygo.WatchFiles(ctx, dir, mygo.FileWatchOptions{HintsOnly: true})
	if err != nil {
		log.Printf("watch %s: %v", dir, err)
		return
	}
	defer watcher.Close()
	last := readConfig(dir)
	for {
		events, err := watcher.Next(ctx)
		if err != nil {
			return
		}
		if !touchesConfig(events) {
			continue
		}
		cur := readConfig(dir)
		if slices.EqualFunc(cur, last, bytes.Equal) {
			continue
		}
		last = cur
		mygo.RunOnMain(func() {
			a.settings.Reload()
			a.keys.Reload()
			a.settingsChanged()
		})
	}
}

func (a *App) openHistory(dir string) {
	store, err := history.Open(filepath.Join(dir, dataDir, historyDir), history.Limits{})
	if err != nil {
		log.Printf("local history: %v", err)
		return
	}
	mygo.RunOnMain(func() { a.env.History = store })
}

func readConfig(dir string) [][]byte {
	out := make([][]byte, 0, 2)
	for _, name := range []string{settingsFile, keybindingsFile} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			data = []byte(err.Error())
		}
		out = append(out, data)
	}
	return out
}

func touchesConfig(events []mygo.FileEvent) bool {
	for _, e := range events {
		switch e.Path {
		case "", ".", settingsFile, keybindingsFile:
			return true
		}
	}
	return false
}

func (a *App) applyDiffSettings() {
	mode, _ := git.ParseWhitespaceMode(a.settings.String(settings.DiffIgnoreWhitespace))
	diffmode.Session.Whitespace = mode
	diffmode.Session.ShowMoves = a.settings.Bool(settings.DiffShowMoves)
}

func (a *App) settingsChanged() {
	a.applyDiffSettings()
	a.refreshMenu()
	for _, pw := range a.projects {
		pw.ws.Update(pw.ws.SettingsChanged)
	}
}
