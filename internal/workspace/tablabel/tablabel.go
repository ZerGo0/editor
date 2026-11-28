package tablabel

import (
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/text/unicode/bidi"
)

const (
	ellipsis      = "…" + string(filepath.Separator)
	leftToRightMk = "\u200E"
)

func LTR(name string) string {
	label, _ := LTRMatches(name, nil)
	return label
}

func LTRMatches(name string, matches []int) (string, []int) {
	if !startsRightToLeft(name) {
		return name, matches
	}
	shifted := make([]int, len(matches))
	for i, m := range matches {
		shifted[i] = m + 1
	}
	return leftToRightMk + name, shifted
}

func startsRightToLeft(s string) bool {
	for _, r := range s {
		p, _ := bidi.LookupRune(r)
		switch p.Class() {
		case bidi.L:
			return false
		case bidi.R, bidi.AL:
			return true
		}
	}
	return false
}

func Describe(paths []string) map[string]string {
	byName := map[string][]string{}
	for _, p := range paths {
		name := filepath.Base(p)
		if !slices.Contains(byName[name], p) {
			byName[name] = append(byName[name], p)
		}
	}
	out := map[string]string{}
	for _, group := range byName {
		if len(group) < 2 {
			continue
		}
		for p, d := range distinguish(group) {
			out[p] = d
		}
	}
	return out
}

func Qualify(paths []string) map[string]string {
	out := map[string]string{}
	for p, d := range Describe(paths) {
		name := filepath.Base(p)
		if d = strings.TrimPrefix(d, ellipsis); d != "" {
			name = d + string(filepath.Separator) + name
		}
		out[p] = name
	}
	return out
}

func distinguish(paths []string) map[string]string {
	parts := make([][]string, len(paths))
	longest := 0
	for i, p := range paths {
		dir := filepath.Dir(p)
		parts[i] = strings.Split(strings.Trim(dir, string(filepath.Separator)), string(filepath.Separator))
		longest = max(longest, len(parts[i]))
	}
	for depth := 1; ; depth++ {
		out := make(map[string]string, len(paths))
		seen := map[string]bool{}
		unique := true
		for i, p := range paths {
			d := tail(parts[i], depth)
			if seen[d] {
				unique = false
			}
			seen[d] = true
			out[p] = d
		}
		if unique || depth >= longest {
			return out
		}
	}
}

func tail(segments []string, n int) string {
	if n >= len(segments) {
		return strings.Join(segments, string(filepath.Separator))
	}
	return ellipsis + strings.Join(segments[len(segments)-n:], string(filepath.Separator))
}
