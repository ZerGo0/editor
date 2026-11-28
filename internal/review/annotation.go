package review

import "fmt"

type Scope string

const (
	ScopeLine    Scope = "line"
	ScopeFile    Scope = "file"
	ScopeGeneral Scope = "general"
)

type Side string

const (
	SideOld Side = "old"
	SideNew Side = "new"
)

type Annotation struct {
	Scope      Scope  `json:"scope"`
	Path       string `json:"path,omitempty"`
	Side       Side   `json:"side,omitempty"`
	Start      int    `json:"start,omitempty"`
	End        int    `json:"end,omitempty"`
	Text       string `json:"text,omitempty"`
	Quote      string `json:"quote,omitempty"`
	Suggestion string `json:"suggestion,omitempty"`
}

func (a Annotation) Short() string {
	switch a.Scope {
	case ScopeFile:
		return "File"
	case ScopeGeneral:
		return "Global"
	}
	if a.End > a.Start {
		return fmt.Sprintf("L%d-%d", a.Start, a.End)
	}
	return fmt.Sprintf("L%d", a.Start)
}

func LineLabel(start, end int) string { return rangeLabel("Line", "Lines", "-", start, end) }

func rangeLabel(one, many, dash string, start, end int) string {
	if end > start {
		return fmt.Sprintf("%s %d%s%d", many, start, dash, end)
	}
	return fmt.Sprintf("%s %d", one, start)
}
