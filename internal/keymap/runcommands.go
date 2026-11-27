package keymap

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ZerGo0/editor/internal/command"
)

const RunCommands = "runCommands"

var errRunCommandsArgs = errors.New(RunCommands + " needs the args {\"commands\": [...]}")

type RunItem struct {
	ID       string
	Resolved string
	Err      error
}

func RunCommandItems(args json.RawMessage) ([]RunItem, error) {
	var a struct {
		Commands []json.RawMessage `json:"commands"`
	}
	if err := json.Unmarshal(args, &a); err != nil || a.Commands == nil {
		return nil, errRunCommandsArgs
	}
	items := make([]RunItem, len(a.Commands))
	for i, raw := range a.Commands {
		id, itemArgs, err := runItemOf(raw)
		items[i] = RunItem{ID: id, Err: err}
		if err == nil {
			items[i].Resolved, items[i].Err = ResolveCommand(id, itemArgs)
		}
	}
	return items, nil
}

func runItemOf(raw json.RawMessage) (string, json.RawMessage, error) {
	var id string
	if err := json.Unmarshal(raw, &id); err == nil {
		return id, nil, nil
	}
	var item struct {
		Command string          `json:"command"`
		Args    json.RawMessage `json:"args"`
	}
	if err := json.Unmarshal(raw, &item); err != nil || item.Command == "" {
		return string(bytes.TrimSpace(raw)), nil, fmt.Errorf("%s items must be a command ID or {\"command\": ID}", RunCommands)
	}
	return item.Command, item.Args, nil
}

func runCommandsWithArgs(args json.RawMessage) (string, error) {
	items, err := RunCommandItems(args)
	if err != nil {
		return "", err
	}
	ids := make([]string, 0, len(items))
	for _, it := range items {
		if it.Err == nil {
			ids = append(ids, it.Resolved)
		}
	}
	return command.SequenceID(ids), nil
}
