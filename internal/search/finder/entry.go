package finder

import (
	"github.com/ZerGo0/editor/internal/search/everywhere"
	"github.com/egoist/mygo/ui/editor"
)

type entryKind uint8

const (
	kindFile entryKind = iota
	kindSymbol
	kindAction
	kindMatch
	kindLine
	kindMore
	kindHint
)

type entry struct {
	kind      entryKind
	group     string
	label     string
	detail    string
	keys      string
	positions []int
	path      string
	line, col int
	hasPos    bool
	actionID  string
	file      *FileNode
	match     int
	tab       Tab
	query     string
	recent    bool
}

func fileEntry(r everywhere.Result, group string) entry {
	return entry{kind: kindFile, group: group, label: r.Label, detail: r.Detail, positions: r.Positions, path: r.Path, recent: r.Recent}
}

func symbolEntry(r everywhere.Result, group string) entry {
	detail := r.Detail
	if r.Container != "" {
		detail = r.Container + " · " + r.Detail
	}
	return entry{kind: kindSymbol, group: group, label: r.Label, detail: detail, positions: r.Positions, path: r.Path, line: r.Line, col: r.Col, hasPos: true}
}

func actionEntry(r everywhere.Result, group string) entry {
	return entry{kind: kindAction, group: group, label: r.Label, keys: r.Keys, positions: r.Positions, actionID: r.ActionID}
}

func (e entry) location() (Location, bool) {
	switch e.kind {
	case kindMatch:
		return e.file.location(e.match), true
	case kindFile:
		return Location{Path: e.path}, true
	case kindSymbol, kindLine:
		pos := editor.Pos{Line: e.line, Col: e.col}
		return Location{Path: e.path, Start: pos, End: pos, HasRange: e.hasPos}, true
	}
	return Location{}, false
}
