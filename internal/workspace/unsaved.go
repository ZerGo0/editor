package workspace

import "strconv"

const (
	unsavedFilesSuffix = " unsaved files"
	unsavedFileSuffix  = " unsaved file"
)

func (w *Workspace) unsavedCount() int {
	return len(w.modified())
}

func unsavedTip(n int) string {
	if n == 1 {
		return strconv.Itoa(n) + unsavedFileSuffix
	}
	return strconv.Itoa(n) + unsavedFilesSuffix
}
