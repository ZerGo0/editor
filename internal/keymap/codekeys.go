package keymap

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

const firstExtraFunctionKey = 13

var (
	codeInfos   = buildCodeInfos()
	codesByName = buildCodesByName()
)

func buildCodeInfos() map[ui.KeyCode]keyInfo {
	numpadDigits := [digitCount]ui.KeyCode{
		ui.CodeNumpad0, ui.CodeNumpad1, ui.CodeNumpad2, ui.CodeNumpad3, ui.CodeNumpad4,
		ui.CodeNumpad5, ui.CodeNumpad6, ui.CodeNumpad7, ui.CodeNumpad8, ui.CodeNumpad9,
	}
	extraFunctions := []ui.KeyCode{
		ui.CodeF13, ui.CodeF14, ui.CodeF15, ui.CodeF16, ui.CodeF17, ui.CodeF18,
		ui.CodeF19, ui.CodeF20, ui.CodeF21, ui.CodeF22, ui.CodeF23, ui.CodeF24,
	}
	infos := map[ui.KeyCode]keyInfo{
		ui.CodeNumpadMultiply: {"numpad_multiply", "Num*", ""},
		ui.CodeNumpadAdd:      {"numpad_add", "Num+", ""},
		ui.CodeNumpadComma:    {"numpad_separator", "Num,", ""},
		ui.CodeNumpadSubtract: {"numpad_subtract", "Num-", ""},
		ui.CodeNumpadDecimal:  {"numpad_decimal", "Num.", ""},
		ui.CodeNumpadDivide:   {"numpad_divide", "Num/", ""},
		ui.CodeNumLock:        {"numlock", "NumLock", ""},
		ui.CodePause:          {"pausebreak", "Pause", ""},
		ui.CodeScrollLock:     {"scrolllock", "ScrollLock", ""},
		ui.CodePrintScreen:    {"printscreen", "PrintScreen", ""},
	}
	for i, code := range numpadDigits {
		infos[code] = keyInfo{fmt.Sprintf("numpad%d", i), fmt.Sprintf("Num%d", i), ""}
	}
	for i, code := range extraFunctions {
		n := i + firstExtraFunctionKey
		infos[code] = keyInfo{fmt.Sprintf("f%d", n), fmt.Sprintf("F%d", n), ""}
	}
	return infos
}

func buildCodesByName() map[string]ui.KeyCode {
	byName := make(map[string]ui.KeyCode, len(codeInfos))
	for code, info := range codeInfos {
		byName[info.name] = code
	}
	return byName
}
