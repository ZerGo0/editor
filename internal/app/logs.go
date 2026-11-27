package app

import (
	"log"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/egoist/mygo"
)

func openLogsFolder() {
	dir, err := mygo.App.Path(mygo.PathLogs)
	if err != nil {
		log.Printf("logs folder: %v", err)
		return
	}
	mygo.Shell.ShowItemInFolder(applog.Path(dir))
}
