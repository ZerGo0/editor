package prefs

import (
	"strings"

	"github.com/ZerGo0/editor/internal/settings"
)

const (
	filterModified  = "@modified"
	filterID        = "@id:"
	filterSource    = "@source:"
	sourceUserID    = "user"
	sourceDefaultID = "default"
)

type settingFilter struct {
	words    []string
	id       string
	modified bool
}

func parseSettingFilter(query string) settingFilter {
	var f settingFilter
	for _, w := range strings.Fields(strings.ToLower(query)) {
		switch {
		case w == filterModified:
			f.modified = true
		case strings.HasPrefix(w, filterID):
			f.id = strings.TrimPrefix(w, filterID)
		default:
			f.words = append(f.words, w)
		}
	}
	return f
}

func (f settingFilter) active() bool {
	return len(f.words) > 0 || f.id != "" || f.modified
}

func (f settingFilter) match(d *settings.Def, modified bool) bool {
	if f.modified && !modified {
		return false
	}
	if f.id != "" && !strings.HasPrefix(strings.ToLower(d.Key), f.id) {
		return false
	}
	return containsAll(d.Category+" "+d.Title+" "+d.Key+" "+d.Description, f.words)
}

type keyFilter struct {
	words  []string
	source string
}

func parseKeyFilter(query string) keyFilter {
	var f keyFilter
	for _, w := range strings.Fields(strings.ToLower(query)) {
		if strings.HasPrefix(w, filterSource) {
			f.source = strings.TrimPrefix(w, filterSource)
			continue
		}
		f.words = append(f.words, w)
	}
	return f
}

func (f keyFilter) match(text string, bound, user bool) bool {
	switch f.source {
	case "":
	case sourceUserID:
		if !user {
			return false
		}
	case sourceDefaultID:
		if !bound || user {
			return false
		}
	}
	return containsAll(text, f.words)
}

func containsAll(text string, words []string) bool {
	text = strings.ToLower(text)
	for _, w := range words {
		if !strings.Contains(text, w) {
			return false
		}
	}
	return true
}
