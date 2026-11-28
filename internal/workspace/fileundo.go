package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/ZerGo0/editor/internal/applog"
	"github.com/ZerGo0/editor/internal/workspace/filetree"
	"github.com/egoist/mygo"
)

const undoButton = "Undo"

var errNotInTrash = errors.New("The item is no longer in the Trash.")

func (w *Workspace) UndoFileOperation() {
	w.replayFileChange(w.tree.PopUndo(), true)
}

func (w *Workspace) RedoFileOperation() {
	w.replayFileChange(w.tree.PopRedo(), false)
}

func (w *Workspace) replayFileChange(ch *filetree.Change, undo bool) {
	if ch == nil {
		return
	}
	applog.Go(func() {
		if undo && len(ch.Created) > 0 && !w.confirmUndo(ch) {
			w.update(func() {
				w.tree.PushUndo(ch)
				w.tree.Settled()
			})
			return
		}
		refresh, err := applyFileChange(ch, undo)
		w.update(func() {
			defer w.tree.Settled()
			for _, f := range refresh {
				f(w)
			}
			verb := "redo"
			if undo {
				verb = "undo"
			}
			switch {
			case err != nil:
				w.showError(fmt.Sprintf("Cannot %s %s", verb, ch.Label), err)
			case undo:
				w.tree.PushRedo(ch)
			default:
				w.tree.PushUndo(ch)
			}
		})
	})
}

func (w *Workspace) confirmUndo(ch *filetree.Change) bool {
	detail := fmt.Sprintf("This moves %q to the Trash.", filepath.Base(ch.Created[0].Path))
	if n := len(ch.Created); n > 1 {
		detail = fmt.Sprintf("This moves %d items to the Trash.", n)
	}
	res, err := mygo.Dialog.Message(mygo.MessageOptions{
		Parent:             w.win,
		Type:               mygo.MessageWarning,
		Message:            fmt.Sprintf("Undo %s?", ch.Label),
		Detail:             detail,
		Buttons:            []string{undoButton, "Cancel"},
		CancelButton:       1,
		DestructiveButtons: []int{0},
	})
	return err == nil && res.Button == 0
}

func applyFileChange(ch *filetree.Change, undo bool) ([]func(*Workspace), error) {
	restore, trash := ch.Created, ch.Removed
	moves := slices.Clone(ch.Moves)
	if undo {
		restore, trash = ch.Removed, ch.Created
		slices.Reverse(moves)
		for i, m := range moves {
			moves[i] = filetree.Move{From: m.To, To: m.From}
		}
	}
	var refresh []func(*Workspace)
	for i := range restore {
		e := &restore[i]
		if err := restoreEntry(*e); err != nil {
			return refresh, err
		}
		p := e.Path
		refresh = append(refresh, func(w *Workspace) {
			w.tree.Invalidate(filepath.Dir(p))
			w.tree.Reveal(p)
		})
	}
	for _, m := range moves {
		if err := renameFree(m.From, m.To); err != nil {
			return refresh, err
		}
		refresh = append(refresh, func(w *Workspace) { w.afterMove(m.From, m.To) })
	}
	for i := range trash {
		e := &trash[i]
		trashed, err := mygo.Shell.TrashItemResult(e.Path)
		if err != nil {
			return refresh, err
		}
		e.Trash = trashed
		p := e.Path
		refresh = append(refresh, func(w *Workspace) { w.removed(p) })
	}
	return refresh, nil
}

func restoreEntry(e filetree.Trashed) error {
	if e.Trash == "" {
		return errNotInTrash
	}
	if _, err := os.Lstat(e.Trash); err != nil {
		return errNotInTrash
	}
	if err := os.MkdirAll(filepath.Dir(e.Path), dirPerm); err != nil {
		return err
	}
	return renameFree(e.Trash, e.Path)
}

func renameFree(from, to string) error {
	if _, err := os.Lstat(to); err == nil {
		return fmt.Errorf("%s already exists.", filepath.Base(to))
	}
	return os.Rename(from, to)
}
