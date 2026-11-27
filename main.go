package main

import (
	"log"
	"os"

	"github.com/ZerGo0/editor/internal/app"
	"github.com/egoist/mygo"
)

func main() {
	applyProfile()
	options := e2eOptions()
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}
	if err := app.New(options).Run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}
