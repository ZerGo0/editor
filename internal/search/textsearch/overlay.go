package textsearch

import "github.com/egoist/mygo/ui/editor"

func SnapshotOverlay(snaps map[string]editor.Snapshot) func(path string) (string, bool) {
	return func(p string) (string, bool) {
		s, ok := snaps[p]
		if !ok {
			return "", false
		}
		return s.String(), true
	}
}
