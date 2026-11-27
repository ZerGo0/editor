package main

import (
	"log"
	"os"
	"runtime"

	"github.com/ZerGo0/editor/internal/app"
	"github.com/ZerGo0/editor/internal/applog"
	"github.com/egoist/mygo"
)

func main() {
	applyProfile()
	options := e2eOptions()
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}
	setupLogs()
	if err := app.New(options).Run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func setupLogs() {
	dir, err := mygo.App.Path(mygo.PathLogs)
	if err == nil {
		err = applog.Setup(dir)
	}
	if err != nil {
		log.Printf("logs: %v", err)
	}
	log.Printf("start %s %s on %s/%s", mygo.App.Name(), mygo.App.Version(), runtime.GOOS, runtime.GOARCH)
}
