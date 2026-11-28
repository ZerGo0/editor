package prefs

import (
	"path/filepath"
	"testing"

	"github.com/ZerGo0/editor/internal/command"
	"github.com/ZerGo0/editor/internal/keymap"
	"github.com/ZerGo0/editor/internal/session"
	"github.com/ZerGo0/editor/internal/settings"
)

const (
	stateTestKey       = "prefs.test"
	savedCategory      = 2
	savedSettingsQuery = "@modified font"
	savedKeysQuery     = "save"
)

func TestSettingsStateRoundTripsThroughMemento(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	newView := func() *SettingsView {
		user := settings.Load(filepath.Join(dir, "settings.json"))
		return &SettingsView{Store: user, Folder: settings.LoadFolder(filepath.Join(dir, "folder.json"), user)}
	}
	v := newView()
	v.scope, v.category, v.query, v.modified = scopeFolder, savedCategory, savedSettingsQuery, true
	m := session.NewMemento(nil)
	m.Track(stateTestKey, func() any { return v.State() })
	var got SettingsState
	if !session.NewMemento(m.Snapshot()).Load(stateTestKey, &got) {
		t.Fatal("no saved settings state")
	}
	restored := newView()
	restored.Restore(got)
	if restored.State() != v.State() {
		t.Fatalf("restored %+v, want %+v", restored.State(), v.State())
	}
}

func TestSettingsRestoreClampsBadValues(t *testing.T) {
	t.Parallel()
	v := &SettingsView{Store: settings.Load(filepath.Join(t.TempDir(), "settings.json"))}
	v.Restore(SettingsState{Scope: scopeFolder, Category: len(settings.Categories) + 1})
	if v.scope != scopeUser || v.category != len(settings.Categories)-1 {
		t.Fatalf("scope %d, category %d", v.scope, v.category)
	}
}

func TestKeysStateRoundTripsThroughMemento(t *testing.T) {
	t.Parallel()
	newView := func() *KeysView {
		return &KeysView{Keys: keymap.Load(filepath.Join(t.TempDir(), "keybindings.json")), Commands: command.NewRegistry()}
	}
	v := newView()
	v.query = savedKeysQuery
	m := session.NewMemento(nil)
	m.Track(stateTestKey, func() any { return v.State() })
	var got KeysState
	if !session.NewMemento(m.Snapshot()).Load(stateTestKey, &got) {
		t.Fatal("no saved keys state")
	}
	restored := newView()
	restored.Restore(got)
	if restored.query != savedKeysQuery {
		t.Fatalf("query %q, want %q", restored.query, savedKeysQuery)
	}
}
