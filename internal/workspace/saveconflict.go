package workspace

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/document"
	"github.com/egoist/mygo"
)

const (
	overwriteButton = 0
	compareButton   = 1
	keepButton      = 2
	saveAgainButton = 0
	skipSaveButton  = 1
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

func deletedOnDisk(docs []*document.Document) []*document.Document {
	var out []*document.Document
	for _, d := range docs {
		if _, mod := d.DiskStamp(); mod.IsZero() {
			continue
		}
		if _, err := os.Stat(d.Path); errors.Is(err, fs.ErrNotExist) {
			out = append(out, d)
		}
	}
	return out
}

func (w *Workspace) confirmOverwrite(docs []*document.Document, save func()) {
	if w.win == nil {
		save()
		return
	}
	w.confirmRecreate(docs, func() { w.confirmChanged(docs, save) })
}

func (w *Workspace) confirmRecreate(docs []*document.Document, next func()) {
	gone := deletedOnDisk(docs)
	if len(gone) == 0 {
		next()
		return
	}
	message := fmt.Sprintf("%q was deleted from disk.", filepath.Base(gone[0].Path))
	if len(gone) > 1 {
		message = fmt.Sprintf("%d files were deleted from disk.", len(gone))
	}
	applog.Go(func() {
		res, err := mygo.Dialog.Message(mygo.MessageOptions{
			Parent:       w.win,
			Type:         mygo.MessageWarning,
			Message:      message,
			Detail:       "Another program deleted or moved it. Saving creates the file again with your changes.",
			Buttons:      []string{"Save", "Cancel"},
			CancelButton: skipSaveButton,
		})
		if err != nil || res.Button != saveAgainButton {
			return
		}
		w.update(next)
	})
}

func (w *Workspace) confirmChanged(docs []*document.Document, save func()) {
	conflicts := changedOnDisk(docs)
	if len(conflicts) == 0 {
		save()
		return
	}
	message := fmt.Sprintf("%q is newer on disk.", filepath.Base(conflicts[0].Path))
	if len(conflicts) > 1 {
		message = fmt.Sprintf("%d files are newer on disk.", len(conflicts))
	}
	first := conflicts[0]
	applog.Go(func() {
		res, err := mygo.Dialog.Message(mygo.MessageOptions{
			Parent:             w.win,
			Type:               mygo.MessageWarning,
			Message:            message,
			Detail:             "Another program changed it after you opened it. Compare the versions first, or overwrite the file with your changes.",
			Buttons:            []string{"Overwrite", "Compare", "Cancel"},
			CancelButton:       keepButton,
			DestructiveButtons: []int{overwriteButton},
		})
		if err != nil {
			return
		}
		switch res.Button {
		case overwriteButton:
			w.update(save)
		case compareButton:
			w.update(func() { w.compareWithSaved(first) })
		}
	})
}
