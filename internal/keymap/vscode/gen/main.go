package main

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"os"
)

//go:embed commands.txt
var commandsText string

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	dumps, err := download(context.Background())
	if err != nil {
		return err
	}
	supported, err := parseCommands(commandsText)
	if err != nil {
		return err
	}
	t, err := build(dumps, supported)
	if err != nil {
		return err
	}
	src, err := render(t)
	if err != nil {
		return err
	}
	if err := os.WriteFile(tableFile, src, filePerm); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "wrote VS Code %s keybindings: %d macOS, %d Windows\n", t.version, len(t.platforms[0].entries), len(t.platforms[1].entries))
	return nil
}
