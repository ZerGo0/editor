package app

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/shellcmd"
	"github.com/egoist/mygo"
)

func (a *App) installCommand(*mygo.MenuItem, *mygo.Window) {
	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
	}
	path := ""
	if err == nil {
		path, err = shellcmd.Install(exe)
	}
	a.refreshMenu()
	applog.Go(func() {
		if err != nil {
			mygo.Dialog.Error("Cannot install the '"+shellcmd.Name+"' command", err.Error())
			return
		}
		_, _ = mygo.Dialog.Message(mygo.MessageOptions{
			Type:    mygo.MessageInfo,
			Message: "The '" + shellcmd.Name + "' command is installed.",
			Detail:  fmt.Sprintf("It is at %s. Make sure that %s is in your PATH. Run '%s .' to open the current folder.", path, filepath.Dir(path), shellcmd.Name),
		})
	})
}

func (a *App) uninstallCommand(*mygo.MenuItem, *mygo.Window) {
	_, err := shellcmd.Uninstall()
	a.refreshMenu()
	if err != nil {
		go mygo.Dialog.Error("Cannot uninstall the '"+shellcmd.Name+"' command", err.Error())
	}
}
