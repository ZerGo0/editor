package finder

import (
	"path"
	"strconv"
	"strings"

	"github.com/ZerGo0/editor/internal/search/textsearch"
	"github.com/egoist/mygo/ui/editor"
)

type FileNode struct {
	Path    string
	Rel     string
	Name    string
	Dir     string
	Matches []textsearch.Match
}

func NewFileNode(abs, rel string, matches []textsearch.Match) *FileNode {
	dir := ""
	if i := strings.LastIndexByte(rel, '/'); i >= 0 {
		dir = rel[:i]
	}
	return &FileNode{Path: abs, Rel: rel, Name: path.Base(rel), Dir: dir, Matches: matches}
}

func (f *FileNode) location(i int) Location {
	if i < 0 || i >= len(f.Matches) {
		return Location{Path: f.Path}
	}
	m := f.Matches[i]
	return Location{
		Path:     f.Path,
		Start:    editor.Pos{Line: m.Start.Line, Col: m.Start.Col},
		End:      editor.Pos{Line: m.End.Line, Col: m.End.Col},
		HasRange: true,
	}
}

func lineLabel(m textsearch.Match) string {
	return "line " + strconv.Itoa(m.Start.Line+1) + ": " + strings.TrimSpace(m.Preview)
}
