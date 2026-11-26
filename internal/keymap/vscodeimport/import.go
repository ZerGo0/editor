package vscodeimport

import (
	"bytes"
	"fmt"
	"os"
	"time"

	"github.com/ZerGo0/editor/internal/jsonc"
	"github.com/ZerGo0/editor/internal/keymap"
)

const emptyArray = "[]"

type Report struct {
	Source        Source
	Total         int
	Plan          Plan
	Backup        string
	Dispatch      keymap.Dispatch
	DispatchFound bool
	DispatchError error
}

func (r Report) Headline() string {
	return fmt.Sprintf("Imported %d of %d keybindings from %s. Skipped %d.", len(r.Plan.Add), r.Total, r.Source.Product, len(r.Plan.Skipped))
}

func (r Report) Details() []string {
	var out []string
	if r.Plan.Duplicates > 0 {
		out = append(out, fmt.Sprintf("%d were already in your keybindings.json.", r.Plan.Duplicates))
	}
	if r.Backup != "" {
		out = append(out, "Backup of your old keybindings.json: "+r.Backup)
	}
	switch {
	case r.DispatchError != nil:
		out = append(out, "Cannot read keyboard.dispatch: "+r.DispatchError.Error())
	case r.DispatchFound:
		out = append(out, fmt.Sprintf("Set %s to %q.", keymap.SettingDispatch, r.Dispatch))
	}
	return out
}

func Import(src Source, dst string, known func(id string) bool, now time.Time) (Report, error) {
	r := Report{Source: src}
	theirs, err := readEntries(src.Keybindings())
	if err != nil {
		return r, err
	}
	r.Total = len(theirs)
	data, err := os.ReadFile(dst)
	if err != nil && !os.IsNotExist(err) {
		return r, fmt.Errorf("read %s: %w", dst, err)
	}
	ours, err := keymap.ParseEntries(orEmptyArray(data), dst)
	if err != nil {
		return r, fmt.Errorf("fix %s before you import: %w", dst, err)
	}
	r.Plan = Convert(theirs, ours, known)
	r.Dispatch, r.DispatchFound, r.DispatchError = ReadDispatch(src.Settings())
	if len(r.Plan.Add) == 0 {
		return r, nil
	}
	r.Backup, err = Merge(dst, data, r.Plan.Add, now)
	return r, err
}

func readEntries(path string) ([]keymap.Entry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return keymap.ParseEntries(orEmptyArray(data), path)
}

func orEmptyArray(data []byte) []byte {
	if len(bytes.TrimSpace(jsonc.Standard(data))) == 0 {
		return []byte(emptyArray)
	}
	return data
}
