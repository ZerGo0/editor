package findwidget

import (
	"encoding/json"
	"slices"
	"strings"
)

const (
	historyLimit    = 50
	historyMaxBytes = 1024
)

type History struct {
	entries  []string
	at       int
	browsing bool
	draft    string
}

func (h History) Entries() []string { return slices.Clone(h.entries) }

func (h History) IsZero() bool { return len(h.entries) == 0 }

func (h History) MarshalJSON() ([]byte, error) { return json.Marshal(h.entries) }

func (h *History) UnmarshalJSON(data []byte) error {
	var entries []string
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}
	*h = History{}
	for _, e := range entries {
		h.Add(e)
	}
	return nil
}

func (h *History) Add(query string) {
	h.browsing = false
	if strings.TrimSpace(query) == "" || len(query) > historyMaxBytes {
		return
	}
	h.entries = append(slices.DeleteFunc(h.entries, func(e string) bool { return e == query }), query)
	if extra := len(h.entries) - historyLimit; extra > 0 {
		h.entries = slices.Delete(h.entries, 0, extra)
	}
}

func (h *History) Prev(current string) (string, bool) { return h.step(current, -1) }

func (h *History) Next(current string) (string, bool) { return h.step(current, 1) }

func (h *History) step(current string, delta int) (string, bool) {
	if h.browsing && h.entries[h.at] != current {
		h.browsing = false
	}
	if !h.browsing {
		if delta > 0 {
			return "", false
		}
		h.at, h.draft = len(h.entries), current
		if h.at > 0 && h.entries[h.at-1] == current {
			h.at--
		}
	}
	if h.at+delta < 0 {
		return "", false
	}
	h.at += delta
	if h.at == len(h.entries) {
		h.browsing = false
		return h.draft, true
	}
	h.browsing = true
	return h.entries[h.at], true
}
