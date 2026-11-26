package keymap

import "github.com/ZerGo0/editor/internal/settings"

const (
	SettingKeymap   = "keyboard.keymap"
	SettingDispatch = "keyboard.dispatch"

	PresetVSCode        = "vscode"
	PresetVSCodeWindows = "vscode-windows"
)

type Dispatch string

const (
	DispatchCode    Dispatch = "code"
	DispatchKeyCode Dispatch = "keyCode"
)

var settingDefs = []settings.Def{
	{
		Key:         SettingKeymap,
		Title:       "Keymap",
		Description: "The default keyboard shortcuts: VS Code's macOS keys (vscode), or VS Code's Windows and Linux keys with Ctrl (vscode-windows). keybindings.json applies on top.",
		Category:    settings.CategoryWorkbench,
		Kind:        settings.Enum,
		Default:     PresetVSCode,
		Options:     []string{PresetVSCode, PresetVSCodeWindows},
		AppWide:     true,
	},
	{
		Key:         SettingDispatch,
		Title:       "Keyboard Dispatch",
		Description: "How a key press is matched to a keybinding while you record one: by the character the layout types, with the key's position for non-Latin layouts (code), or always by the key's position on a US keyboard (keyCode).",
		Category:    settings.CategoryWorkbench,
		Kind:        settings.Enum,
		Default:     string(DispatchCode),
		Options:     []string{string(DispatchCode), string(DispatchKeyCode)},
		AppWide:     true,
	},
}

func init() {
	// TECHDEBT: Adds the keyboard settings to settings.Defs at start-up because this change may not edit internal/settings; move settingDefs into settings/schema.go.
	settings.Defs = append(settings.Defs, settingDefs...)
}
