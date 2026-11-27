package references

import (
	"context"
	"os"
	"slices"
	"strings"

	"github.com/ZerGo0/editor/internal/search/textsearch"
	"github.com/ZerGo0/editor/internal/syntax"
	"github.com/egoist/mygo/ui/editor"
)

const (
	MaxMatches       = 2000
	MaxFiles         = 20000
	MaxFileBytes     = 1 << 20
	scanMatchesLimit = 10 * MaxMatches
)

type Site struct {
	Path   string
	Line   int
	Col    int
	Member bool
}

type siteKey struct {
	path      string
	line, col int
}

type Match struct {
	textsearch.Match
	Definition bool
	Member     bool
}

type File struct {
	Path    string
	Matches []Match
}

type Result struct {
	Files     []File
	Total     int
	Truncated bool
}

type Query struct {
	Word     string
	Root     string
	Current  string
	Files    []string
	Exclude  []string
	Sites    []Site
	Member   bool
	Overlay  func(path string) (string, bool)
	Language string
}

func (q Query) siteKeys() map[siteKey]bool {
	out := make(map[siteKey]bool, len(q.Sites))
	for _, s := range q.Sites {
		out[siteKey{s.Path, s.Line, s.Col}] = true
	}
	return out
}

func FollowsMember(line string, start int) bool {
	_, ok := memberAccess(line, start)
	return ok
}

func PreferMember(sites []Site, member bool) []Site {
	preferred := slices.DeleteFunc(slices.Clone(sites), func(s Site) bool { return s.Member != member })
	if len(preferred) == 0 {
		return sites
	}
	return preferred
}

// TECHDEBT: Name-based search without scopes or types: it lists whole-word code matches of the same language family; use a language server when one exists.
func Find(ctx context.Context, q Query) (Result, error) {
	res, err := q.findScoped(ctx)
	if err != nil {
		return Result{}, err
	}
	res.prefer(q.Member)
	return res, nil
}

func (q Query) findScoped(ctx context.Context) (Result, error) {
	if len(q.Sites) == 0 && q.Current != "" {
		local, err := q.scan(ctx, []string{q.Current}, false)
		if err != nil || local.Total > 0 {
			return local, err
		}
	}
	candidates := q.Files[:min(len(q.Files), MaxFiles)]
	if q.Language != "" {
		family := familyOf(q.Language)
		related := slices.DeleteFunc(slices.Clone(candidates), func(p string) bool { return !inFamily(p, family) })
		res, err := q.scan(ctx, related, false)
		if err != nil || res.Total > 0 {
			return res, err
		}
	}
	return q.scan(ctx, candidates, false)
}

func (r *Result) prefer(member bool) {
	matches := func(m Match) bool { return m.Definition || m.Member == member }
	preferred := 0
	for _, f := range r.Files {
		for _, m := range f.Matches {
			if !m.Definition && m.Member == member {
				preferred++
			}
		}
	}
	if preferred == 0 {
		return
	}
	kept := r.Files[:0]
	r.Total = 0
	for _, f := range r.Files {
		f.Matches = slices.DeleteFunc(f.Matches, func(m Match) bool { return !matches(m) })
		if len(f.Matches) > 0 {
			kept = append(kept, f)
			r.Total += len(f.Matches)
		}
	}
	r.Files = kept
}

func Locate(ctx context.Context, q Query) (Result, error) {
	var files []string
	for _, s := range q.Sites {
		if !slices.Contains(files, s.Path) {
			files = append(files, s.Path)
		}
	}
	return q.scan(ctx, files, true)
}

func (q Query) scan(ctx context.Context, files []string, onlySites bool) (Result, error) {
	m, err := textsearch.Compile(textsearch.Query{Pattern: q.Word, MatchCase: true, WholeWord: true, Root: q.Root, Exclude: q.Exclude})
	if err != nil {
		return Result{}, err
	}
	sites := q.siteKeys()
	var res Result
	opts := textsearch.Options{Root: q.Root, Files: files, Overlay: q.Overlay, MaxFileSize: MaxFileBytes, Limit: scanMatchesLimit}
	stats, err := textsearch.Search(ctx, m, opts, func(fr textsearch.FileResult) {
		if kept := q.keep(fr, sites, onlySites); len(kept) > 0 {
			res.Files = append(res.Files, File{Path: fr.Path, Matches: kept})
		}
	})
	if err != nil {
		return Result{}, err
	}
	res.Truncated = stats.Truncated
	q.sortFiles(&res)
	return res, nil
}

func (q Query) text(path string) (string, bool) {
	if q.Overlay != nil {
		if text, ok := q.Overlay(path); ok {
			return text, true
		}
	}
	data, err := os.ReadFile(path)
	return string(data), err == nil
}

func (q Query) keep(fr textsearch.FileResult, sites map[siteKey]bool, onlySites bool) []Match {
	text, ok := q.text(fr.Path)
	if !ok {
		return nil
	}
	lines := splitLines(text)
	wanted := map[int]bool{}
	for _, m := range fr.Matches {
		wanted[m.Start.Line] = true
	}
	var tokens map[int][]editor.Token
	if !onlySites {
		tokens = lineTokens(syntax.For(fr.Path, text), lines, wanted)
	}
	var out []Match
	for _, m := range fr.Matches {
		if m.Multiline() || m.Start.Line >= len(lines) {
			continue
		}
		line := lines[m.Start.Line]
		if m.End.Col > len(line) || !wholeWord(line, m.Start.Col, m.End.Col) {
			continue
		}
		site := sites[siteKey{fr.Path, m.Start.Line, m.Start.Col}]
		if onlySites && !site || !onlySites && !site && !inCodeAt(tokens[m.Start.Line], m.Start.Col) {
			continue
		}
		op, member := memberAccess(line, m.Start.Col)
		out = append(out, Match{Match: m, Definition: site, Member: member && inCodeAt(tokens[m.Start.Line], op)})
	}
	return out
}

func (q Query) sortFiles(res *Result) {
	slices.SortFunc(res.Files, func(a, b File) int {
		switch {
		case a.Path == b.Path:
			return 0
		case a.Path == q.Current:
			return -1
		case b.Path == q.Current:
			return 1
		}
		return strings.Compare(a.Path, b.Path)
	})
	for i := range res.Files {
		slices.SortStableFunc(res.Files[i].Matches, func(a, b Match) int {
			if a.Definition != b.Definition {
				if a.Definition {
					return -1
				}
				return 1
			}
			return 0
		})
	}
	for i := range res.Files {
		room := MaxMatches - res.Total
		if room <= 0 {
			res.Files, res.Truncated = res.Files[:i], true
			return
		}
		if len(res.Files[i].Matches) > room {
			res.Files[i].Matches = res.Files[i].Matches[:room]
			res.Files, res.Truncated = res.Files[:i+1], true
		}
		res.Total += len(res.Files[i].Matches)
	}
}
