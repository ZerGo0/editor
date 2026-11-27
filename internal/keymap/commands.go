package keymap

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/ZerGo0/editor/internal/keymap/vscode"
	"github.com/egoist/mygo/ui/editor"
)

const (
	legacyEditorPrefix = "editor."
	quickOpenCommand   = "workbench.action.quickOpen"
	cursorMoveCommand  = "cursorMove"
	moveByLine         = "line"
	moveByWrappedLine  = "wrappedLine"
	moveByCharacter    = "character"
	moveByDefault      = ""
	singleStep         = 1
	unknownCommandName = "unknown"
)

var jsonNull = []byte("null")

var quickOpenPrefixes = map[string]string{
	"":  quickOpenCommand,
	">": "workbench.action.showCommands",
	":": "workbench.action.gotoLine",
	"@": "workbench.action.gotoSymbol",
	"#": "workbench.action.showAllSymbols",
}

var argsIgnored = map[string]bool{
	"cursorHome":       true,
	"cursorEnd":        true,
	"cursorHomeSelect": true,
	"cursorEndSelect":  true,
}

type cursorTarget struct {
	move, sel editor.Command
	stepBy    []string
}

var cursorMoves = map[string]cursorTarget{
	"left":          {editor.MoveLeft, editor.SelectLeft, []string{moveByDefault, moveByCharacter}},
	"right":         {editor.MoveRight, editor.SelectRight, []string{moveByDefault, moveByCharacter}},
	"up":            {editor.MoveUp, editor.SelectUp, []string{moveByDefault, moveByLine, moveByWrappedLine}},
	"down":          {editor.MoveDown, editor.SelectDown, []string{moveByDefault, moveByLine, moveByWrappedLine}},
	"prevBlankLine": {editor.MoveParagraphUp, editor.SelectParagraphUp, []string{moveByDefault, moveByWrappedLine}},
	"nextBlankLine": {editor.MoveParagraphDown, editor.SelectParagraphDown, []string{moveByDefault, moveByWrappedLine}},
}

var (
	editorIDs, editorByID = buildEditorTables()
	aliases               = buildAliases()
)

func buildEditorTables() (map[editor.Command]string, map[string]editor.Command) {
	ids := map[editor.Command]string{}
	for _, c := range vscode.EditorCommands {
		ids[c.Command] = c.ID
	}
	byID := map[string]editor.Command{}
	for cmd := editor.NoCommand + 1; cmd.String() != unknownCommandName; cmd++ {
		if cmd == editor.ToggleSoftWrap {
			continue
		}
		id, ok := ids[cmd]
		if !ok {
			id = legacyEditorPrefix + cmd.String()
		}
		byID[id] = cmd
	}
	return ids, byID
}

func buildAliases() map[string]string {
	out := map[string]string{}
	for cmd := editor.NoCommand + 1; cmd.String() != unknownCommandName; cmd++ {
		if legacy := legacyEditorPrefix + cmd.String(); legacy != EditorCommandID(cmd) {
			out[legacy] = EditorCommandID(cmd)
		}
	}
	for id, cmd := range vscode.EditorAliases {
		out[id] = EditorCommandID(cmd)
	}
	for id, to := range vscode.WorkbenchAliases {
		out[id] = to
	}
	return out
}

func EditorCommandID(cmd editor.Command) string {
	if id, ok := editorIDs[cmd]; ok {
		return id
	}
	return legacyEditorPrefix + cmd.String()
}

func EditorCommand(id string) (editor.Command, bool) {
	cmd, ok := editorByID[CanonicalID(id)]
	return cmd, ok
}

func CanonicalID(id string) string {
	if to, ok := aliases[id]; ok {
		return to
	}
	return id
}

func ResolveCommand(id string, args json.RawMessage) (string, error) {
	canonical := CanonicalID(id)
	if len(bytes.TrimSpace(args)) == 0 || bytes.Equal(bytes.TrimSpace(args), jsonNull) {
		return canonical, nil
	}
	switch {
	case canonical == quickOpenCommand:
		return quickOpenWithArgs(id, args)
	case canonical == RunCommands:
		return runCommandsWithArgs(args)
	case canonical == cursorMoveCommand:
		return cursorMoveWithArgs(args)
	case argsIgnored[canonical]:
		return canonical, nil
	}
	return "", fmt.Errorf("%s with args is not supported", id)
}

func quickOpenWithArgs(id string, args json.RawMessage) (string, error) {
	var prefix string
	if err := json.Unmarshal(args, &prefix); err != nil {
		return "", fmt.Errorf("%s: args must be a string: %w", id, err)
	}
	to, ok := quickOpenPrefixes[prefix]
	if !ok {
		return "", fmt.Errorf("%s with the prefix %q is not supported", id, prefix)
	}
	return to, nil
}

func cursorMoveWithArgs(args json.RawMessage) (string, error) {
	var a struct {
		To     string `json:"to"`
		By     string `json:"by"`
		Value  *int   `json:"value"`
		Select bool   `json:"select"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("%s: %w", cursorMoveCommand, err)
	}
	target, ok := cursorMoves[a.To]
	if !ok {
		return "", fmt.Errorf("%s to %q is not supported", cursorMoveCommand, a.To)
	}
	if a.Value != nil && *a.Value != singleStep {
		return "", fmt.Errorf("%s by %d steps is not supported", cursorMoveCommand, *a.Value)
	}
	if !slices.Contains(target.stepBy, a.By) {
		return "", fmt.Errorf("%s to %q by %q is not supported", cursorMoveCommand, a.To, a.By)
	}
	if a.Select {
		return EditorCommandID(target.sel), nil
	}
	return EditorCommandID(target.move), nil
}
