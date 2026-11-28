package issueui

import "errors"

const detailIDPrefix = "issue"

var errOtherRepo = errors.New("This issue belongs to another repository.")

const sideStateKey = "issueui.side"

type sideMemo struct {
	Open map[string]bool `json:"open,omitempty"`
}

func (i *Issues) sideMemo() sideMemo {
	if i.side == nil {
		return i.saved
	}
	if i.saved.Open == nil {
		i.saved.Open = map[string]bool{}
	}
	for _, s := range i.side.sections {
		i.saved.Open[s.id] = s.open
	}
	return i.saved
}

func (i *Issues) resetSide() {
	i.sideMemo()
	i.side = newSideState(i.settings.Queries)
	for _, s := range i.side.sections {
		if open, ok := i.saved.Open[s.id]; ok {
			s.open = open
		}
	}
}
