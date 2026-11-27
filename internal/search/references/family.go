package references

import (
	"slices"

	"github.com/ZerGo0/editor/internal/syntax"
)

var families = [][]string{
	{"JavaScript", "TypeScript", "TypeScript React", "Svelte", "Astro"},
	{"C", "C++"},
	{"Java", "Kotlin", "Scala"},
}

func familyOf(name string) string {
	for _, f := range families {
		if slices.Contains(f, name) {
			return f[0]
		}
	}
	return name
}

func inFamily(path string, family string) bool {
	lang := syntax.ForName(path)
	return lang != nil && familyOf(lang.Name) == family
}
