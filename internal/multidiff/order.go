package multidiff

import (
	"cmp"
	"slices"
	"strings"
)

func sortTreeOrder(entries []Entry) {
	start := 0
	for i := 0; i <= len(entries); i++ {
		if i < len(entries) && entries[i].isFile() {
			continue
		}
		slices.SortStableFunc(entries[start:i], func(a, b Entry) int { return compareTreePaths(a.Path, b.Path) })
		start = i + 1
	}
}

func compareTreePaths(a, b string) int {
	as, bs := strings.Split(a, "/"), strings.Split(b, "/")
	for i := range min(len(as), len(bs)) {
		aDir, bDir := i < len(as)-1, i < len(bs)-1
		if aDir != bDir {
			if aDir {
				return -1
			}
			return 1
		}
		if c := cmp.Compare(as[i], bs[i]); c != 0 {
			return c
		}
	}
	return cmp.Compare(len(as), len(bs))
}
