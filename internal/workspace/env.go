package workspace

import (
	"github.com/ZerGo0/editor/internal/command"
	"github.com/ZerGo0/editor/internal/history"
	"github.com/ZerGo0/editor/internal/keymap"
	"github.com/ZerGo0/editor/internal/settings"
	"github.com/egoist/mygo"
)

type Env struct {
	Settings      *settings.Store
	Keys          *keymap.Store
	Commands      *command.Registry
	UserData      string
	BackupDir     string
	History       *history.Store
	RecentFolders func() []string
	OpenFolders   func() []string
	OpenFolder    func(from *mygo.Window, root string, replace bool)
	ForgetFolder  func(root string)
}
