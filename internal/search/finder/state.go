package finder

import (
	"strings"

	"github.com/ZerGo0/editor/internal/design/picker"
)

type Tab int

const (
	TabAll Tab = iota
	TabActions
	TabFiles
	TabText
	tabCount
)

var tabTitles = [tabCount]string{"All", "Actions", "Files", "Text"}

func (t Tab) String() string {
	if t < 0 || t >= tabCount {
		return tabTitles[TabAll]
	}
	return tabTitles[t]
}

func parseTab(name string) Tab {
	for i, n := range tabTitles {
		if strings.EqualFold(n, name) {
			return Tab(i)
		}
	}
	return TabAll
}

const (
	ScopeProject   = "Project"
	ScopeDirectory = "Directory"
)

type State struct {
	Tab          string        `json:"tab,omitempty"`
	Query        string        `json:"query,omitempty"`
	MatchCase    bool          `json:"matchCase,omitempty"`
	WholeWord    bool          `json:"wholeWord,omitempty"`
	Regex        bool          `json:"regex,omitempty"`
	Preview      bool          `json:"preview,omitempty"`
	FileTypes    []string      `json:"fileTypes,omitempty"`
	NonProject   bool          `json:"nonProject,omitempty"`
	With         string        `json:"with,omitempty"`
	PreserveCase bool          `json:"preserveCase,omitempty"`
	ShowReplace  bool          `json:"showReplace,omitempty"`
	Mask         string        `json:"mask,omitempty"`
	Scope        string        `json:"scope,omitempty"`
	Directory    string        `json:"directory,omitempty"`
	Layout       picker.Layout `json:"layout"`
}
