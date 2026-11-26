package prefs

import (
	"testing"

	"github.com/ZerGo0/editor/internal/settings"
)

func TestSettingFilterMatchesWordsAndIDs(t *testing.T) {
	d := &settings.Def{Key: "editor.wordWrap", Title: "Word Wrap", Category: "Editor", Description: "Wraps long lines."}
	cases := []struct {
		query    string
		modified bool
		want     bool
	}{
		{"wrap word", false, true},
		{"editor.wordwrap", false, true},
		{"long lines", false, true},
		{"wrap folding", false, false},
		{"@id:editor.word", false, true},
		{"@id:files.", false, false},
		{"@modified", false, false},
		{"@modified wrap", true, true},
	}
	for _, tc := range cases {
		if got := parseSettingFilter(tc.query).match(d, tc.modified); got != tc.want {
			t.Errorf("%q modified %v = %v, want %v", tc.query, tc.modified, got, tc.want)
		}
	}
	if parseSettingFilter("  ").active() {
		t.Error("a blank query is an active filter")
	}
}

func TestKeyFilterSource(t *testing.T) {
	cases := []struct {
		query       string
		bound, user bool
		want        bool
	}{
		{"save", true, false, true},
		{"@source:user save", true, false, false},
		{"@source:user save", true, true, true},
		{"@source:default", true, false, true},
		{"@source:default", false, false, false},
		{"@source:default", true, true, false},
	}
	for _, tc := range cases {
		if got := parseKeyFilter(tc.query).match("File: Save workbench.action.files.save", tc.bound, tc.user); got != tc.want {
			t.Errorf("%q bound %v user %v = %v, want %v", tc.query, tc.bound, tc.user, got, tc.want)
		}
	}
}
