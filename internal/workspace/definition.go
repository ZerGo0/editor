package workspace

import (
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/ZerGo0/editor/internal/layout"
	"github.com/ZerGo0/editor/internal/search/finder"
	"github.com/ZerGo0/editor/internal/search/references"
	"github.com/ZerGo0/editor/internal/search/symbols"
	"github.com/ZerGo0/editor/internal/workspace/symbolnav"
	"github.com/egoist/mygo/ui/editor"
)

var (
	definitionAction = symbolnav.Action{Kind: symbolnav.Definitions, Mode: symbolnav.ModeGo}
	referencesAction = symbolnav.Action{Kind: symbolnav.References, Mode: symbolnav.ModeGo}
)

func (w *Workspace) RunSymbolNav(a symbolnav.Action) {
	switch a.Mode {
	case symbolnav.ModeNext:
		w.peek.view.Step(1)
	case symbolnav.ModePrevious:
		w.peek.view.Step(-1)
	case symbolnav.ModeClose:
		w.peek.view.Dismiss()
	default:
		if t := w.active.ActiveTab(); t != nil && w.hasSymbolProviders(t.Editor) {
			w.symbolNav(t, a)
		}
	}
}

func (w *Workspace) hasSymbolProviders(st *editor.State) bool {
	if st == nil {
		return false
	}
	lang := st.Buffer.Language()
	return lang != nil && symbols.Supports(lang.Name)
}

func (w *Workspace) goToDefinition(st *editor.State) {
	for _, t := range w.tabs() {
		if t.Editor == st {
			w.symbolNav(t, definitionAction)
			return
		}
	}
}

func (w *Workspace) symbolNav(t *layout.Tab, a symbolnav.Action) {
	st := t.Editor
	pos := st.CursorPos()
	line := st.Buffer.Line(pos.Line)
	start, end := wordRange(line, pos.Col)
	word := line[start:end]
	if word == "" {
		w.setNotice(a.Kind.Missing(""))
		return
	}
	member := references.FollowsMember(line, start)
	sites := references.PreferMember(w.definitionSites(word), member)
	if a.Kind == symbolnav.References {
		w.findReferences(t, a, word, member, sites)
		return
	}
	switch {
	case len(sites) == 0:
		w.peek.view.Close()
		w.setNotice(a.Kind.Missing(word))
	case a.Mode != symbolnav.ModePeek && len(sites) == 1:
		w.goToSite(t, a, sites[0], word, pos)
	default:
		w.startSearch(t, a, word, references.Query{Sites: sites, Member: member}, true)
	}
}

func (w *Workspace) goToSite(t *layout.Tab, a symbolnav.Action, s references.Site, word string, pos editor.Pos) {
	atCursor := s.Path == t.Path && s.Line == pos.Line && s.Col <= pos.Col && pos.Col <= s.Col+len(word)
	if atCursor && a.Kind == symbolnav.Definitions {
		w.symbolNav(t, referencesAction)
		return
	}
	w.openLocation(finder.Location{Path: s.Path, Start: editor.Pos{Line: s.Line, Col: s.Col}, End: editor.Pos{Line: s.Line, Col: s.Col}, HasRange: true},
		finder.OpenOptions{Preview: true, Focus: true, Side: a.Mode == symbolnav.ModeSide})
}

func (w *Workspace) definitionSites(word string) []references.Site {
	var sites []references.Site
	w.search.symbols.All(func(path string, s symbols.Symbol) bool {
		if s.Name == word {
			sites = append(sites, references.Site{Path: path, Line: s.Line, Col: s.Col, Member: s.Kind == symbols.Method || s.Kind == symbols.Field})
		}
		return true
	})
	slices.SortFunc(sites, func(a, b references.Site) int {
		if c := strings.Compare(a.Path, b.Path); c != 0 {
			return c
		}
		return a.Line - b.Line
	})
	return sites
}

func (w *Workspace) findReferences(t *layout.Tab, a symbolnav.Action, word string, member bool, sites []references.Site) {
	q := references.Query{Sites: sites, Member: member}
	if lang := t.Editor.Buffer.Language(); lang != nil {
		q.Language = lang.Name
	}
	w.startSearch(t, a, word, q, false)
}

func wordAround(line string, col int) string {
	start, end := wordRange(line, col)
	return line[start:end]
}

func wordRange(line string, col int) (start, end int) {
	col = max(0, min(col, len(line)))
	start, end = col, col
	for start > 0 {
		r, size := utf8.DecodeLastRuneInString(line[:start])
		if !references.IsWordRune(r) {
			break
		}
		start -= size
	}
	for end < len(line) {
		r, size := utf8.DecodeRuneInString(line[end:])
		if !references.IsWordRune(r) {
			break
		}
		end += size
	}
	return start, end
}
