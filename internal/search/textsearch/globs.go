package textsearch

import (
	"path/filepath"
	"strings"

	"github.com/ZerGo0/editor/internal/glob"
)

const (
	anyDirs      = "**"
	anyDirsLead  = "**/"
	anyDirsTrail = "/**"
	rootPrefix   = "./"
)

func ParseGlobs(s string) []string {
	var out []string
	for _, part := range splitTopLevel(s) {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, expandBraces(part)...)
		}
	}
	return out
}

func splitTopLevel(s string) []string {
	var parts []string
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth = max(depth-1, 0)
		case ',':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	return append(parts, s[start:])
}

func expandBraces(p string) []string {
	open := strings.IndexByte(p, '{')
	if open < 0 {
		return []string{p}
	}
	depth, closeAt := 0, -1
	for i := open; i < len(p) && closeAt < 0; i++ {
		switch p[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				closeAt = i
			}
		}
	}
	if closeAt < 0 {
		return []string{p}
	}
	var out []string
	for _, alt := range splitTopLevel(p[open+1 : closeAt]) {
		for _, rest := range expandBraces(p[closeAt+1:]) {
			out = append(out, expandBraces(p[:open]+alt+rest)...)
		}
	}
	return out
}

type globPattern struct {
	pattern  string
	rootOnly bool
}

type globSet []globPattern

func newGlobSet(patterns []string, root string) globSet {
	var set globSet
	for _, p := range patterns {
		set = append(set, variants(strings.TrimSpace(p), root)...)
	}
	return set
}

func variants(p, root string) []globPattern {
	p = filepath.ToSlash(p)
	anchored := false
	switch {
	case strings.HasPrefix(p, rootPrefix):
		p, anchored = strings.TrimPrefix(p, rootPrefix), true
	case strings.HasPrefix(p, "/"):
		rel := p
		if root != "" {
			if r, err := filepath.Rel(root, p); err == nil && filepath.IsLocal(r) {
				rel = filepath.ToSlash(r)
			}
		}
		p, anchored = strings.TrimPrefix(rel, "/"), true
	}
	p = strings.Trim(p, "/")
	if p == "" || p == "." {
		return nil
	}
	if anchored {
		return []globPattern{{pattern: p, rootOnly: !strings.Contains(p, "/")}, {pattern: p + anyDirsTrail}}
	}
	if p != anyDirs && !strings.HasPrefix(p, anyDirsLead) {
		p = anyDirsLead + p
	}
	return []globPattern{{pattern: p}, {pattern: p + anyDirsTrail}}
}

func (g globSet) empty() bool { return len(g) == 0 }

func (g globSet) match(rel string) bool {
	nested := strings.Contains(rel, "/")
	for _, p := range g {
		if p.rootOnly && nested {
			continue
		}
		if glob.Match(p.pattern, rel) {
			return true
		}
	}
	return false
}
