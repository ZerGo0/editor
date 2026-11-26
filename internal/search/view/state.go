package searchview

import "slices"

const historyLimit = 30

type State struct {
	Scope        string   `json:"scope,omitempty"`
	Query        string   `json:"query,omitempty"`
	MatchCase    bool     `json:"matchCase,omitempty"`
	WholeWord    bool     `json:"wholeWord,omitempty"`
	Regex        bool     `json:"regex,omitempty"`
	PreserveCase bool     `json:"preserveCase,omitempty"`
	Replace      string   `json:"replace,omitempty"`
	ShowReplace  bool     `json:"showReplace,omitempty"`
	ShowFilters  bool     `json:"showFilters,omitempty"`
	Include      string   `json:"include,omitempty"`
	Exclude      string   `json:"exclude,omitempty"`
	NoExcludes   bool     `json:"noExcludes,omitempty"`
	OnlyOpen     bool     `json:"onlyOpen,omitempty"`
	ListView     bool     `json:"listView,omitempty"`
	SymbolsTree  bool     `json:"symbolsTree,omitempty"`
	History      []string `json:"history,omitempty"`
}

func addHistory(history []string, q string) []string {
	if q == "" {
		return history
	}
	out := append([]string{q}, slices.DeleteFunc(slices.Clone(history), func(h string) bool { return h == q })...)
	return out[:min(len(out), historyLimit)]
}
