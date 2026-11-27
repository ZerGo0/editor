package symbolnav

import (
	"path/filepath"
	"strconv"

	"github.com/ZerGo0/editor/internal/search/finder"
	"github.com/ZerGo0/editor/internal/search/references"
	"github.com/ZerGo0/editor/internal/search/textsearch"
	"github.com/egoist/mygo/ui/editor"
)

type site struct {
	path      string
	line, col int
}

type Result struct {
	Kind      Kind
	Word      string
	Files     []*finder.FileNode
	Total     int
	Truncated bool
	defs      map[site]bool
}

func NewResult(kind Kind, word, root string, res references.Result) Result {
	out := Result{Kind: kind, Word: word, Total: res.Total, Truncated: res.Truncated, defs: map[site]bool{}}
	for _, f := range res.Files {
		rel, err := filepath.Rel(root, f.Path)
		if err != nil || !filepath.IsLocal(rel) {
			rel = f.Path
		}
		matches := make([]textsearch.Match, len(f.Matches))
		for i, m := range f.Matches {
			matches[i] = m.Match
			if m.Definition {
				out.defs[site{f.Path, m.Start.Line, m.Start.Col}] = true
			}
		}
		out.Files = append(out.Files, finder.NewFileNode(f.Path, filepath.ToSlash(rel), matches))
	}
	return out
}

func (r Result) Definition(f *finder.FileNode, i int) bool {
	m := f.Matches[i]
	return r.defs[site{f.Path, m.Start.Line, m.Start.Col}]
}

func (r Result) Find() finder.FindResults {
	title := SearchTitle(r.Kind, r.Word)
	if r.Truncated {
		title += " (first " + strconv.Itoa(references.MaxMatches) + ")"
	}
	return finder.FindResults{Title: title, Files: r.Files, Total: r.Total}
}

func SearchTitle(kind Kind, word string) string {
	return kind.Title() + " for " + strconv.Quote(word)
}

func (r Result) Location(f *finder.FileNode, i int) finder.Location {
	m := f.Matches[i]
	return finder.Location{
		Path:     f.Path,
		Start:    editor.Pos{Line: m.Start.Line, Col: m.Start.Col},
		End:      editor.Pos{Line: m.End.Line, Col: m.End.Col},
		HasRange: true,
	}
}

type Near struct {
	Path string
	Pos  editor.Pos
}

func (r Result) Nearest(n Near) (file, match int) {
	found, bestDist := false, 0
	for fi, f := range r.Files {
		if f.Path != n.Path {
			continue
		}
		for mi, m := range f.Matches {
			if m.Start.Line == n.Pos.Line && m.Start.Col <= n.Pos.Col && n.Pos.Col <= m.End.Col {
				return fi, mi
			}
			dist := max(m.Start.Line-n.Pos.Line, n.Pos.Line-m.Start.Line)
			if !found || dist < bestDist {
				found, bestDist, file, match = true, dist, fi, mi
			}
		}
	}
	return file, match
}
