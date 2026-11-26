package vscodeimport

import (
	"encoding/json"
	"fmt"

	"github.com/ZerGo0/editor/internal/jsonc"
	"github.com/ZerGo0/editor/internal/keymap"
)

func ReadDispatch(path string) (keymap.Dispatch, bool, error) {
	data, err := jsonc.ReadFile(path)
	if err != nil || data == nil {
		return "", false, err
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(data, &values); err != nil {
		return "", false, fmt.Errorf("%s: %w", path, err)
	}
	raw, ok := values[keymap.SettingDispatch]
	if !ok {
		return "", false, nil
	}
	var d keymap.Dispatch
	if err := json.Unmarshal(raw, &d); err != nil {
		return "", false, fmt.Errorf("%s: %s: %w", path, keymap.SettingDispatch, err)
	}
	switch d {
	case keymap.DispatchCode, keymap.DispatchKeyCode:
		return d, true, nil
	}
	return "", false, fmt.Errorf("%s: %s has the unknown value %q", path, keymap.SettingDispatch, d)
}
