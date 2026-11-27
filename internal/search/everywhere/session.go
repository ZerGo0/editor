package everywhere

import (
	"cmp"
	"context"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/ZerGo0/editor/internal/fsutil"
	"github.com/ZerGo0/editor/internal/fuzzy"
	"github.com/ZerGo0/editor/internal/glob"
	"github.com/ZerGo0/editor/internal/search/symbols"
)

const (
	openFileBonus   = 40
	recentFileBonus = 24
	recentBonusStep = 1
)

type Filter struct {
	Globs       []string
	ProjectOnly bool
}

type Session struct {
	*Search
	filter Filter
	recent []string
	open   map[string]bool
}

func (x *Search) Session() *Session {
	s := &Session{Search: x}
	if x.env.Recent != nil {
		s.recent = x.env.Recent()
	}
	return s
}

func (s *Session) SetFilter(f Filter) {
	s.filter = f
}

func (s *Session) allowed(rel string) bool {
	if s.filter.ProjectOnly && !filepath.IsLocal(rel) {
		return false
	}
	return len(s.filter.Globs) == 0 || glob.MatchAny(s.filter.Globs, rel)
}

func (s *Session) MarkOpen(paths []string) {
	s.open = make(map[string]bool, len(paths))
	for _, p := range paths {
		s.open[p] = true
	}
}

func (s *Session) Recent(limit int) []Result {
	var out []Result
	for _, p := range s.recent {
		if len(out) == limit {
			break
		}
		if !s.allowed(s.rel(p)) {
			continue
		}
		r := fileResult(s.rel(p), p, nil)
		r.Recent, r.Open = true, s.open[p]
		out = append(out, r)
	}
	return out
}

func (s *Session) Files(q string, limit int) ([]Result, int) {
	if q == "" {
		out := s.Recent(limit)
		return out, len(out)
	}
	if s.env.Files == nil {
		return nil, 0
	}
	paths, _ := s.env.Files.Paths()
	rs := fuzzy.Filter(q, len(paths), func(i int) string { return paths[i] })
	rs = slices.DeleteFunc(rs, func(r fuzzy.Result) bool { return !s.allowed(paths[r.Index]) })
	rank := make(map[string]int, len(s.recent))
	for i, p := range s.recent {
		rank[p] = i
	}
	bonus := func(abs string) int {
		if s.open[abs] {
			return openFileBonus
		}
		if i, ok := rank[abs]; ok {
			return max(recentFileBonus-i*recentBonusStep, 1)
		}
		return 0
	}
	out := make([]Result, 0, min(len(rs), limit*2))
	for _, r := range rs {
		rel := paths[r.Index]
		abs := filepath.Join(s.env.Root, filepath.FromSlash(rel))
		fr := fileResult(rel, abs, r.Match.Positions)
		b := bonus(abs)
		fr.Open, fr.Recent = s.open[abs], b > 0
		fr.score = r.Match.Score + b
		out = append(out, fr)
	}
	slices.SortStableFunc(out, func(a, b Result) int { return cmp.Compare(b.score, a.score) })
	return out[:min(len(out), limit)], len(rs)
}

func fileResult(rel, abs string, positions []int) Result {
	name, dir := path.Base(rel), path.Dir(rel)
	if dir == "." {
		dir = ""
	}
	nameStart := utf8.RuneCountInString(rel) - utf8.RuneCountInString(name)
	if path.IsAbs(rel) {
		dir = fsutil.DisplayPath(dir)
	}
	r := Result{Kind: KindFile, Label: name, Detail: dir, Path: abs}
	for _, p := range positions {
		if p >= nameStart {
			r.Positions = append(r.Positions, p-nameStart)
		} else {
			r.DetailPositions = append(r.DetailPositions, p)
		}
	}
	return r
}

func (s *Session) Symbols(ctx context.Context, q string, limit int) ([]Result, int) {
	if q == "" || s.env.Symbols == nil {
		return nil, 0
	}
	type hit struct {
		path  string
		sym   symbols.Symbol
		match fuzzy.Match
	}
	var hits []hit
	s.env.Symbols.All(func(p string, sym symbols.Symbol) bool {
		if m, ok := fuzzy.Find(q, sym.Name); ok {
			hits = append(hits, hit{p, sym, m})
		}
		return ctx.Err() == nil
	})
	slices.SortFunc(hits, func(a, b hit) int {
		return cmp.Or(cmp.Compare(b.match.Score, a.match.Score), cmp.Compare(len(a.sym.Name), len(b.sym.Name)), strings.Compare(a.sym.Name, b.sym.Name), strings.Compare(a.path, b.path), cmp.Compare(a.sym.Line, b.sym.Line))
	})
	out := make([]Result, 0, min(len(hits), limit))
	for _, h := range hits[:min(len(hits), limit)] {
		out = append(out, SymbolResult(h.sym, h.match.Positions, h.path, s.rel(h.path)))
	}
	return out, len(hits)
}

func SymbolResult(sym symbols.Symbol, positions []int, abs, rel string) Result {
	return Result{
		Kind: KindSymbol, Label: sym.Name, Detail: rel, Positions: positions, Symbol: sym.Kind, Container: sym.Container,
		Path: abs, Line: sym.Line, Col: sym.Col, HasPos: true,
	}
}

func (s *Session) Actions(q string, limit int) ([]Result, int) {
	if s.env.Actions == nil {
		return nil, 0
	}
	all := s.env.Actions()
	rs := fuzzy.Filter(q, len(all), func(i int) string { return all[i].Label })
	out := make([]Result, 0, min(len(rs), limit))
	for _, r := range rs[:min(len(rs), limit)] {
		a := all[r.Index]
		out = append(out, Result{Kind: KindAction, Label: a.Label, Keys: a.Keys, ActionID: a.ID, Positions: r.Match.Positions})
	}
	return out, len(rs)
}
