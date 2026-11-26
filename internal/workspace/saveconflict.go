package workspace

import (
	"fmt"
	"path/filepath"

	"github.com/ZerGo0/editor/internal/document"
	"github.com/egoist/mygo"
)

const (
	overwriteButton = 0
	keepButton      = 1
)

func changedOnDisk(docs []*document.Document) []*document.Document {
	var out []*document.Document
	for _, d := range docs {
		if d.ChangedOnDisk() {
			out = append(out, d)
		}
	}
	return out
}

func (w *Workspace) confirmOverwrite(docs []*document.Document, save func()) {
	conflicts := changedOnDisk(docs)
	if len(conflicts) == 0 || w.win == nil {
		save()
		return
	}
	message := fmt.Sprintf("%s is newer on disk.", filepath.Base(conflicts[0].Path))
	if len(conflicts) > 1 {
		message = fmt.Sprintf("%d files are newer on disk.", len(conflicts))
	}
	go func() {
		res, err := mygo.Dialog.Message(mygo.MessageOptions{
			Parent:             w.win,
			Type:               mygo.MessageWarning,
			Message:            message,
			Detail:             "Another program changed the file after you opened it. Overwrite replaces those changes with yours.",
			Buttons:            []string{"Overwrite", "Cancel"},
			CancelButton:       keepButton,
			DestructiveButtons: []int{overwriteButton},
		})
		if err != nil || res.Button != overwriteButton {
			return
		}
		w.update(save)
	}()
}
