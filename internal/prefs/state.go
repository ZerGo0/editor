package prefs

import "github.com/ZerGo0/editor/internal/settings"

type SettingsState struct {
	Scope    int    `json:"scope,omitempty"`
	Category int    `json:"category,omitempty"`
	Query    string `json:"query,omitempty"`
	Modified bool   `json:"modified,omitempty"`
}

func (v *SettingsView) State() SettingsState {
	return SettingsState{Scope: v.scope, Category: v.category, Query: v.query, Modified: v.modified}
}

func (v *SettingsView) Restore(s SettingsState) {
	v.scope = scopeUser
	if s.Scope == scopeFolder && v.Folder != nil {
		v.scope = scopeFolder
	}
	v.category = min(max(s.Category, 0), len(settings.Categories)-1)
	v.query, v.modified = s.Query, s.Modified
	v.drafts = nil
}

type KeysState struct {
	Query string `json:"query,omitempty"`
}

func (v *KeysView) State() KeysState {
	return KeysState{Query: v.query}
}

func (v *KeysView) Restore(s KeysState) {
	v.query = s.Query
}
