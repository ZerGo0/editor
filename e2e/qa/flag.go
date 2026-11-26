package qa

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
)

type Flag struct {
	Rule     string
	View     string
	Context  string
	Owner    string
	Role     string
	Label    string
	Bounds   Rect
	Measured string
	Expected string
	Fix      string
}

func (f Flag) String() string {
	return fmt.Sprintf("[%s] %s: %s %q at (%.0f,%.0f %.0fx%.0f): measured %s, expected %s; fix: %s",
		f.Where(), f.Rule, f.Role, f.Label, f.Bounds.X, f.Bounds.Y, f.Bounds.Width, f.Bounds.Height, f.Measured, f.Expected, f.Fix)
}

func (f Flag) Where() string {
	if f.Context == "" {
		return f.View
	}
	return f.View + "@" + f.Context
}

type Allow struct {
	Rule        string
	View        string
	Role        string
	LabelPrefix string
	Reason      string
}

func (a Allow) Matches(f Flag) bool {
	return (a.Rule == "" || a.Rule == f.Rule) &&
		(a.View == "" || a.View == f.View) &&
		(a.Role == "" || a.Role == f.Role) &&
		strings.HasPrefix(f.Label, a.LabelPrefix)
}

func Filter(flags []Flag, allow []Allow) (kept, allowed []Flag) {
	for _, f := range flags {
		if slices.ContainsFunc(allow, func(a Allow) bool { return a.Matches(f) }) {
			allowed = append(allowed, f)
		} else {
			kept = append(kept, f)
		}
	}
	return kept, allowed
}

func Stale(flags []Flag, allow []Allow) []Allow {
	return slices.DeleteFunc(slices.Clone(allow), func(a Allow) bool {
		return slices.ContainsFunc(flags, a.Matches)
	})
}

func Dedupe(flags []Flag) []Flag {
	seen := map[string]bool{}
	var out []Flag
	for _, f := range flags {
		key := f.Rule + "|" + f.Context + "|" + f.Role + "|" + f.Label
		if !seen[key] {
			seen[key] = true
			out = append(out, f)
		}
	}
	return out
}

func GroupByOwner(flags []Flag) map[string][]Flag {
	out := map[string][]Flag{}
	for _, f := range flags {
		out[f.Owner] = append(out[f.Owner], f)
	}
	return out
}

const maxShownPerRule = 6

func Format(flags []Flag) string {
	groups := GroupByOwner(flags)
	owners := make([]string, 0, len(groups))
	for owner := range groups {
		owners = append(owners, owner)
	}
	slices.Sort(owners)
	var b strings.Builder
	for _, owner := range owners {
		fmt.Fprintf(&b, "%s (%d)\n", cmp.Or(owner, "unowned"), len(groups[owner]))
		shown := map[string]int{}
		hidden := map[string]int{}
		for _, f := range groups[owner] {
			key := f.Rule + " in " + f.Where()
			if shown[key] >= maxShownPerRule {
				hidden[key]++
				continue
			}
			shown[key]++
			fmt.Fprintf(&b, "  %s\n", f)
		}
		keys := make([]string, 0, len(hidden))
		for key := range hidden {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		for _, key := range keys {
			fmt.Fprintf(&b, "  (+%d more %s)\n", hidden[key], key)
		}
	}
	return b.String()
}
